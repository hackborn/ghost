package node

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Run a command.
type Exec struct {
	Name      string `xml:"name,attr"`
	Cmd       string `xml:"cmd,attr"`
	Args      string `xml:"args,attr"`
	Dir       string `xml:"dir,attr"`
	Interrupt bool   `xml:"interrupt,attr"`
	Rerun     bool   `xml:"rerun,attr"`
	input     Channels
	Channels  // Output
}

func (n *Exec) IsValid() bool {
	return len(n.Cmd) > 0
}

func (e *Exec) ApplyArgs(cs ChangeString) {
	e.Cmd = cs.ChangeString(e.Cmd)
	e.Args = cs.ChangeString(e.Args)
	e.Dir = cs.ChangeString(e.Dir)
}

func (e *Exec) StartChannels(a StartArgs, inputs []Source) {
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

	e.input.CloseChannels()
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

	control := a.Owner.NewControlChannel()
	if control == nil {
		return errors.New("Can't make control channel")
	}

	// The cmd runs in a separate gofunc. This channel communicates back to the main func,
	// returning the result of the run (specifically, exec.Cmd.Wait()).
	status := make(chan error)

	a.NodeWaiter.Add(1)
	go func(control chan Msg, merge chan Msg) {
		var cmd *exec.Cmd = nil
		fmt.Println("start exec func")
		defer a.NodeWaiter.Done()
		defer fmt.Println("end exec func")
		defer execCleanup(cmd)
		defer e.CloseChannels()
		needs_run := false

		for {
			select {
			case _, more := <-control:
				if more {
					// XXX Handle control message
				} else {
					return
				}
			case msg, more := <-merge:
				if more {
					fmt.Println("exec msg", msg)
					if cmd == nil {
						cmd = e.startCmd(status)
					} else {
						needs_run = true
					}
				} else {
					return
				}
			case runerr, more := <-status:
				if more {
					fmt.Println("run err", runerr)
					execCleanup(cmd)
					cmd = nil
					if needs_run {
						needs_run = false
						cmd = e.startCmd(status)
					}
				} else {
					// The channel closed for some unknown reason,
					// that's an error but shouldn't shut down the graph.
					execCleanup(cmd)
					cmd = nil
					fmt.Println("exec error: channel closed, cause unknown")
				}
			}
		}
	}(control, merge)

	return nil
}

func (e *Exec) startMerge(a StartArgs) (chan Msg, error) {
	// Must have 1 someone upstream, or else this is useless.
	// However, this isn't currently an error, it's just an orphan
	// node that won't run.
	if len(e.input.Out) != 1 {
		return nil, nil
	}

	control := a.Owner.NewControlChannel()
	if control == nil {
		return nil, errors.New("Can't make control channel")
	}

	// The cmd runs in a separate gofunc. This channel communicates back to the main func,
	// returning the result of the run (specifically, exec.Cmd.Wait()).
	merge := make(chan Msg)

	// Route incoming messages through a timer, which prevents multiple calls
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}

	a.NodeWaiter.Add(1)
	go func(timer *time.Timer, merge chan Msg, control chan Msg) {
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
			case msg, more := <-control:
				if more {
					merge <- msg
				} else {
					return
				}
			case msg, more := <-e.input.Out[0]:
				if more {
					last = msg
					timer.Reset(100 * time.Millisecond)
				} else {
					return
				}
			case <-timer.C:
				merge <- last
			}
		}
	}(timer, merge, control)
	return merge, nil
}

func execCleanup(cmd *exec.Cmd) {
	fmt.Println("cleanup 1")
	if cmd != nil && cmd.Process != nil {
		fmt.Println("cleanup 2")
		cmd.Process.Kill()
	}
}

func (e *Exec) startCmd(c chan error) *exec.Cmd {
	cmd := e.createCmd()
	if cmd == nil {
		return nil
	}
	go func() {
		fmt.Println("*****run exec", e.Cmd)
		c <- cmd.Run()
	}()
	return cmd
}

func (e *Exec) createCmd() *exec.Cmd {
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

func (e *Exec) RequestAccess(data *RequestArgs) {
}
