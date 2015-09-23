package coreunix

import (
	"io"
	"io/ioutil"
	"os"
	gopath "path"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/ipfs/go-ipfs/unixfs"

	"github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	importer "github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	merkledag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	logging "github.com/ipfs/go-ipfs/vendor/go-log-v1.0.0"
)

var log = logging.Logger("coreunix")

// Add builds a merkledag from the a reader, pinning all objects to the local
// datastore. Returns a key representing the root node.
func Add(n *core.IpfsNode, r io.Reader) (string, error) {
	fileAdder := adder{n.Context(), n, false, "default"}
	node, err := fileAdder.add(r, true)
	if err != nil {
		return "", err
	}
	k, err := node.Key()
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

	fileAdder := adder{n.Context(), n, false, "default"}
	dagnode, err := fileAdder.addFile(f)
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
	fileAdder := adder{n.Context(), n, false, "default"}
	dagnode, err := fileAdder.addDir(dir)
	if err != nil {
		return "", nil, err
	}
	k, err := dagnode.Key()
	if err != nil {
		return "", nil, err
	}
	return gopath.Join(k.String(), filename), dagnode, nil
}

type adder struct {
	ctx     context.Context
	node    *core.IpfsNode
	trickle bool
	chunker string
}

// Perform the actual add & pin locally, outputting results to reader
func (params *adder) add(reader io.Reader, pinDirect bool) (*merkledag.Node, error) {
	mp := params.node.Pinning.GetManual()

	chnk, err := chunk.FromString(reader, params.chunker)
	if err != nil {
		return nil, err
	}

	cb := importer.PinIndirectCB(mp)
	if pinDirect {
		cb = importer.BasicPinnerCB(mp)
	}

	if params.trickle {
		return importer.BuildTrickleDagFromReader(
			params.node.DAG,
			chnk,
			cb,
		)
	}
	return importer.BuildDagFromReader(
		params.node.DAG,
		chnk,
		cb,
	)
}

func (params *adder) addNode(node *merkledag.Node) error {
	return params.node.DAG.AddRecursive(node) // add the file to the graph + local storage
}

func (params *adder) addFile(file files.File) (*merkledag.Node, error) {
	if file.IsDirectory() {
		return params.addDir(file)
	}
	return params.add(file, false)
}

func (params *adder) addDir(dir files.File) (*merkledag.Node, error) {
	tree := newDirNode()

	for {
		file, err := dir.NextFile()
		switch {
		case err != nil && err != io.EOF:
			return nil, err
		case err == io.EOF:
			break
		}

		node, err := params.addFile(file)
		if err != nil {
			return nil, err
		}

		_, name := gopath.Split(file.FileName())

		if err := tree.AddNodeLink(name, node); err != nil {
			return nil, err
		}
	}

	if err := params.addNode(tree); err != nil {
		return nil, err
	}

	// ensure we keep it
	k, err := tree.Key()
	if err != nil {
		return nil, err
	}
	params.node.Pinning.GetManual().PinWithMode(k, pin.Indirect)

	return tree, nil
}

// TODO: generalize this to more than unix-fs nodes.
func newDirNode() *merkledag.Node {
	return &merkledag.Node{Data: unixfs.FolderPBData()}
}
