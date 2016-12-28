package node

import (
	"fmt"
	"os"
	"os/exec"
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
	if len(e.input.Out) != 1 {
		return nil
	}
	fmt.Println("Start exec", e, "ins", len(e.input.Out), "outs", len(e.Out))

	// The cmd runs in a separate gofunc. This channel communicates back to the main func,
	// returning the result of the run (specifically, exec.Cmd.Wait()).
	status := make(chan error)

	cmd := exec.Command(e.Cmd)
	cmd.Dir = e.Dir
	if len(e.Args) > 0 {
		cmd.Args = append(cmd.Args, e.Args)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	a.NodeWaiter.Add(1)
	go func() {
		var cmd *exec.Cmd = nil
		fmt.Println("start exec func")
		defer a.NodeWaiter.Done()
		defer fmt.Println("end exec func")
		defer execCleanup(cmd)
		defer e.CloseChannels()
		needs_run := false

		for {
			select {
			case msg, more := <-e.input.Out[0]:
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
	}()

	return nil
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
		c<-cmd.Run()
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

func (e *Exec) Start(a StartArgs, inputs []Source) {
	fmt.Println("exec cmd", e.Cmd, "args", e.Args, "dir", e.Dir)
	if len(inputs) != 1 {
		fmt.Println("node.Exec.Start() must have 1 input (for now)", len(inputs))
		return
	}

	/*
		cmd := exec.Command("C:/work/dev/go/src/github.com/hackborn/broom_server/server/server.exe")
		cmd.Dir = "C:/work/dev/go/src/github.com/hackborn/broom_server/server"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		//	cmd.Run()

		// How to stop:
		// http://stackoverflow.com/questions/11886531/terminating-a-process-started-with-os-exec-in-golang
		done := make(chan error, 1)
		go func() {
			done <- cmd.Run()
		}()
		select {
		case <-time.After(10 * time.Second):
			if err := cmd.Process.Kill(); err != nil {
				log.Fatal("failed to kill: ", err)
			}
			log.Println("process killed as timeout reached")
		case err := <-done:
			if err != nil {
				log.Printf("process done with error = %v", err)
			} else {
				log.Print("process done gracefully without error")
			}
		}
	*/
}

func (e *Exec) RequestAccess(data *RequestArgs) {
}
