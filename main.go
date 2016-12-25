package main

// A little reading I need to do:
// https://tour.golang.org/concurrency/2
// https://blog.golang.org/pipelines

import (
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/hackborn/ghost/graph"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Runner struct {
	Cmd exec.Cmd
}

// Take an arg of the pattern "-str" and
// clean it up -- trim hyphens and white space.
func cleanArg(s string) string {
	return strings.Trim(s, "-")
}

func makeArgs() (string, map[string]string) {
	// This is a hybrid of the raw os args and the flag parsing.
	tmp := make(map[string]*string)
	for _, a := range os.Args[1:] {
		i := strings.Index(a, "=")
		if i > -1 {
			key := cleanArg(a[:i])
			tmp[key] = flag.String(key, "", "")
		}
	}
	flag.Parse()

	// Compile the parsed flags into a map.
	cfg := ""
	m := make(map[string]string)
	for k, v := range tmp {
		if k == "config" {
			cfg = *v
		} else {
			m[k] = *v
		}
	}
	return cfg, m
}

func main() {
//	cfg, args := makeArgs()
//	fmt.Println("CFG", cfg)
//	fmt.Println("ARGS", args)

	//	g, _ := graph.LoadFile("sds")
	g, gerr := graph.Load("gulp")
	if gerr != nil {
		fmt.Println("Error loading graph:", gerr)
		return
	}
	if g ==  nil {
		fmt.Println("Unknown error loading graph:")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		//        cleanup()
		fmt.Println("signal quit")
		done<-true
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

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add("C:/work/dev/go/src/github.com/hackborn/ghost")
	if err != nil {
		log.Fatal(err)
	}

//	runBroomServer()
	fmt.Println("after RUN")
	g.Start()
	fmt.Println("after START")
	<-done
	fmt.Println("done, son!")
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
