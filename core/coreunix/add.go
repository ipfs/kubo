package coreunix

import (
	"errors"
	"io"
	"os"
	"path"

	"github.com/jbenet/go-ipfs/commands/files"
	core "github.com/jbenet/go-ipfs/core"
	importer "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	merkledag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
	"github.com/jbenet/go-ipfs/thirdparty/eventlog"
	unixfs "github.com/jbenet/go-ipfs/unixfs"
	u "github.com/jbenet/go-ipfs/util"
)

var log = eventlog.Logger("coreunix")

// Add builds a merkledag from the a reader, pinning all objects to the local
// datastore. Returns a key representing the root node.
func Add(n *core.IpfsNode, r io.Reader) (u.Key, error) {
	// TODO more attractive function signature importer.BuildDagFromReader
	dagNode, err := importer.BuildDagFromReader(
		r,
		n.DAG,
		n.Pinning.GetManual(), // Fix this interface
		chunk.DefaultSplitter,
	)
	if err != nil {
		return "", err
	}
	if err := n.Pinning.Flush(); err != nil {
		return "", err
	}
	return dagNode.Key()
}

// AddR recursively adds files in |path|.
func AddR(n *core.IpfsNode, root string) (key string, err error) {
	f, err := os.Open(root)
	if err != nil {
		return "", err
	}
	defer f.Close()
	ff, err := files.NewSerialFile(root, f)
	if err != nil {
		return "", err
	}
	dagnode, err := addFile(n, ff)
	if err != nil {
		return "", err
	}
	k, err := dagnode.Key()
	if err != nil {
		return "", err
	}
	return k.String(), nil
}

func add(n *core.IpfsNode, readers []io.Reader) ([]*merkledag.Node, error) {
	mp, ok := n.Pinning.(pin.ManualPinner)
	if !ok {
		return nil, errors.New("invalid pinner type! expected manual pinner")
	}
	dagnodes := make([]*merkledag.Node, 0)
	for _, reader := range readers {
		node, err := importer.BuildDagFromReader(reader, n.DAG, mp, chunk.DefaultSplitter)
		if err != nil {
			return nil, err
		}
		dagnodes = append(dagnodes, node)
	}
	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}
	return dagnodes, nil
}

func addNode(n *core.IpfsNode, node *merkledag.Node) error {
	err := n.DAG.AddRecursive(node) // add the file to the graph + local storage
	if err != nil {
		return err
	}
	err = n.Pinning.Pin(node, true) // ensure we keep it
	if err != nil {
		return err
	}
	return nil
}

func addFile(n *core.IpfsNode, file files.File) (*merkledag.Node, error) {
	if file.IsDirectory() {
		return addDir(n, file)
	}

	dns, err := add(n, []io.Reader{file})
	if err != nil {
		return nil, err
	}

	return dns[len(dns)-1], nil // last dag node is the file.
}

func addDir(n *core.IpfsNode, dir files.File) (*merkledag.Node, error) {

	tree := &merkledag.Node{Data: unixfs.FolderPBData()}

Loop:
	for {
		file, err := dir.NextFile()
		switch {
		case err != nil && err != io.EOF:
			return nil, err
		case err == io.EOF:
			break Loop
		}

		node, err := addFile(n, file)
		if err != nil {
			return nil, err
		}

		_, name := path.Split(file.FileName())

		err = tree.AddNodeLink(name, node)
		if err != nil {
			return nil, err
		}
		k, err := node.Key()
		if err != nil {
			return nil, err
		}
		log.Debugf("add %s %s", k, name)
	}

	err := addNode(n, tree)
	if err != nil {
		return nil, err
	}
	return tree, nil
}
