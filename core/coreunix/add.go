package coreunix

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	gopath "path"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	importer "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	"github.com/ipfs/go-ipfs/pin"

	"github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
	logging "github.com/ipfs/go-ipfs/vendor/QmQg1J6vikuXF9oDvm4wpdeAUvvkVEKW1EYDw9HhTMnP2b/go-log"
)

var log = logging.Logger("coreunix")

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

type Link struct {
	Name, Hash string
	Size       uint64
}

type Object struct {
	Hash  string
	Links []Link
}

type hiddenFileError struct {
	fileName string
}

func (e *hiddenFileError) Error() string {
	return fmt.Sprintf("%s is a hidden file", e.fileName)
}

type ignoreFileError struct {
	fileName string
}

func (e *ignoreFileError) Error() string {
	return fmt.Sprintf("%s is an ignored file", e.fileName)
}

type AddedObject struct {
	Name  string
	Hash  string `json:",omitempty"`
	Bytes int64  `json:",omitempty"`
}

func NewAdder(ctx context.Context, n *core.IpfsNode, out chan interface{}) *Adder {
	e := dagutils.NewDagEditor(newDirNode(), nil)
	return &Adder{
		ctx:      ctx,
		node:     n,
		editor:   e,
		out:      out,
		Progress: false,
		Hidden:   true,
		Pin:      true,
		Trickle:  false,
		Wrap:     false,
		Chunker:  "",
	}
}

// Internal structure for holding the switches passed to the `add` call
type Adder struct {
	ctx      context.Context
	node     *core.IpfsNode
	editor   *dagutils.Editor
	out      chan interface{}
	Progress bool
	Hidden   bool
	Pin      bool
	Trickle  bool
	Wrap     bool
	Chunker  string
	root     *dag.Node
}

// Perform the actual add & pin locally, outputting results to reader
func (params Adder) add(reader io.Reader) (*dag.Node, error) {
	chnk, err := chunk.FromString(reader, params.Chunker)
	if err != nil {
		return nil, err
	}

	if params.Trickle {
		return importer.BuildTrickleDagFromReader(
			params.node.DAG,
			chnk,
		)
	}
	return importer.BuildDagFromReader(
		params.node.DAG,
		chnk,
	)
}

func (params *Adder) RootNode() (*dag.Node, error) {
	// for memoizing
	if params.root != nil {
		return params.root, nil
	}

	root := params.editor.GetNode()

	// if not wrapping, AND one root file, use that hash as root.
	if !params.Wrap && len(root.Links) == 1 {
		var err error
		root, err = root.Links[0].GetNode(params.ctx, params.editor.GetDagService())
		params.root = root
		// no need to output, as we've already done so.
		return root, err
	}

	// otherwise need to output, as we have not.
	err := outputDagnode(params.out, "", root)
	params.root = root
	return root, err
}

func (params *Adder) PinRoot() error {
	root, err := params.RootNode()
	if err != nil {
		return err
	}

	rnk, err := root.Key()
	if err != nil {
		return err
	}

	params.node.Pinning.PinWithMode(rnk, pin.Recursive)
	return params.node.Pinning.Flush()
}

func (params *Adder) Finalize(DAG dag.DAGService) (*dag.Node, error) {
	return params.editor.Finalize(DAG)
}

// Add builds a merkledag from the a reader, pinning all objects to the local
// datastore. Returns a key representing the root node.
func Add(n *core.IpfsNode, r io.Reader) (string, error) {
	unlock := n.Blockstore.PinLock()
	defer unlock()

	fileAdder := NewAdder(n.Context(), n, nil)

	node, err := fileAdder.add(r)
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
	unlock := n.Blockstore.PinLock()
	defer unlock()

	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(root, root, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fileAdder := NewAdder(n.Context(), n, nil)

	dagnode, err := fileAdder.AddFile(f)
	if err != nil {
		return "", err
	}

	k, err := dagnode.Key()
	if err != nil {
		return "", err
	}

	return k.String(), nil
}

// AddWrapped adds data from a reader, and wraps it with a directory object
// to preserve the filename.
// Returns the path of the added file ("<dir hash>/filename"), the DAG node of
// the directory, and and error if any.
func AddWrapped(n *core.IpfsNode, r io.Reader, filename string) (string, *dag.Node, error) {
	file := files.NewReaderFile(filename, filename, ioutil.NopCloser(r), nil)
	dir := files.NewSliceFile("", "", []files.File{file})
	fileAdder := NewAdder(n.Context(), n, nil)

	unlock := n.Blockstore.PinLock()
	defer unlock()
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

func (params *Adder) addNode(node *dag.Node, path string) error {
	// patch it into the root
	if path == "" {
		key, err := node.Key()
		if err != nil {
			return err
		}

		path = key.Pretty()
	}

	if err := params.editor.InsertNodeAtPath(params.ctx, path, node, newDirNode); err != nil {
		return err
	}

	return outputDagnode(params.out, path, node)
}

// Add the given file while respecting the params.
func (params *Adder) AddFile(file files.File) (*dag.Node, error) {
	switch {
	case files.IsHidden(file) && !params.Hidden:
		log.Debugf("%s is hidden, skipping", file.FileName())
		return nil, &hiddenFileError{file.FileName()}
	case file.IsDirectory():
		return params.addDir(file)
	}

	// case for symlink
	if s, ok := file.(*files.Symlink); ok {
		sdata, err := unixfs.SymlinkData(s.Target)
		if err != nil {
			return nil, err
		}

		dagnode := &dag.Node{Data: sdata}
		_, err = params.node.DAG.Add(dagnode)
		if err != nil {
			return nil, err
		}

		err = params.addNode(dagnode, s.FileName())
		return dagnode, err
	}

	// case for regular file
	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if params.Progress {
		reader = &progressReader{file: file, out: params.out}
	}

	dagnode, err := params.add(reader)
	if err != nil {
		return nil, err
	}

	// patch it into the root
	log.Infof("adding file: %s", file.FileName())
	err = params.addNode(dagnode, file.FileName())
	return dagnode, err
}

func (params *Adder) addDir(dir files.File) (*dag.Node, error) {
	tree := newDirNode()
	log.Infof("adding directory: %s", dir.FileName())

	for {
		file, err := dir.NextFile()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if file == nil {
			break
		}

		node, err := params.AddFile(file)
		if _, ok := err.(*hiddenFileError); ok {
			// hidden file error, skip file
			continue
		} else if err != nil {
			return nil, err
		}

		_, name := gopath.Split(file.FileName())

		if err := tree.AddNodeLinkClean(name, node); err != nil {
			return nil, err
		}
	}

	if err := params.addNode(tree, dir.FileName()); err != nil {
		return nil, err
	}

	if _, err := params.node.DAG.Add(tree); err != nil {
		return nil, err
	}

	return tree, nil
}

// outputDagnode sends dagnode info over the output channel
func outputDagnode(out chan interface{}, name string, dn *dag.Node) error {
	if out == nil {
		return nil
	}

	o, err := getOutput(dn)
	if err != nil {
		return err
	}

	out <- &AddedObject{
		Hash: o.Hash,
		Name: name,
	}

	return nil
}

func NewMemoryDagService() dag.DAGService {
	// build mem-datastore for editor's intermediary nodes
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	return dag.NewDAGService(bsrv)
}

// TODO: generalize this to more than unix-fs nodes.
func newDirNode() *dag.Node {
	return &dag.Node{Data: unixfs.FolderPBData()}
}

// from core/commands/object.go
func getOutput(dagnode *dag.Node) (*Object, error) {
	key, err := dagnode.Key()
	if err != nil {
		return nil, err
	}

	output := &Object{
		Hash:  key.Pretty(),
		Links: make([]Link, len(dagnode.Links)),
	}

	for i, link := range dagnode.Links {
		output.Links[i] = Link{
			Name: link.Name,
			Hash: link.Hash.B58String(),
			Size: link.Size,
		}
	}

	return output, nil
}

type progressReader struct {
	file         files.File
	out          chan interface{}
	bytes        int64
	lastProgress int64
}

func (i *progressReader) Read(p []byte) (int, error) {
	n, err := i.file.Read(p)

	i.bytes += int64(n)
	if i.bytes-i.lastProgress >= progressReaderIncrement || err == io.EOF {
		i.lastProgress = i.bytes
		i.out <- &AddedObject{
			Name:  i.file.FileName(),
			Bytes: i.bytes,
		}
	}

	return n, err
}
