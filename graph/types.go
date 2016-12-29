package graph

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/hackborn/ghost/node"
	"strings"
	"sync"
)

type Arg struct {
	// As far as I can tell there's no xml tag to directly grab the name of the XML tag.
	XMLName xml.Name
	Value   string `xml:",chardata"`
	Usage   string `xml:"usage,attr"`
	// This has been added to Go1.8. When that's released, I can
	// use this to simplify (i.e. eliminate) a lot of the parsing.
	//	Attrs   []xml.Attr `xml:",any,attr"`
}

type Args struct {
	Arg []Arg `xml:",any"`
}

type Macro struct {
	// As far as I can tell there's no xml tag to directly grab the name of the XML tag.
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type Macros struct {
	List []Macro `xml:",any"`
}

// A generic interface for modifying a string.
func (a Args) ChangeString(s string) string {
	for _, v := range a.Arg {
		s = strings.Replace(s, "${"+v.XMLName.Local+"}", v.Value, -1)
	}
	return s
}

func (m Macros) ChangeString(s string) string {
	for _, v := range m.List {
		s = strings.Replace(s, "${"+v.XMLName.Local+"}", v.Value, -1)
	}
	return s
}

type LoadArgs func(*Args)

// The complete graph.
type Graph struct {
	Args   Args
	Macros Macros
	// All nodes that were created for the graph.
	_nodes []graphnode
	// The collection of channels to my root nodes
	output     node.Channels
	control    node.Channels
	// Registered control channels.
	regctrl		map[node.Id]chan node.Msg
	nodeWaiter sync.WaitGroup
}

func (m *Macros) Expand(cs node.ChangeString) {
	for i := 0; i < len(m.List); i++ {
		dst := &(m.List[i])
		dst.Value = cs.ChangeString(dst.Value)
	}
}

func (g *Graph) Expand(cs node.ChangeString) {
	for i := 0; i < len(g._nodes); i++ {
		dst := &(g._nodes[i])
		if dst.node != nil {
			dst.node.ApplyArgs(cs)
		}
	}
}

// Source interface
func (g *Graph) NewChannel() chan node.Msg {
	return g.output.NewChannel()
}

// Owner interface
func (g *Graph) NewControlChannel(id node.Id) chan node.Msg {
	c := g.control.NewChannel()
	if id > 0 && c != nil {
		if g.regctrl == nil {
			g.regctrl = make(map[node.Id]chan node.Msg)
		}
		g.regctrl[id] = c
	}
	return c
}

// Owner interface
func (g *Graph) SendCmd(cmd node.Cmd, source node.Id) error {
	fmt.Println("send", cmd)
	for i := 0; i < len(g._nodes); i++ {
		dst := g._nodes[i].node
		if dst != nil && dst.GetId() == cmd.TargetId {
			fmt.Println("found")
			c, _ := g.regctrl[cmd.TargetId]
			if c != nil {
				c<-cmd.AsMsg()
			}
			return nil
		}

	}
	return errors.New("No target")
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

	a := node.StartArgs{g, &g.nodeWaiter}

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
	g.control.CloseChannels()
	g.output.CloseChannels()
	g.nodeWaiter.Wait()
	fmt.Println("graph stopped")
}

// Describe the structure of the graph.
type graphnode struct {
	node   node.Node
	inputs []node.Source
}
