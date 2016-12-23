package graph

// Data sent out to Node.RequestAccess.
type RequestData struct {
	// Any files we're requesting access to.
	Files	[]string
}

// A message passed between nodes.
type Msg struct {
}

// A single node in the processing graph.
type Node interface {
	Stop()
	// Received when a node is 
	RequestAccess(data *RequestData)
}

type Arg struct {
	Key string
	Value string
}

type Graph struct {
	Args	[]Arg
	Nodes	[]Node
}

