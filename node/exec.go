package node

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
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
	//	input     Channels
	Channels // Output
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
	// No inputs means this node is never hit, so ignore.
	if len(inputs) <= 0 {
		return nil, nil
	}

	// If we want multiple inputs, we'll need to expand the running
	// code to handle merging.
	if len(inputs) != 1 {
		fmt.Println("node.Exec.Start() must have 1 input (for now)", len(inputs))
		return nil, errors.New("node.Exec.Start does not support multiple inputs")
	}

	data := prepareDataExec{}
	data.mainControlChan, _ = p.NewControlChannel(e.Id)
	if data.mainControlChan == nil {
		return nil, errors.New("node.Exec can't make control channel")
	}
	data.mergeChan = make(chan Msg)
	if data.mergeChan == nil {
		return nil, errors.New("node.Exec can't make merge channel")
	}
	if len(e.CmdList) > 0 {
		data.cmdChan = make(chan Msg)
		if data.cmdChan == nil {
			return nil, errors.New("node.Exec can't make cmd channel")
		}
		data.cmdControlChan, data.cmdControlChanId = p.NewControlChannel(0)
		if data.cmdControlChan == nil {
			return nil, errors.New("node.Exec can't make cmd control channel")
		}
	}

	for _, i := range inputs {
		data.input.Add(i.NewChannel())
	}

	return data, nil
}

func (e *Exec) Start(s Start, idata interface{}) error {
	data, ok := idata.(prepareDataExec)
	if !ok {
		return errors.New("node.Exec no prepareData")
	}
	if len(data.input.Out) != 1 {
		return errors.New("node.Exec no inputs")
	}
	fmt.Println("Start exec", e, "ins", len(data.input.Out), "outs", len(e.Out))

	inputChan := data.mergeChan
	err := e.startMerge(s, data)
	if err != nil {
		return err
	}
	// If we have any commands, insert a handling stage between merge and my exec routine.
	if data.cmdChan != nil {
		err = e.startCmds(s, data, inputChan)
		if err != nil {
			return err
		}
		inputChan = data.cmdChan
	}

	// The cmd runs in a separate gofunc. This channel communicates back to the main func,
	// returning the result of the run (specifically, exec.Cmd.Wait()).
	status := make(chan error)

	done := s.GetDoneChannel()
	waiter := s.GetDoneWaiter()
	waiter.Add(1)
	go func(owner Owner, done chan struct{}, waiter *sync.WaitGroup, data prepareDataExec, inputChan chan Msg) {
		var process *exec.Cmd = nil

		defer waiter.Done()
		defer fmt.Println("end exec func")
		defer killExec(process)
		defer e.CloseChannels()

		fmt.Println("start exec func")

		needs_run := false
		in_stop := false
		var stop_msg Msg

		if e.Autorun {
			process = e.runProcess(status)
		}

		for {
			select {
			case <-done:
				return
			case msg, more := <-data.mainControlChan:
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
			case msg, more := <-inputChan:
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
	}(s.GetOwner(), done, waiter, data, inputChan)

	return nil
}

func (e *Exec) startMerge(s Start, data prepareDataExec) error {
	// Route incoming messages through a timer, which prevents multiple calls
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}

	done := s.GetDoneChannel()
	waiter := s.GetDoneWaiter()

	waiter.Add(1)
	go func(done chan struct{}, waiter *sync.WaitGroup, timer *time.Timer, data prepareDataExec) {
		defer waiter.Done()
		defer close(data.mergeChan)

		var last Msg

		// An extremely simply handling of the timer right now --
		// it will get retriggered as long as I receive new events,
		// only firing once that stops. Need to improve this so
		// it will always fire after a small delay, even if it's
		// still receiving events.
		for {
			select {
			case <-done:
				return
			case msg, more := <-data.input.Out[0]:
				if more {
					last = msg
					timer.Reset(100 * time.Millisecond)
				}
			case <-timer.C:
				data.mergeChan <- last
			}
		}
	}(done, waiter, timer, data)
	return nil
}

func (e *Exec) startCmds(s Start, data prepareDataExec, inputChan chan Msg) error {
	done := s.GetDoneChannel()
	waiter := s.GetDoneWaiter()

	waiter.Add(1)
	go func(done chan struct{}, waiter *sync.WaitGroup, owner Owner, data prepareDataExec, inputChan chan Msg) {
		defer waiter.Done()
		defer close(data.cmdChan)

		handler := newHandleFromCmds(owner, data.cmdControlChanId, e.CmdList, data.cmdChan)

		for {
			select {
			case <-done:
				return
			case msg, more := <-inputChan:
				if more {
					handler.handle(&msg, fromMerge)
				}
			case msg, more := <-data.cmdControlChan:
				if more {
					handler.handle(&msg, fromControl)
				}
			}
		}
	}(done, waiter, s.GetOwner(), data, inputChan)
	return nil
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

// -----------------------------------------------
// prepareDataExec struct
// Store data generate in the Prepare.
type prepareDataExec struct {
	input Channels
	// Forwarding channels to the main go function.
	mergeChan chan Msg
	cmdChan   chan Msg
	// Control channels for my various stages.
	mainControlChan  chan Msg
	cmdControlChan   chan Msg
	cmdControlChanId Id
}
