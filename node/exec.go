package node

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// execfini is the result of a finished exec.Cmd.Run(). It bundles
// the error value returned from Run() with a counter to identify the run.
type execfini struct {
	runId int
	err   error
}

// Exec runs a command. The command runs in a separate gofunc, spawned from
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
	LogList   []Logt `xml:"log"`
	//	input     Channels
	Channels // Output
	Cmds
}

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
	for i := 0; i < len(e.LogList); i++ {
		v := &e.LogList[i]
		v.Text = cs.ChangeString(v.Text)
	}
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
	data.mainFiniChan = make(chan execfini)
	if data.mainFiniChan == nil {
		return nil, errors.New("node.Exec can't make fini channel")
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
	//	fmt.Println("Start exec", e, "ins", len(data.input.Out), "outs", len(e.Out))

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

	done := s.GetDoneChannel()
	waiter := s.GetDoneWaiter()
	waiter.Add(1)
	go func(owner Owner, done <-chan struct{}, waiter *sync.WaitGroup, data prepareDataExec, inputChan chan Msg) {
		proc := process{e.Cmd, e.Args, e.Dir, 0, nil}
		handler := newHandleFromMain(owner, &proc, data.mainFiniChan, e)

		defer waiter.Done()
		defer debug("end exec main %v", e.Id)
		defer proc.close()
		defer e.CloseChannels()

		debug("start exec main %v name=%v", e.Id, e.Name)

		if e.Autorun {
			proc.run(data.mainFiniChan, e.LogList)
		}

		for {
			select {
			case <-done:
				return
			case msg, more := <-data.mainControlChan:
				if more {
					handler.handleMsg(&msg, fromControl)
				}
			case msg, more := <-inputChan:
				if more {
					handler.handleMsg(&msg, fromInput)
				}
			case fini, more := <-data.mainFiniChan:
				if more {
					handler.handleFini(fini, fromStatus)
				} else {
					// The channel closed for some unknown reason,
					// that's an error but shouldn't shut down the graph.
					proc.close()
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
	go func(done <-chan struct{}, waiter *sync.WaitGroup, timer *time.Timer, data prepareDataExec) {
		defer waiter.Done()
		defer debug("end exec merge %v", e.Id)
		defer close(data.mergeChan)

		debug("start exec merge %v", e.Id)
		var last Msg

		// An extremely simple handling of the timer right now --
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
	go func(done <-chan struct{}, waiter *sync.WaitGroup, owner Owner, data prepareDataExec, inputChan <-chan Msg) {
		defer waiter.Done()
		defer debug("end exec cmds %v", e.Id)
		defer close(data.cmdChan)

		debug("start exec cmds %v", e.Id)

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

// prepareDataExec stores data generated in the Prepare.
type prepareDataExec struct {
	input Channels
	// Forwarding channels to the main go function.
	mergeChan chan Msg
	cmdChan   chan Msg
	// Control channels for my various stages.
	mainControlChan  chan Msg
	cmdControlChan   chan Msg
	cmdControlChanId Id
	// The cmd runs in a separate gofunc. This channel communicates back to the main func,
	// returning the result of the run (specifically, exec.Cmd.Run()).
	mainFiniChan chan execfini
}

// process manages an exec cmd.
type process struct {
	cmdStr string
	argStr string
	dirStr string
	// The ID for the current run
	runId int
	cmd   *exec.Cmd
}

func (p *process) isRunning() bool {
	return p.cmd != nil
}

func (p *process) stop() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	p.cmd = nil
}

func (p *process) close() {
	p.stop()
}

// finished() is set when the cmd status channel has reported completion.
// Answer true if I handled this fini, false if I discarded it.
func (p *process) finished(fini execfini) bool {
	// Discard if I'm from a previous run
	if fini.runId != p.runId {
		debug("exec.process.finished() discard previous run (current=%v, received=%v)", p.runId, fini.runId)
		return false
	}
	p.cmd = nil
	return true
}

func (p *process) run(c chan execfini, logs []Logt) {
	// XXX We're just ignoring if the current one is running.
	// Should this try and kill it?
	p.cmd = p.newCmd()
	if p.cmd == nil {
		return
	}
	p.runId++
	go func(runId int, proc *exec.Cmd, c chan execfini, logs []Logt) {
		for _, v := range logs {
			fmt.Println(v.Text)
		}
		c <- execfini{runId, proc.Run()}
	}(p.runId, p.cmd, c, logs)
}

func (p *process) newCmd() *exec.Cmd {
	cmd := exec.Command(p.cmdStr)
	if cmd == nil {
		fmt.Println("exec error: Couldn't create exec.Command")
		return nil
	}
	cmd.Dir = p.dirStr
	if len(p.argStr) > 0 {
		for _, v := range formatArgs(p.argStr) {
			cmd.Args = append(cmd.Args, v)
		}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// formatArgs is I hope a hack: The user specifies args
// as a single string, with spaces. However the cmd.Run() chokes
// on this; it needs each arg as a separate entry in a slice. So
// take a quick stab at something that should get us there: The
// source list is parsed, separating by space, unless we're in quotes
func formatArgs(_in string) []string {
	in := strings.TrimSpace(_in)
	out := []string{}
	in_dquote := false
	start := 0
	for i := 0; i < len(in); i++ {
		c := string(in[i])
		if c == " " && !in_dquote {
			if i-start > 0 {
				out = append(out, in[start:i])
			}
			start = i + 1
		} else if c == "\"" {
			in_dquote = !in_dquote
		}
	}
	if len(in) > start {
		out = append(out, in[start:len(in)])
	}
	return out
}
