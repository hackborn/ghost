package main

import (
	"flag"
	"fmt"
	"github.com/hackborn/ghost/graph"
	"os"
	"os/signal"
	"syscall"
)

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

	g.Start()
	<-done
	g.Stop()
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
	if len(os.Args) <= 2 {
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
