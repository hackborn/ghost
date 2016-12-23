package graph

import (
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
	Nodes	[]*node.Node
}

