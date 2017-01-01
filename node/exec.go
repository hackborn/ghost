package node

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// -----------------------------------------------
// Exec struct
// Run a command.
// The command runs in a separate gofunc, spawned from
// the main running gofunc. The command func has ownership
// over the command, with the main fun being granted access
// to the command for the sole purpose of killing it if necessary.
type Exec struct {
	Id        Id
	Name      string `xml:"name,attr"`
	Cmd       string `xml:"cmd,attr"`
	Args      string `xml:"args,attr"`
	Dir       string `xml:"dir,attr"`
	Interrupt bool   `xml:"interrupt,attr"`
	Autorun   bool   `xml:"autorun,attr"`
	Rerun     bool   `xml:"rerun,attr"`
	input     Channels
	Channels  // Output
	Cmds
}

type fromChan int

const (
	fromMerge fromChan = iota
	fromControl
)

const (
	blockIdKey = "block_id"
)

func (e *Exec) IsValid() bool {
	return len(e.Cmd) > 0
}

func (e *Exec) GetId() Id {
	return e.Id
}

func (e *Exec) GetName() string {
	return e.Name
}

func (e *Exec) ApplyArgs(cs ChangeString) {
	e.Cmd = cs.ChangeString(e.Cmd)
	e.Args = cs.ChangeString(e.Args)
	e.Dir = cs.ChangeString(e.Dir)
}

func (e *Exec) PrepareToStart(p Prepare, inputs []Source) (interface{}, error) {
	return nil, nil
}

func (e *Exec) Start(s Start, data interface{}) error {
	return nil
}

func (e *Exec) StartChannels(a StartArgs, inputs []Source) {
	e.input.CloseChannels()

	// No inputs means this node is never hit, so ignore.
	if len(inputs) <= 0 {
		return
	}

	// If we want multiple inputs, we'll need to expand the running
	// code to handle merging.
	if len(inputs) != 1 {
		fmt.Println("node.Exec.Start() must have 1 input (for now)", len(inputs))
		return
	}

	for _, i := range inputs {
		e.input.Add(i.NewChannel())
	}
}

func (e *Exec) StartRunning(a StartArgs) error {
	fmt.Println("Start exec", e, "ins", len(e.input.Out), "outs", len(e.Out))
	merge, err := e.startMerge(a)
	if err != nil {
		fmt.Println("exec.StartRunning: ", err)
		return err
	}
	if merge == nil {
		return nil
	}

	// If we have any commands, insert a handling stage between merge and my exec routine.
	if len(e.CmdList) > 0 {
		merge, err = e.startCmds(a, merge)
		if err != nil {
			fmt.Println("exec.StartRunning: ", err)
			return err
		}
	}

	done := a.Owner.DoneChannel()
	control, _ := a.Owner.NewControlChannel(e.Id)
	if control == nil {
		return errors.New("Can't make control channel")
	}

	// The cmd runs in a separate gofunc. This channel communicates back to the main func,
	// returning the result of the run (specifically, exec.Cmd.Wait()).
	status := make(chan error)

	a.NodeWaiter.Add(1)
	go func(owner Owner, done chan int, control chan Msg, merge chan Msg) {
		var process *exec.Cmd = nil
		fmt.Println("start exec func")
		defer a.NodeWaiter.Done()
		defer fmt.Println("end exec func")
		defer killExec(process)
		defer e.CloseChannels()

		needs_run := false
		in_stop := false
		var stop_msg Msg

		if e.Autorun {
			process = e.runProcess(status)
		}

		for {
			select {
			case _, more := <-done:
				if !more {
					return
				}
			case msg, more := <-control:
				if more {
					cmd := CmdFromMsg(msg)
					fmt.Println("exec", e.Id, "control msg", msg, "cmd", cmd)
					if cmd != nil {
						if cmd.Method == cmdStop {
							fmt.Println("Got stop for", cmd)
							if process == nil {
								reply := Cmd{Method: cmdStopReply, TargetId: msg.SenderId}
								rmsg := reply.AsMsg()
								rmsg.SetInt(blockIdKey, msg.MustGetInt(blockIdKey))
								owner.SendMsg(rmsg, msg.SenderId)
							} else {
								in_stop = true
								stop_msg = msg
								killExec(process)
							}
						}
					}
				}
			case msg, more := <-merge:
				if more {
					fmt.Println("exec msg", msg)
					if process == nil {
						process = e.runProcess(status)
					} else {
						needs_run = true
					}
				}
			case err, more := <-status:
				if more {
					fmt.Println("run err", err)
					process = nil
					if in_stop {
						in_stop = false
						reply := Cmd{Method: cmdStopReply, TargetId: stop_msg.SenderId}
						rmsg := reply.AsMsg()
						rmsg.SetInt(blockIdKey, stop_msg.MustGetInt(blockIdKey))
						owner.SendMsg(rmsg, stop_msg.SenderId)
					} else if needs_run {
						needs_run = false
						process = e.runProcess(status)
					} else if err == nil {
						// Process completed successfully
						e.SendMsg(Msg{})
					}
				} else {
					// The channel closed for some unknown reason,
					// that's an error but shouldn't shut down the graph.
					killExec(process)
					process = nil
					fmt.Println("exec error: channel closed, cause unknown")
				}
			}
		}
	}(a.Owner, done, control, merge)

	return nil
}

func (e *Exec) startMerge(a StartArgs) (chan Msg, error) {
	// Must have 1 someone upstream, or else this is useless.
	// However, this isn't currently an error, it's just an orphan
	// node that won't run.
	if len(e.input.Out) != 1 {
		return nil, nil
	}

	done := a.Owner.DoneChannel()
	// A forwarding channel to the main go function.
	merge := make(chan Msg)

	// Route incoming messages through a timer, which prevents multiple calls
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}

	a.NodeWaiter.Add(1)
	go func(timer *time.Timer, merge chan Msg, done chan int) {
		defer a.NodeWaiter.Done()
		defer close(merge)

		var last Msg

		// An extremely simply handling of the timer right now --
		// it will get retriggered as long as I receive new events,
		// only firing once that stops. Need to improve this so
		// it will always fire after a small delay, even if it's
		// still receiving events.
		for {
			select {
			case _, more := <-done:
				if !more {
					return
				}
			case msg, more := <-e.input.Out[0]:
				if more {
					last = msg
					timer.Reset(100 * time.Millisecond)
				}
			case <-timer.C:
				merge <- last
			}
		}
	}(timer, merge, done)
	return merge, nil
}

func (e *Exec) startCmds(a StartArgs, merge chan Msg) (chan Msg, error) {
	done := a.Owner.DoneChannel()
	control, controlId := a.Owner.NewControlChannel(0)
	if control == nil {
		return nil, errors.New("Can't make control channel for cmds")
	}

	// A forwarding channel to the main go function.
	cmds := make(chan Msg)

	a.NodeWaiter.Add(1)
	go func(owner Owner, merge chan Msg, done chan int, cmds chan Msg, control chan Msg, controlId Id) {
		defer a.NodeWaiter.Done()
		defer close(cmds)

		handler := newHandleFromCmds(owner, controlId, e.CmdList, cmds)

		for {
			select {
			case _, more := <-done:
				if !more {
					return
				}
			case msg, more := <-merge:
				if more {
					handler.handle(&msg, fromMerge)
				}
			case msg, more := <-control:
				if more {
					handler.handle(&msg, fromControl)
				}
			}
		}
	}(a.Owner, merge, done, cmds, control, controlId)
	return cmds, nil
}

func (e *Exec) runProcess(c chan error) *exec.Cmd {
	proc := e.newCmd()
	if proc == nil {
		return nil
	}
	go func(proc *exec.Cmd) {
		fmt.Println("*****run exec:", e.Cmd, "path", proc.Path, "dir", proc.Dir)
		c <- proc.Run()
		fmt.Println("*****exec finished")
	}(proc)
	return proc
}

func (e *Exec) newCmd() *exec.Cmd {
	cmd := exec.Command(e.Cmd)
	if cmd == nil {
		fmt.Println("exec error: Couldn't create cmd")
		return nil
	}
	cmd.Dir = e.Dir
	if len(e.Args) > 0 {
		cmd.Args = append(cmd.Args, e.Args)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func killExec(cmd *exec.Cmd) {
	fmt.Println("kill 1")
	if cmd != nil && cmd.Process != nil {
		fmt.Println("kill 2")
		cmd.Process.Kill()
	}
}

// -----------------------------------------------
// handleFromCmds struct
// Handle messages received from the command func.
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
