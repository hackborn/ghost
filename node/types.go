package node

// Data sent out to Node.RequestAccess.
type RequestArgs struct {
	// Any files we're requesting access to.
	Files	[]string
}

// A message passed between nodes.
type Msg struct {
}

// Provide access to the source feeding into a node.
// Sources can have multiple channels (but currently won't).
type Source interface {
	// Create the channel at the index. Nil for no channel.
	Connect() (chan Msg)
}

// Bundle behaviour for managing Node output channels.
type Output struct {
	Out []chan Msg
}

// A single node in the processing graph.
type Node interface {
	Connect() (chan Msg)
	// The channel is sent true when the node should stop.
	Start(inputs []Source)
	// Received when a node is 
	RequestAccess(data *RequestArgs)
}

// Bundle behaviour for managing Node output channels.
func (o *Output) Add() (chan Msg) {
	c := make(chan Msg)
	o.Out = append(o.Out, c)
	return c
}
