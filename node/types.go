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

const (
	cmdStop      = "stop"
	cmdStopReply = "stopreply"
)

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

type Prepare interface {
	// Answer a new channel used to provide communication between the
	// graph and nodes (and indirectly from node to node). If the Id is
	// > 0 then this will be registered as the control channel for the node.
	// Answer the Id this channel is registered at, which will either be
	// the Id supplied, or, if that wasn't valid, an auto-generated one.
	NewControlChannel(id Id) (chan Msg, Id)
}

type Start interface {
	// Answer the channel that determines whether the graph is running.
	// There is nothing to read from this channel; when it's closed, the
	// node should stop running. Note that this implies a new go func is
	// being created, which should always be paired with adding to the
	// Start.GetDoneWaiter.
	GetDoneChannel() chan int
	GetDoneWaiter() *sync.WaitGroup
	// Get access to a message-sending object.
	GetOwner() Owner
}

// A message passed between nodes.
type Msg struct {
	SenderId Id
	Values   map[string]interface{}
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
	// Send a command from a source.
	SendMsg(msg Msg, to Id) error
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

	// Running the node happens in two stages: First Prepare is
	// called, where the node should construct all data it will
	// need for the run, placing that in the returned interface.
	// Next Start is called, where the node is supplied the interface.
	PrepareToStart(p Prepare, inputs []Source) (interface{}, error)
	Start(s Start, data interface{}) error

	// Set target IDs (for commands)
	// XXX I think this will be deprecated when I rewrite XML load with 1.8.
	FillIds(get GetId)
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

func (m *Msg) SetInt(key string, value int) error {
	if m.Values == nil {
		m.Values = make(map[string]interface{})
	}
	m.Values[key] = value
	return nil
}

func (m *Msg) MustGetInt(key string) int {
	s, _ := m.GetInt(key)
	return s
}

func (m *Msg) GetInt(key string) (int, bool) {
	if m.Values == nil {
		return 0, false
	}
	ii, ok := m.Values[key]
	if !ok {
		return 0, false
	}
	i, ok := ii.(int)
	if !ok {
		return 0, false
	}
	return i, true
}

func (m *Msg) SetString(key string, value string) error {
	if m.Values == nil {
		m.Values = make(map[string]interface{})
	}
	m.Values[key] = value
	return nil
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
