package graph

import (
	//	"fmt"
	"encoding/xml"
	"strings"
	"sync"
	"github.com/hackborn/ghost/node"
)

type Arg struct {
	// As far as I can tell there's no xml tag to directly grab the name of the XML tag.
	XMLName xml.Name
	Value   string `xml:",chardata"`
	Usage   string `xml:"usage,attr"`
}

type Args struct {
	Arg []Arg `xml:",any"`
}

// A generic interface for modifying a string.
func (a Args) ChangeString(s string) string {
	for _, v := range a.Arg {
		s = strings.Replace(s, "${"+v.XMLName.Local+"}", v.Value, -1)
	}
	return s
}

type LoadArgs func(*Args)

// The complete graph.
type Graph struct {
	Args Args
	// All nodes that were created for the graph.
	_nodes []graphnode
	// The collection of channels to my root nodes
	output     node.Channels
	nodeWaiter sync.WaitGroup
}

func (g *Graph) ApplyArgs(cs node.ChangeString) {
	for i := 0; i < len(g._nodes); i++ {
		dst := &(g._nodes[i])
		if dst.node != nil {
			dst.node.ApplyArgs(cs)
		}
	}
}

func (g *Graph) NewChannel() chan node.Msg {
	return g.output.NewChannel()
}

func (g *Graph) add(n node.Node) {
	var gn graphnode
	gn.node = n
	g._nodes = append(g._nodes, gn)
}

func (g *Graph) addInput(n node.Node, s node.Source) {
	for i := 0; i < len(g._nodes); i++ {
		dst := &(g._nodes[i])
		if dst.node == n {
			// XXX Check that the source is not a duplicate
			dst.inputs = append(dst.inputs, s)
		}
	}
}

func (g *Graph) Start() {
	g.Stop()

	a := node.StartArgs{&g.nodeWaiter}

	// Create all the channel connections
	for i := 0; i < len(g._nodes); i++ {
		n := &(g._nodes[i])
		if n.node != nil && len(n.inputs) > 0 {
			n.node.StartChannels(a, n.inputs)
		}
	}

	// Start each node
	for i := 0; i < len(g._nodes); i++ {
		n := &(g._nodes[i])
		if n.node != nil {
			n.node.StartRunning(a)
		}
	}
}

func (g *Graph) Stop() {
	g.output.CloseChannels()
	g.nodeWaiter.Wait()
}

// Describe the structure of the graph.
type graphnode struct {
	node   node.Node
	inputs []node.Source
}
