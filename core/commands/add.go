package commands

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/importer"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

// Add is a command that imports files and directories -- given as arguments -- into ipfs.
func Add(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	depth := 1

	// if recursive, set depth to reflect so
	if r, ok := opts["r"].(bool); r && ok {
		depth = -1
	}

	// add every path in args
	for _, path := range args {

		// Add the file
		_, err := AddPath(n, path, depth, out)
		if err != nil {
			if err == ErrDepthLimitExceeded && depth == 1 {
				err = errors.New("use -r to recursively add directories")
			}
			return fmt.Errorf("addFile error: %v", err)
		}

	}
	return nil
}

// AddPath adds a particular path to ipfs.
func AddPath(n *core.IpfsNode, fpath string, depth int, out io.Writer) (*dag.Node, error) {
	if depth == 0 {
		return nil, ErrDepthLimitExceeded
	}

	fi, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return addDir(n, fpath, depth, out)
	}

	return addFile(n, fpath, depth, out)
}

func addDir(n *core.IpfsNode, fpath string, depth int, out io.Writer) (*dag.Node, error) {
	tree := &dag.Node{Data: ft.FolderPBData()}

	files, err := ioutil.ReadDir(fpath)
	if err != nil {
		return nil, err
	}

	// construct nodes for containing files.
	for _, f := range files {
		fp := filepath.Join(fpath, f.Name())
		nd, err := AddPath(n, fp, depth-1, out)
		if err != nil {
			return nil, err
		}

		if err = tree.AddNodeLink(f.Name(), nd); err != nil {
			return nil, err
		}
	}

	log.Info("adding dir: %s", fpath)

	return tree, addNode(n, tree, fpath, out)
}

func addFile(n *core.IpfsNode, fpath string, depth int, out io.Writer) (*dag.Node, error) {
	root, err := importer.NewDagFromFile(fpath)
	if err != nil {
		return nil, err
	}

	log.Info("adding file: %s", fpath)

	for _, l := range root.Links {
		log.Info("adding subblock: %s %s", l.Name, l.Hash.B58String())
	}

	return root, addNode(n, root, fpath, out)
}

// addNode adds the node to the graph + local storage
func addNode(n *core.IpfsNode, nd *dag.Node, fpath string, out io.Writer) error {
	// add the file to the graph + local storage
	err := n.DAG.AddRecursive(nd)
	if err != nil {
		return err
	}

	k, err := nd.Key()
	if err != nil {
		return err
	}

	// output that we've added this node
	fmt.Fprintf(out, "added %s %s\n", k, fpath)

	// ensure we keep it. atm no-op
	return n.PinDagNodeRecursively(nd, -1)
}
