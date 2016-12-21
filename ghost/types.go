package ghost

// Data sent out to Node.RequestAccess.
type RequestData struct {
	// Any files we're requesting access to.
	Files	[]string
}

// A single node in the processing graph.
type Node interface {
	Stop()
	// Received when a node is 
	RequestAccess(data *RequestData)
}

type Graph struct {
	Nodes	[]Node
}

