package main

import (
	"fmt"
	"github.com/gonuts/flag"
	"github.com/jbenet/commander"
	core "github.com/jbenet/go-ipfs/core"
	importer "github.com/jbenet/go-ipfs/importer"
	u "github.com/jbenet/go-ipfs/util"
	mh "github.com/jbenet/go-multihash"
	"os"
)

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
		err = addPath(n, fpath, depth)
		if err != nil {
			u.PErr("error adding %s: %v\n", fpath, err)
		}
	}
	return err
}

func addPath(n *core.IpfsNode, fpath string, depth int) error {
	fi, err := os.Stat(fpath)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		return addDir(n, fpath, depth)
	} else {
		return addFile(n, fpath, depth)
	}
}

func addDir(n *core.IpfsNode, fpath string, depth int) error {
	return u.NotImplementedError
}

func addFile(n *core.IpfsNode, fpath string, depth int) error {
	stat, err := os.Stat(fpath)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return fmt.Errorf("addFile: `fpath` is a directory")
	}

	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	root, err := importer.NewDagFromReader(f, stat.Size())
	if err != nil {
		return err
	}

	h, _ := root.Multihash()
	u.POut("hash: %v\n", h)
	u.POut("data: %v\n", root.Data)

	// add the file to the graph + local storage
	k, err := n.AddDagNode(root)
	if err != nil {
		return err
	}

	u.POut("added %s %s\n", fpath, mh.Multihash(k).B58String())
	return nil

	// ensure we keep it. atm no-op
	// return n.PinDagNode(root)
}
