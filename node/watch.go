package node

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
)

type Folder struct {
	Path string `xml:",chardata"`
}

// Watch a folder path, emitting messages when it changes.
type Watch struct {
	Id       Id
	Name     string   `xml:"name,attr"`
	Folders  []Folder `xml:"folder"`
	input    Channels
	Channels // Output
	Cmds
}

func (n *Watch) GetId() Id {
	return n.Id
}

func (n *Watch) GetName() string {
	return n.Name
}

func (w *Watch) ApplyArgs(cs ChangeString) {
	for i := 0; i < len(w.Folders); i++ {
		dst := &w.Folders[i]
		dst.Path = cs.ChangeString(dst.Path)
	}
}

func (w *Watch) StartChannels(a StartArgs, inputs []Source) {
	// No inputs means this node is never hit, so ignore.
	if len(inputs) <= 0 {
		return
	}
	// If we want multiple inputs, we'll need to expand the running
	// code to handle merging.
	if len(inputs) != 1 {
		fmt.Println("node.Watcher.Start() must have 1 input (for now)", len(inputs))
		return
	}

	w.input.CloseChannels()
	for _, i := range inputs {
		w.input.Add(i.NewChannel())
	}
}

func (w *Watch) StartRunning(a StartArgs) error {
	if len(w.input.Out) != 1 {
		return nil
	}
	fmt.Println("Start watcher", w, "ins", len(w.input.Out), "outs", len(w.Out))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("watch err", err)
		return errors.New("Watcher won't start")
	}

	done := a.Owner.DoneChannel()
	if done == nil {
		return errors.New("Can't make done channel")
	}

	a.NodeWaiter.Add(1)
	go func(done chan int) {
		fmt.Println("start watch func")
		defer a.NodeWaiter.Done()
		defer fmt.Println("end watch func")
		defer w.CloseChannels()
		defer watcher.Close()

		for {
			select {
			case _, more := <-done:
				if !more {
					return
				}
			case _, imore := <-w.input.Out[0]:
				if !imore {
					return
				}
			case event := <-watcher.Events:
				//				fmt.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					//					fmt.Println("modified file:", event.Name)
					w.SendMsg(Msg{})
				}
			case err := <-watcher.Errors:
				fmt.Println("error:", err)
			}
		}
	}(done)

	err = watcher.Add("C:/work/dev/go/src/github.com/hackborn/ghost")
	if err != nil {
		return errors.New("Watcher can't add path")
	}

	return nil
}
