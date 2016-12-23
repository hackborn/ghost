package node

// Run a command.
type Exec struct {
	Name string		`xml:"name,attr"`
	Cmd string		`xml:"cmd,attr"`
	Args string		`xml:"args,attr"`
	Dir string		`xml:"dir,attr"`
}

func (n *Exec) IsValid() bool {
	return len(n.Cmd) > 0
}

func (n Exec) Stop() {
}

func (n Exec) RequestAccess(data *RequestData) {
}
