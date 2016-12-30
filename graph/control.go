package graph

// Handle communication between the graph and nodes.

import (
	"errors"
	"fmt"
	"github.com/hackborn/ghost/node"
	"sync"
)

// -----------------------------------------------
// Control. Manage channels responsibile for
// communicating between the grapnh and its nodes.
type control struct {
	// All control channelers
	all    node.Channels
	// Registered control channels addressable via a Send().
	// This is accessed from multiple threads, so protect it.
	// XXX I've changed things -- channels shouldn't be closed
	// (or cleared) while nodes are running now, so this should
	// pprobably go away.
	regmu			sync.Mutex
	registered		map[node.Id]chan node.Msg
}

func (ct *control) newChannel(id node.Id) chan node.Msg {
	c := ct.all.NewChannel()
	if id > 0 && c != nil {
		ct.regmu.Lock()
		defer ct.regmu.Unlock()

		if ct.registered == nil {
			ct.registered = newRegistered()
		}
		ct.registered[id] = c
	}
	return c
}

func (ct *control) sendCmd(cmd node.Cmd, source node.Id) error {
	fmt.Println("send", cmd)
	ct.regmu.Lock()
	c, ok := ct.registered[cmd.TargetId]
	ct.regmu.Unlock()
	if ok && c != nil {
		c<-cmd.AsMsg()
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