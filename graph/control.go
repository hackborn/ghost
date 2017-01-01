package graph

// Handle communication between the graph and nodes.

import (
	"errors"
	"fmt"
	"github.com/hackborn/ghost/node"
	"sync"
)

// -----------------------------------------------
// control struct
// Manage channels responsibile for communicating between the grapnh and its nodes.
type control struct {
	// All control channelers
	all    node.Channels
	autoid node.Id
	// Registered control channels addressable via a Send().
	// This is accessed from multiple threads, so protect it.
	// XXX I've changed things -- channels shouldn't be closed
	// (or cleared) while nodes are running now, so this should
	// pprobably go away.
	regmu      sync.Mutex
	registered map[node.Id]chan node.Msg
}

func (ct *control) initialize() {
	// No node would have a higher ID.
	ct.autoid = 10000
}

func (ct *control) newChannel(id node.Id) (chan node.Msg, node.Id) {
	c := ct.all.NewChannel()
	if id <= 0 {
		id = ct.autoid
		ct.autoid++
	}
	if id > 0 && c != nil {
		ct.regmu.Lock()
		defer ct.regmu.Unlock()

		if ct.registered == nil {
			ct.registered = newRegistered()
		}
		ct.registered[id] = c
	} else if id <= 0 {
		fmt.Println("Creating control channel without node Id. This isn't very useful anymore.")
	}
	return c, id
}

func (ct *control) sendMsg(msg node.Msg, to node.Id) error {
	fmt.Println("send", msg, "to", to)
	ct.regmu.Lock()
	c, ok := ct.registered[to]
	ct.regmu.Unlock()
	if ok && c != nil {
		c <- msg
		return nil
	}
	return errors.New("No target")
}

func (ct *control) close() {
	ct.all.CloseChannels()
	ct.registered = newRegistered()
}

func newRegistered() map[node.Id]chan node.Msg {
	return make(map[node.Id]chan node.Msg)
}
