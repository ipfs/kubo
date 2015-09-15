package coreunix

import (
	"io"
	"io/ioutil"
	"os"
	gopath "path"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	logging "github.com/ipfs/go-ipfs/vendor/go-log-v1.0.0"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
)

var log = logging.Logger("coreunix")

// Add builds a merkledag from the a reader, pinning all objects to the local
// datastore. Returns a key representing the root node.
func Add(n *core.IpfsNode, r io.Reader) (string, error) {
	// TODO more attractive function signature importer.BuildDagFromReader

	dagNode, err := importer.BuildDagFromReader(
		n.DAG,
		chunk.NewSizeSplitter(r, chunk.DefaultBlockSize),
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
	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(root, root, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()

	dagnode, err := addFile(n, f)
	if err != nil {
		return "", err
	}

	k, err := dagnode.Key()
	if err != nil {
		return "", err
	}

	n.Pinning.GetManual().RemovePinWithMode(k, pin.Indirect)
	if err := n.Pinning.Flush(); err != nil {
		return "", err
	}

	return k.String(), nil
}

// AddWrapped adds data from a reader, and wraps it with a directory object
// to preserve the filename.
// Returns the path of the added file ("<dir hash>/filename"), the DAG node of
// the directory, and and error if any.
func AddWrapped(n *core.IpfsNode, r io.Reader, filename string) (string, *merkledag.Node, error) {
	file := files.NewReaderFile(filename, filename, ioutil.NopCloser(r), nil)
	dir := files.NewSliceFile("", "", []files.File{file})
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

	return importer.BuildDagFromReader(
		n.DAG,
		chunk.DefaultSplitter(reader),
		importer.PinIndirectCB(mp),
	)
}

func addNode(n *core.IpfsNode, node *merkledag.Node) error {
	if err := n.DAG.AddRecursive(node); err != nil { // add the file to the graph + local storage
		return err
	}
	ctx, cancel := context.WithCancel(n.Context())
	defer cancel()
	err := n.Pinning.Pin(ctx, node, true) // ensure we keep it
	return err
}

func addFile(n *core.IpfsNode, file files.File) (*merkledag.Node, error) {
	if file.IsDirectory() {
		return addDir(n, file)
	}
	return add(n, file)
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

		if err := tree.AddNodeLink(name, node); err != nil {
			return nil, err
		}
	}

	if err := addNode(n, tree); err != nil {
		return nil, err
	}
	return tree, nil
}
