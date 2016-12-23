package graph

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"github.com/kardianos/osext"
)

// Find the graph with the given name and load it.
func Load(n string) (*Graph, error) {
	fmt.Println("load graph", n)
	n = strings.ToLower(n)
	p, err := osext.ExecutableFolder()
	if err != nil {
		return nil, err
	}
	// Search every location with graphs for the requested.
	p = path.Join(p, "data", "graphs")
	return loadFromPath(n, p)
}

// Construct a graph by loading from a filename.
func LoadFile(n string) (*Graph, error) {
	fmt.Println("load file", n)
	return nil, nil
}

// Iterate the files in the path, loading any matching graph.
func loadFromPath(n string, p string) (*Graph, error) {
	p = path.Join(p, "*.xml")
	files, _ := filepath.Glob(p)
	for _, f := range files {
		b := formatName(filepath.Base(f))
		if n == b {
			return LoadFile(f)
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
