package node

import (
	"fmt"
	"sync"
)

// Data sent out to Node.RequestAccess.
type RequestArgs struct {
	// Any files we're requesting access to.
	Files []string
}

// Data sent out to Node.Start.
type StartArgs struct {
	Owner Owner
	// A graph lock so we can wait for all nodes to finish running.
	NodeWaiter *sync.WaitGroup
}

// A message passed between nodes.
type Msg struct {
}

// Provide access to the source feeding into a node.
// Sources can have multiple channels (but currently won't).
type Source interface {
	// Create and answer a new channel (adding it to the source).
	NewChannel() chan Msg
}

// A node owner. Provide an API for various functions and a channel
// to receive control events.
type Owner interface {
	// Create and answer a new channel (adding it to the source).
	NewControlChannel() chan Msg

	// Request access to a given resource. The control channel
	// will receive notification when access has been given.
	RequestAccess(resources []string)
}

// Bundle behaviour for managing Node input/output channels.
type Channels struct {
	Out []chan Msg
}

// A generic interface for modifying a string.
type ChangeString interface {
	ChangeString(s string) string
}

// A single node in the processing graph.
type Node interface {
	ApplyArgs(cs ChangeString)
	NewChannel() chan Msg
	// Starting the graph goes through a two-step process on each
	// node: First all channels are generated, then they are run.
	StartChannels(a StartArgs, inputs []Source)
	StartRunning(a StartArgs) error
	// Received when a node is
	RequestAccess(data *RequestArgs)
}

func (cs *Channels) Add(c chan Msg) {
	if c != nil {
		cs.Out = append(cs.Out, c)
	}
}

func (cs *Channels) NewChannel() chan Msg {
	c := make(chan Msg)
	cs.Add(c)
	return c
}

// Close and clear out my channels.
func (cs *Channels) CloseChannels() {
	for _, c := range cs.Out {
		close(c)
	}
	cs.Out = cs.Out[:0]
}

// Close and clear out my channels.
func (cs *Channels) SendMsg(msg Msg) {
	for _, c := range cs.Out {
		c <- msg
	}
}

// Close and clear out my channels.
func (cs *Channels) Test() {
	fmt.Println("test channels")
	for _, c := range cs.Out {
		fmt.Println("\ttest channel")
		c <- Msg{}
	}
}
