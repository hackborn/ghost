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
func LoadFile(filename string) (*Graph, error) {
	fmt.Println("load file", filename)
	xmlFile, err := os.Open(filename)
    if err != nil {
    	return nil, err
	}
	defer xmlFile.Close()
	decoder := xml.NewDecoder(xmlFile)

	fmt.Println("decode...")
	g := new(Graph)
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
        		fmt.Println("\tstart", ele.Name.Local)
        		if ele.Name.Local == "arg" {
        			decodeArgs(token, decoder, g)
        		}
	        case xml.EndElement:
        		fmt.Println("\tend?", ele.Name.Local)
	   	}
    }
    fmt.Println("DONZO!", g)
	return g, nil
}

func decodeArgs(token xml.Token, decoder *xml.Decoder, g *Graph) {
	name := ""
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

    	switch ele := token.(type) {
        	case xml.StartElement:
        		name = ele.Name.Local
	        case xml.EndElement:
	        	if ele.Name.Local == "arg" {
	        		return
	        	}
	        case xml.CharData:
	        	if name != "" {
	        		a := Arg{name, string(ele)}
	        		g.Args = append(g.Args, a)
	        		name = ""
	        	}
	   	}
    }
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
