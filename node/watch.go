package node

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"sync"
)

type Folder struct {
	Path string `xml:",chardata"`
}

// -----------------------------------------------
// Watch struct
// Watch a folder path, emitting messages when it changes.
type Watch struct {
	Id       Id
	Name     string   `xml:"name,attr"`
	Folders  []Folder `xml:"folder"`
	Channels          // Output
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

func (w *Watch) PrepareToStart(p Prepare, inputs []Source) (interface{}, error) {
	// No inputs means this node is never hit, so ignore.
	if len(inputs) <= 0 {
		return nil, nil
	}

	// If we want multiple inputs, we'll need to expand the running
	// code to handle merging.
	if len(inputs) != 1 {
		fmt.Println("node.Watch.Start() must have 1 input (for now)", len(inputs))
		return nil, errors.New("node.Watch does not support multiple inputs")
	}

	data := prepareDataWatch{}
	for _, i := range inputs {
		data.input.Add(i.NewChannel())
	}

	return data, nil
}

func (w *Watch) Start(s Start, idata interface{}) error {
	data, ok := idata.(prepareDataWatch)
	if !ok {
		return errors.New("node.Watch no prepareData")
	}
	if len(data.input.Out) != 1 {
		return errors.New("node.Watch no inputs")
	}
	//	fmt.Println("Start watch", w, "ins", len(data.input.Out), "outs", len(w.Out))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("watch err", err)
		return errors.New("Watch won't start")
	}

	done := s.GetDoneChannel()
	waiter := s.GetDoneWaiter()
	waiter.Add(1)
	go func(done chan struct{}, waiter *sync.WaitGroup, data prepareDataWatch) {
		defer waiter.Done()
		defer debug("end watch %v", w.Id)
		defer w.CloseChannels()
		defer watcher.Close()

		debug("start watch %v name=%v", w.Id, w.Name)

		for {
			select {
			case <-done:
				return
			case _, imore := <-data.input.Out[0]:
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
	}(done, waiter, data)

	err = watcher.Add("C:/work/dev/go/src/github.com/hackborn/ghost")
	if err != nil {
		return errors.New("Watcher can't add path")
	}

	return nil
}

// -----------------------------------------------
// prepareDataWatch struct
// Store data generate in the Prepare.
type prepareDataWatch struct {
	input Channels
}
