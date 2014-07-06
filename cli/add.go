package main

import (
	"fmt"
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	core "github.com/jbenet/go-ipfs/core"
	importer "github.com/jbenet/go-ipfs/importer"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
	"io/ioutil"
	"os"
	"path/filepath"
)

var DepthLimitExceeded = fmt.Errorf("depth limit exceeded")

var cmdIpfsAdd = &commander.Command{
	UsageLine: "add",
	Short:     "Add an object to ipfs.",
	Long: `ipfs add <path>... - Add objects to ipfs.

    Adds contents of <path> to ipfs. Use -r to add directories.
    Note that directories are added recursively, to form the ipfs
    MerkleDAG. A smarter partial add with a staging area (like git)
    remains to be implemented.
`,
	Run:  addCmd,
	Flag: *flag.NewFlagSet("ipfs-add", flag.ExitOnError),
}

func init() {
	cmdIpfsAdd.Flag.Bool("r", false, "add objects recursively")
}

func addCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}

	n, err := localNode()
	if err != nil {
		return err
	}

	recursive := c.Flag.Lookup("r").Value.Get().(bool)
	var depth int
	if recursive {
		depth = -1
	} else {
		depth = 1
	}

	for _, fpath := range inp {
		_, err := addPath(n, fpath, depth)
		if err != nil {
			if !recursive {
				return fmt.Errorf("%s is a directory. Use -r to add recursively.", fpath)
			} else {
				u.PErr("error adding %s: %v\n", fpath, err)
			}
		}
	}
	return err
}

func addPath(n *core.IpfsNode, fpath string, depth int) (*dag.Node, error) {
	if depth == 0 {
		return nil, DepthLimitExceeded
	}

	fi, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return addDir(n, fpath, depth)
	} else {
		return addFile(n, fpath, depth)
	}
}

func addDir(n *core.IpfsNode, fpath string, depth int) (*dag.Node, error) {
	tree := &dag.Node{}

	files, err := ioutil.ReadDir(fpath)
	if err != nil {
		return nil, err
	}

	// construct nodes for containing files.
	for _, f := range files {
		fp := filepath.Join(fpath, f.Name())
		nd, err := addPath(n, fp, depth-1)
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
	k, err := n.AddDagNode(nd)
	if err != nil {
		return err
	}

	u.POut("added %s %s\n", fpath, mh.Multihash(k).B58String())
	return nil

	// ensure we keep it. atm no-op
	// return n.PinDagNode(root)
}
