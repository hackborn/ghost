package node

type Folder struct {
	Path string	`xml:",chardata"`
}

// Watch a folder path, emitting messages when it changes.
type Watcher struct {
	Name string		`xml:"name,attr"`
	Folders []Folder 	`xml:"folder"`
}

func (n *Watcher) Stop() {
}

func (n *Watcher) RequestAccess(data *RequestData) {
}
