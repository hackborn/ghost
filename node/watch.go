package node

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Folder struct {
	Path   string `xml:",chardata"`
	Filter string `xml:"filter,attr"`
}

// Watch receives notification when a folder path changes.
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
	for _, v := range w.Folders {
		w, err := newWatchList(v.Path, v.Filter)
		if err != nil {
			return nil, err
		}
		data.watched = append(data.watched, w)
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
	go func(done <-chan struct{}, waiter *sync.WaitGroup, data prepareDataWatch) {
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

	has_watch_err := false
	for _, l := range data.watched {
		for k, v := range l.items {
			if v {
				err = watcher.Add(k)
				debug("watched added path", k)
				if err != nil {
					has_watch_err = true
				}
			}
		}
	}
	if has_watch_err {
		return errors.New("Watcher can't add path")
	}

	return nil
}

// prepareDataWatch stores data generated in the Prepare.
type prepareDataWatch struct {
	input   Channels
	watched []watch_list
}

// watch_list stores a list of folders, mapped to a value of
// whether or not the folder contains a watched file.
type watch_list struct {
	root  string
	items map[string]bool
}

func newWatchList(root string, _filter string) (watch_list, error) {
	filter := strings.ToLower(_filter)
	ans := watch_list{root, make(map[string]bool)}

	visit := func(file string, info os.FileInfo, err error) error {
		if info.IsDir() && len(filter) < 1 {
			ans.items[file] = true
		} else if !info.IsDir() && len(filter) > 0 {
			path_lc := strings.ToLower(file)
			if strings.Contains(path_lc, filter) {
				parent := filepath.Dir(file)
				ans.items[parent] = true
			}
		}
		return nil
	}

	err := filepath.Walk(root, visit)
	if err != nil {
		return watch_list{}, err
	}
	return ans, nil
}

func (wi *watch_list) add(path string, watched bool) {
	// A watched of true takes precedence over false.
	// XXX Unimplemented -- currently folders are only added
	// if we're watching them
}
