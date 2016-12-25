package node

import (
	"fmt"
)

type Folder struct {
	Path string	`xml:",chardata"`
}

// Watch a folder path, emitting messages when it changes.
type Watcher struct {
	Name string		`xml:"name,attr"`
	Folders []Folder 	`xml:"folder"`
	output Output
}

func (w *Watcher) Connect() (chan Msg) {
	return w.output.Add()
}

func (w *Watcher) Start(inputs []Source) {
	fmt.Println("Start watcher")
	if len(inputs) != 1 {
		fmt.Println("node.Watcher.Start() must have 1 input (for now)", len(inputs))
		return
	}
}

func (w *Watcher) RequestAccess(data *RequestArgs) {
}
