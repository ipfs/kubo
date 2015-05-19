package coreunix

import (
	"io"
	"io/ioutil"
	"os"
	gopath "path"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	"github.com/ipfs/go-ipfs/thirdparty/eventlog"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

var log = eventlog.Logger("coreunix")

// Add builds a merkledag from the a reader, pinning all objects to the local
// datastore. Returns a key representing the root node.
func Add(n *core.IpfsNode, r io.Reader) (string, error) {
	// TODO more attractive function signature importer.BuildDagFromReader
	dagNode, err := importer.BuildDagFromReader(
		r,
		n.DAG,
		chunk.DefaultSplitter,
		importer.BasicPinnerCB(n.Pinning.GetManual()),
	)
	if err != nil {
		return "", err
	}
	k, err := dagNode.Key()
	if err != nil {
		return "", err
	}

	return k.String(), nil
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

	n.Pinning.GetManual().RemovePinWithMode(k, pin.Indirect)
	err = n.Pinning.Flush()
	if err != nil {
		return "", err
	}

	return k.String(), nil
}

// AddWrapped adds data from a reader, and wraps it with a directory object
// to preserve the filename.
// Returns the path of the added file ("<dir hash>/filename"), the DAG node of
// the directory, and and error if any.
func AddWrapped(n *core.IpfsNode, r io.Reader, filename string) (string, *merkledag.Node, error) {
	file := files.NewReaderFile(filename, ioutil.NopCloser(r), nil)
	dir := files.NewSliceFile("", []files.File{file})
	dagnode, err := addDir(n, dir)
	if err != nil {
		return "", nil, err
	}
	k, err := dagnode.Key()
	if err != nil {
		return "", nil, err
	}
	return gopath.Join(k.String(), filename), dagnode, nil
}

func add(n *core.IpfsNode, reader io.Reader) (*merkledag.Node, error) {
	mp := n.Pinning.GetManual()

	node, err := importer.BuildDagFromReader(
		reader,
		n.DAG,
		chunk.DefaultSplitter,
		func(k u.Key, root bool) error {
			mp.PinWithMode(k, pin.Indirect)
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func addNode(n *core.IpfsNode, node *merkledag.Node) error {
	err := n.DAG.AddRecursive(node) // add the file to the graph + local storage
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()
	err = n.Pinning.Pin(ctx, node, true) // ensure we keep it
	if err != nil {
		return err
	}
	return nil
}

func addFile(n *core.IpfsNode, file files.File) (*merkledag.Node, error) {
	if file.IsDirectory() {
		return addDir(n, file)
	}

	dagnode, err := add(n, file)
	if err != nil {
		return nil, err
	}

	return dagnode, nil
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

		_, name := gopath.Split(file.FileName())

		err = tree.AddNodeLink(name, node)
		if err != nil {
			return nil, err
		}
	}

	err := addNode(n, tree)
	if err != nil {
		return nil, err
	}
	return tree, nil
}
