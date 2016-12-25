package node

import (
	"fmt"
)

// Run a command.
type Exec struct {
	Name string		`xml:"name,attr"`
	Cmd string		`xml:"cmd,attr"`
	Args string		`xml:"args,attr"`
	Dir string		`xml:"dir,attr"`
	output Output
}

func (n *Exec) IsValid() bool {
	return len(n.Cmd) > 0
}

func (e *Exec) Connect() (chan Msg) {
	return e.output.Add()
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