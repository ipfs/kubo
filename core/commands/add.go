package commands

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/importer"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

func Add(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	depth := 1
	if r, ok := opts["r"].(bool); r && ok {
		depth = -1
	}
	for _, path := range args {
		nd, err := AddPath(n, path, depth)
		if err != nil {
			return fmt.Errorf("addFile error: %v", err)
		}

		k, err := nd.Key()
		if err != nil {
			return fmt.Errorf("addFile error: %v", err)
		}

		fmt.Fprintf(out, "Added node: %s = %s\n", path, k.Pretty())
	}
	return nil
}

func AddPath(n *core.IpfsNode, fpath string, depth int) (*dag.Node, error) {
	if depth == 0 {
		return nil, ErrDepthLimitExceeded
	}

	fi, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return addDir(n, fpath, depth)
	}

	return addFile(n, fpath, depth)
}

func addDir(n *core.IpfsNode, fpath string, depth int) (*dag.Node, error) {
	tree := &dag.Node{Data: dag.FolderPBData()}

	files, err := ioutil.ReadDir(fpath)
	if err != nil {
		return nil, err
	}

	// construct nodes for containing files.
	for _, f := range files {
		fp := filepath.Join(fpath, f.Name())
		nd, err := AddPath(n, fp, depth-1)
		if err != nil {
			return nil, err
		}

		if err = tree.AddNodeLink(f.Name(), nd); err != nil {
			return nil, err
		}
	}

	return tree, addNode(n, tree, fpath)
}

func addFile(n *core.IpfsNode, fpath string, depth int) (*dag.Node, error) {
	root, err := importer.NewDagFromFile(fpath)
	if err != nil {
		return nil, err
	}

	return root, addNode(n, root, fpath)
}

// addNode adds the node to the graph + local storage
func addNode(n *core.IpfsNode, nd *dag.Node, fpath string) error {
	// add the file to the graph + local storage
	err := n.DAG.AddRecursive(nd)
	if err != nil {
		return err
	}

	u.POut("added %s\n", fpath)

	// ensure we keep it. atm no-op
	return n.PinDagNode(nd)
}
