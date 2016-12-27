package main

import (
	"flag"
	"fmt"
	"github.com/hackborn/ghost/graph"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type Runner struct {
	Cmd exec.Cmd
}

func main() {
	g, gerr := loadGraph()
	if gerr != nil {
		fmt.Println("Error loading graph:", gerr)
		return
	}
	if g == nil {
		fmt.Println("Unknown error loading graph:")
		return
	}

	done := make(chan bool)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		//        cleanup()
		fmt.Println("signal quit")
		done <- true
	}()
	/*
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func(){
		    for sig := range c {
		    	fmt.Println("GOT SIGNAL", sig)
		        // sig is a ^C, handle it
		        log.Fatal("signal")
		    }
		}()
	*/
	fmt.Println("after signal notify")

	//	runBroomServer()
	fmt.Println("after RUN")
	g.Start()
	fmt.Println("after START")
	<-done
	g.Stop()
	fmt.Println("done, son!")
}

func loadGraph() (*graph.Graph, error) {
	// 1. Load graph based on the command line
	// Provide defaults for now
	graph_name := "gulp"
	if len(os.Args) > 1 {
		graph_name = os.Args[1]
	}
	return graph.Load(graph_name, loadCla)
}

// Load graph arguments from the command line.
func loadCla(args *graph.Args) {
	if len(os.Args) <=2 {
		return
	}
	fs := flag.NewFlagSet("fs", flag.ContinueOnError)
	var values []*string
	for _, a := range args.Arg {
		values = append(values, fs.String(a.XMLName.Local, a.Value, a.Usage))
	}
	fs.Parse(os.Args[2:])
	for i := 0; i < len(values); i++ {
		args.Arg[i].Value = *(values[i])
	}
}

func buildBroomTools() {
	go func() {
		cmd := exec.Command("go", "build")
		cmd.Dir = "C:/work/dev/go/src/github.com/hackborn/hello"
		err := cmd.Start()
		if err != nil {
			log.Println("Build command Start() failed:", err)
			log.Fatal(err)
		} else {
			log.Printf("Waiting for build to finish...\n")
			err = cmd.Wait()
			log.Printf("Build finished with error: %v\n", err)
		}
	}()
}

func runBroomServer() {
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
}

func runBroomServerOld() {
	go func() {
		cmd := exec.Command("C:/work/dev/go/src/github.com/hackborn/broom_server/server/server.exe")
		cmd.Dir = "C:/work/dev/go/src/github.com/hackborn/broom_server/server"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		/*
			stdout, err := cmd.StdoutPipe()

			if err := cmd.Start(); err == nil {
				b, _ := ioutil.ReadAll(cmd.Stdout)
				fmt.Println("server:", string(b))
			}
			fmt.Println("done with server, err:", err)
		*/
		/*
		   //		if err == nil {
		   			err := cmd.Start()
		   //		}
		   		if err != nil {
		   			log.Println("runBroomServer() failed:", err)
		   			log.Fatal(err)
		   		} else {
		   			log.Printf("Waiting for runBroomServer to finish...\n")
		   			err = cmd.Wait()
		   			log.Printf("runBroomServer finished with error: %v\n", err)
		   		}
		*/
	}()
}
