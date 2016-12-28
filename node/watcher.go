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
type Watcher struct {
	Name     string   `xml:"name,attr"`
	Folders  []Folder `xml:"folder"`
	input    Channels
	Channels // Output
}

func (w *Watcher) ApplyArgs(cs ChangeString) {
	for i := 0; i < len(w.Folders); i++ {
		dst := &w.Folders[i]
		dst.Path = cs.ChangeString(dst.Path)
	}
}

func (w *Watcher) StartChannels(a StartArgs, inputs []Source) {
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

func (w *Watcher) StartRunning(a StartArgs) error {
	if len(w.input.Out) != 1 {
		return nil
	}
	fmt.Println("Start watcher", w, "ins", len(w.input.Out), "outs", len(w.Out))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("watch err", err)
		return errors.New("Watcher won't start")
	}

	control := a.Owner.NewControlChannel()
	if control == nil {
		return errors.New("Can't make control channel")
	}

	a.NodeWaiter.Add(1)
	go func() {
		fmt.Println("start watch func")
		defer a.NodeWaiter.Done()
		defer fmt.Println("end watcher func")
		defer w.CloseChannels()
		defer watcher.Close()

		for {
			select {
			case _, more := <-control:
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
	}()

	err = watcher.Add("C:/work/dev/go/src/github.com/hackborn/ghost")
	if err != nil {
		return errors.New("Watcher can't add path")
	}

	return nil
}

func (w *Watcher) RequestAccess(data *RequestArgs) {
}
