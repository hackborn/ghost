package graph

import (
//	"fmt"
	"github.com/hackborn/ghost/node"
)

// A user argument to the graph.
type Arg struct {
	Key string
	Value string
}

// The complete graph.
type Graph struct {
	Args	[]Arg
	// All nodes that were created for the graph.
	_nodes	[]graphnode
	done	chan bool
	// The collection of channels to my root nodes
	output	node.Output
}

func (g *Graph) Connect() (chan node.Msg) {
	return g.output.Add()
}

func (g *Graph) add(n node.Node) {
	var gn graphnode
	gn.node = n
	g._nodes = append(g._nodes, gn)
}

func (g *Graph) addInput(n node.Node, s node.Source) {
	for i := 0 ; i < len(g._nodes); i++ {
		dst := &(g._nodes[i])
		if dst.node == n {
			// XXX Check that the source is not a duplicate
			dst.inputs = append(dst.inputs, s)
		}
	}
}

func (g *Graph) Start() {
	g.Stop()
	g.done = make(chan bool)
	for i := 0; i < len(g._nodes); i++ {
		n := &(g._nodes[i])
		if n.node != nil && len(n.inputs) > 0 {
			n.node.Start(n.inputs)
		}
	}
}

func (g *Graph) Stop() {
	// XXX This is a non-blocking send, but it's currently unclear
	// to me if everyone listening to the channel will get the message,
	// or only 1 listener.
	select {
		case g.done<-true:
		default:
	}
	// I have no idea how to wait for everyone to stop
	// Now I know -- sync.WaitGroup
}

// Describe the structure of the graph.
type graphnode struct {
	node		node.Node
	inputs		[]node.Source
}
