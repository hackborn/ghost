package node

import (
	"encoding/xml"
	"fmt"
	"sync"
)

// > 0 is a valid ID
// == 0 is unassigned
// < 0 is undefined
type Id int

// Specific commands sent between nodes
type Cmd struct {
	XMLName  xml.Name
	Method   string `xml:"method,attr"`
	Target   string `xml:"target,attr"`
	TargetId Id
	Reply    bool `xml:"reply,attr"`
}

type Cmds struct {
	CmdList []Cmd `xml:",any"`
}

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
	Values map[string]interface{}
}

// Provide access to the source feeding into a node.
// Sources can have multiple channels (but currently won't).
type Source interface {
	// Create and answer a new channel (adding it to the source).
	NewChannel() chan Msg
}

// Answer an Id for a given name,
// XXX Should be deprecated with 1.8.
type GetId interface {
	// Create and answer a new channel (adding it to the source).
	GetId(name string) Id
}

// A node owner. Provide an API for various functions and a channel
// to receive control events.
type Owner interface {
	// Answer a new channel that determines whether the graph is running.
	// Note that done channels will always be paired with adding to the
	// StartArgs NodeWaiter.
	NewDoneChannel() chan int
	// Create and answer a new channel (adding it to the source).
	// If the ID is > 0 then this will be registered as the control
	// channel for the node.
	NewControlChannel(id Id) chan Msg

	// Send a command from a source.
	SendCmd(cmd Cmd, source Id) error
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
	GetId() Id
	GetName() string

	ApplyArgs(cs ChangeString)
	NewChannel() chan Msg
	// Starting the graph goes through a two-step process on each
	// node: First all channels are generated, then they are run.
	StartChannels(a StartArgs, inputs []Source)
	StartRunning(a StartArgs) error

	// Set target IDs (for commands)
	// XXX I think this will be deprecated when I rewrite XML load with 1.8.
	FillIds(get GetId)

	// Received when a node is
	// XXX Should be deprecated
	RequestAccess(data *RequestArgs)
}

func CmdFromMsg(m Msg) *Cmd {
	t := m.MustGetString("type")
	if t != "cmd" {
		return nil
	}
	var c Cmd
	c.Method = m.MustGetString("method")
	return &c
}

func (c *Cmd) AsMsg() Msg {
	var m Msg
	m.Values = make(map[string]interface{})
	m.Values["type"] = "cmd"
	if c.Method != "" {
		m.Values["method"] = c.Method
	}
	return m
}

func (c *Cmds) FillIds(get GetId) {
	for i := 0; i < len(c.CmdList); i++ {
		cmd := &c.CmdList[i]
		cmd.TargetId = get.GetId(cmd.Target)
	}
}

func (m *Msg) MustGetString(key string) string {
	s, _ := m.GetString(key)
	return s
}

func (m *Msg) GetString(key string) (string, bool) {
	if m.Values == nil {
		return "", false
	}
	si, ok := m.Values[key]
	if !ok {
		return "", false
	}
	s, ok := si.(string)
	if !ok {
		return "", false
	}
	return s, true
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
