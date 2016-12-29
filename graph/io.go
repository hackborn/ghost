package graph

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kardianos/osext"

	"github.com/hackborn/ghost/node"
)

// A message passed between nodes.
type builder struct {
	graph *Graph
	order []node.Node
}

func (b *builder) build(la LoadArgs) {
	// For now there's no branching or multiple connections, so whatever
	// order was specified in the file is the order I'll use.
	if b.graph == nil {
		return
	}

	if la != nil {
		la(&b.graph.Args)
	}
	b.graph.Macros.Expand(b.graph.Args)
	b.graph.Expand(b.graph.Args)
	b.graph.Expand(b.graph.Macros)

	// Construct the inputs for each node. Right now it's very simple,
	// a single channel connection between each node based on the order
	// found in the graph file.
	var prev node.Node = nil
	for _, n := range b.order {
		if n != nil {
			if prev == nil {
				// If this is the first, then the graph is the input.
				b.graph.addInput(n, b.graph)
			} else {
				b.graph.addInput(n, prev)
			}
			prev = n
		}
	}
}

func (b *builder) GetId(name string) node.Id {
	for _, n := range b.order {
		if n.GetName() == name {
			return n.GetId()
		}
	}
	return 0
}

// Find the graph with the given name and load it.
func Load(n string, la LoadArgs) (*Graph, error) {
	n = strings.ToLower(n)
	p, err := osext.ExecutableFolder()
	if err != nil {
		return nil, err
	}
	// Search every location with graphs for the requested.
	p = path.Join(p, "data", "graphs")
	return loadFromPath(n, p, la)
}

// Construct a graph by loading from a filename.
func LoadFile(filename string, la LoadArgs) (*Graph, error) {
	xmlFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()
	decoder := xml.NewDecoder(xmlFile)

	var builder builder
	builder.graph = new(Graph)

	for {
		token, err := decoder.Token()
		if token == nil {
			if err == nil {
				continue
			}
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch ele := token.(type) {
		case xml.StartElement:
			if ele.Name.Local == "args" {
				decoder.DecodeElement(&builder.graph.Args, &ele)
			} else if ele.Name.Local == "macros" {
				decoder.DecodeElement(&builder.graph.Macros, &ele)
			} else if ele.Name.Local == "nodes" {
				decodeNodes(token, decoder, &builder)
			}
		}
	}

	// Fill in the IDs for all cmds. Ideally this would be handled
	// completely inside the load -- with go 1.8 I think I can move to
	// an abstract xml loading representation, and then just walk the
	// tree. For now, I go through some functions on my domain objects.
	for _, n := range builder.order {
		n.FillIds(&builder)
	}

	builder.build(la)
	//    fmt.Println("DONZO!", builder.graph)
	return builder.graph, nil
}

// Iterate the files in the path, loading any matching graph.
func loadFromPath(n string, p string, la LoadArgs) (*Graph, error) {
	p = path.Join(p, "*.xml")
	files, _ := filepath.Glob(p)
	for _, f := range files {
		b := formatName(filepath.Base(f))
		if n == b {
			return LoadFile(f, la)
		}
	}
	return nil, errors.New("No match")
}

// Given a fileaname base, format it so that I can compare against my input.
// For example, "GRAPH.XML" would becone "graph".
func formatName(n string) string {
	n = strings.TrimSuffix(n, filepath.Ext(n))
	return strings.ToLower(n)
}

func decodeNodes(token xml.Token, decoder *xml.Decoder, builder *builder) {
	var id node.Id = 1
	for {
		token, err := decoder.Token()
		if token == nil {
			if err == nil {
				continue
			}
			if err == io.EOF {
				break
			}
			return
		}

		var n node.Node
		switch ele := token.(type) {
		case xml.StartElement:
			if ele.Name.Local == "watcher" {
				var v node.Watcher
				v.Id = id
				v.Name = ele.Name.Local
				decoder.DecodeElement(&v, &ele)
				n = &v
			} else if ele.Name.Local == "exec" {
				var v node.Exec
				v.Id = id
				v.Name = ele.Name.Local
				decoder.DecodeElement(&v, &ele)
				//				fmt.Println("exec decode", v)
				if (&v).IsValid() {
					n = &v
				}
			}
		case xml.EndElement:
			if ele.Name.Local == "nodes" {
				return
			}
		}
		if n != nil {
			id++
			builder.graph.add(n)
			builder.order = append(builder.order, n)
		}
	}
}

// UGH. This exists solely because I constantly have to enable and disable
// the fmt import. SURELY there's some way for things to be slightly less
// stringent outside of a release build.
func empty() {
	fmt.Println("empty")
}
