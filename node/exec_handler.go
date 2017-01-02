package node

import (
	"fmt"
)

type fromChan int

const (
	fromMerge fromChan = iota
	fromControl
	fromInput
	fromStatus
)

const (
	blockIdKey = "block_id"
)

// handleFromMain handles messages received in the main func.
// The primary purpose is to intercept the messages,
// run my list of commands, then potentially wait for
// the commands to finish before forwarding the message.
//
// Note that a side effect is that messages can get
// lost, if I happen to receive a new one while waiting
// to hear the response from my commands after a previous
// message.
type handleFromMain struct {
	owner     Owner
	proc      *process
	status    chan error
	needs_run bool
	in_stop   bool
	stop_msg  Msg
	// Solely so I can send a msg down the pipe. Should be a cleaner way.
	ex *Exec
}

func newHandleFromMain(owner Owner, proc *process, status chan error, ex *Exec) *handleFromMain {
	return &handleFromMain{owner, proc, status, false, false, Msg{}, ex}
}

func (h *handleFromMain) close() {
	h.proc.close()
}

func (h *handleFromMain) handleMsg(msg *Msg, from fromChan) {
	if msg == nil {
		return
	}
	if from == fromControl {
		h.handleFromControl(msg)
	} else if from == fromInput {
		h.handleFromInput(msg)
	}
}

func (h *handleFromMain) handleFromControl(msg *Msg) {
	cmd := CmdFromMsg(*msg)
	fmt.Println("exec control msg", *msg, "cmd", cmd)
	if cmd != nil {
		if cmd.Method == cmdStop {
			fmt.Println("Got stop for", cmd)
			if !h.proc.isRunning() {
				reply := Cmd{Method: cmdStopReply, TargetId: msg.SenderId}
				rmsg := reply.AsMsg()
				rmsg.SetInt(blockIdKey, msg.MustGetInt(blockIdKey))
				h.owner.SendMsg(rmsg, msg.SenderId)
			} else {
				h.in_stop = true
				h.stop_msg = *msg
				h.proc.stop()
			}
		}
	}
}

func (h *handleFromMain) handleFromInput(msg *Msg) {
	fmt.Println("exec msg", msg)
	if !h.proc.isRunning() {
		h.proc.run(h.status)
	} else {
		h.needs_run = true
	}
}

func (h *handleFromMain) handleErr(err error, from fromChan) {
	if from == fromStatus {
		h.handleFromStatus(err)
	}
}

func (h *handleFromMain) handleFromStatus(err error) {
	fmt.Println("run err", err)
	h.proc.finished(err)
	if h.in_stop {
		h.in_stop = false
		reply := Cmd{Method: cmdStopReply, TargetId: h.stop_msg.SenderId}
		rmsg := reply.AsMsg()
		rmsg.SetInt(blockIdKey, h.stop_msg.MustGetInt(blockIdKey))
		h.owner.SendMsg(rmsg, h.stop_msg.SenderId)
	} else if h.needs_run {
		h.needs_run = false
		h.proc.run(h.status)
	} else if err == nil {
		// Process completed successfully
		h.ex.SendMsg(Msg{})
	}
}

// handleFromCmds handles messages received in the command func.
// The primary purpose is to intercept the messages,
// run my list of commands, then potentially wait for
// the commands to finish before forwarding the message.
//
// Note that a side effect is that messages can get
// lost, if I happen to receive a new one while waiting
// to hear the response from my commands after a previous
// message.
type handleFromCmds struct {
	// Send commands in blocks, and wait to hear back from all members
	// of a block before proceeding. As soon as we start a new block, the previous is discarded,
	block_id   int
	block_size int
	block_msg  Msg
	// I don't currently have a "message empty" state, and I don't want to store the pointer, so use this
	block_has_msg bool
	owner         Owner
	controlId     Id
	cmdList       []Cmd
	cmds          chan Msg
}

func newHandleFromCmds(owner Owner, controlId Id, cmdList []Cmd, cmds chan Msg) *handleFromCmds {
	return &handleFromCmds{1, 0, Msg{}, false, owner, controlId, cmdList, cmds}
}

func (h *handleFromCmds) handle(msg *Msg, from fromChan) {
	if msg == nil {
		return
	}
	if from == fromMerge {
		h.handleFromMerge(msg)
	} else if from == fromControl {
		h.handleFromControl(msg)
	}
}

func (h *handleFromCmds) handleFromMerge(msg *Msg) {
	h.block_id++
	h.block_size = 0
	h.block_msg = *msg
	h.block_has_msg = true
	for _, v := range h.cmdList {
		m := v.AsMsg()
		m.SenderId = h.controlId
		m.SetInt(blockIdKey, h.block_id)
		err := h.owner.SendMsg(m, v.TargetId)
		fmt.Println("sent stop err", err, "cmd", v, "controlId", h.controlId)
		if err == nil && v.Reply {
			h.block_size++
		}
	}
	// If I'm not waiting to hear back from anyone then just send the message
	if h.block_size <= 0 {
		fmt.Println("Send immediate")
		h.send(msg)
	}
}

func (h *handleFromCmds) handleFromControl(msg *Msg) {
	cmd := CmdFromMsg(*msg)
	if cmd != nil {
		if cmd.Method == cmdStopReply && h.block_id == msg.MustGetInt(blockIdKey) {
			h.block_size--
			if h.block_size == 0 && h.block_has_msg {
				fmt.Println("Send delayed")
				h.send(&h.block_msg)
			}
		}
	}
}

func (h *handleFromCmds) send(msg *Msg) {
	if msg != nil {
		h.cmds <- *msg
		h.block_has_msg = false
	}
}
