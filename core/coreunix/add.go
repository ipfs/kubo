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
	"github.com/ipfs/go-ipfs/merkledag/utils"
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

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

type Adder interface {
	AddFile(files.File) (*merkledag.Node, error)
	PinRoot() error
	WriteOutputToDAG() error
}

type AddedObject struct {
	Name  string
	Hash  string `json:",omitempty"`
	Bytes int64  `json:",omitempty"`
}

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

// Add builds a merkledag from the a reader, pinning all objects to the local
// datastore. Returns a key representing the root node.
func Add(n *core.IpfsNode, r io.Reader) (string, error) {
	fileAdder := adder{
		ctx:     n.Context(),
		node:    n,
		trickle: false,
		chunker: "default",
	}
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

	fileAdder := adder{
		ctx:     n.Context(),
		node:    n,
		trickle: false,
		chunker: "default",
	}
	dagnode, err := fileAdder.AddFile(f)
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
	fileAdder := adder{
		ctx:     n.Context(),
		node:    n,
		trickle: false,
		chunker: "default",
	}
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

func NewAdder(ctx context.Context, n *core.IpfsNode, out chan interface{}, progress bool, hidden bool, trickle bool, wrap bool, chunker string) Adder {
	e := dagutils.NewDagEditor(newMemoryDagService(), newDirNode())
	return &adder{
		ctx:      ctx,
		node:     n,
		editor:   e,
		out:      out,
		progress: progress,
		hidden:   hidden,
		trickle:  trickle,
		wrap:     wrap,
		chunker:  chunker,
	}
}

// Internal structure for holding the switches passed to the `add` call
type adder struct {
	ctx      context.Context
	node     *core.IpfsNode
	editor   *dagutils.Editor
	out      chan interface{}
	progress bool
	hidden   bool
	trickle  bool
	wrap     bool
	chunker  string
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

func (params *adder) PinRoot() error {
	root := params.editor.GetNode()

	// if not wrapping, AND one root file, use that hash as root.
	if !params.wrap && len(root.Links) == 1 {
		var err error
		root, err = root.Links[0].GetNode(params.ctx, params.editor.GetDagService())
		// no need to output, as we've already done so.
		if err != nil {
			return err
		}
	} else {
		// otherwise need to output, as we have not.
		if err := outputDagnode(params.out, "", root); err != nil {
			return err
		}
	}

	rnk, err := root.Key()
	if err != nil {
		return err
	}

	mp := params.node.Pinning.GetManual()
	mp.RemovePinWithMode(rnk, pin.Indirect)
	mp.PinWithMode(rnk, pin.Recursive)
	return params.node.Pinning.Flush()
}

func (params *adder) WriteOutputToDAG() error {
	return params.editor.WriteOutputTo(params.node.DAG)
}

func (params *adder) addNode(node *merkledag.Node, path string) error {
	// add the file to the graph + local storage
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
func (params *adder) AddFile(file files.File) (*merkledag.Node, error) {
	switch {
	case files.IsHidden(file) && !params.hidden:
		log.Debugf("%s is hidden, skipping", file.FileName())
		return nil, &hiddenFileError{file.FileName()}
	case file.IsDirectory():
		return params.addDir(file)
	}

	// Check if file is a symlink
	if s, ok := file.(*files.Symlink); ok {
		sdata, err := unixfs.SymlinkData(s.Target)
		if err != nil {
			return nil, err
		}

		dagnode := &merkledag.Node{Data: sdata}
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
	if params.progress {
		reader = &progressReader{file: file, out: params.out}
	}
	dagnode, err := params.add(reader, false)
	if err != nil {
		return nil, err
	}

	// patch it into the root
	log.Infof("adding file: %s", file.FileName())
	err = params.addNode(dagnode, file.FileName())
	return dagnode, err
}

func (params *adder) addDir(dir files.File) (*merkledag.Node, error) {
	tree := newDirNode()
	log.Infof("adding directory: %s", dir.FileName())

	for {
		file, err := dir.NextFile()
		switch {
		case err != nil && err != io.EOF:
			return nil, err
		case err == io.EOF:
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

		if err := tree.AddNodeLink(name, node); err != nil {
			return nil, err
		}
	}

	if err := params.addNode(tree, dir.FileName()); err != nil {
		return nil, err
	}

	// ensure we keep it
	// TODO: should this be k, err := tree.Key() ?
	k, err := params.node.DAG.Add(tree)
	if err != nil {
		return nil, err
	}
	params.node.Pinning.GetManual().PinWithMode(k, pin.Indirect)

	return tree, nil
}

// outputDagnode sends dagnode info over the output channel
func outputDagnode(out chan interface{}, name string, dn *merkledag.Node) error {
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

// TODO: generalize this to more than unix-fs nodes.
func newDirNode() *merkledag.Node {
	return &merkledag.Node{Data: unixfs.FolderPBData()}
}

func newMemoryDagService() merkledag.DAGService {
	// build mem-datastore for editor's intermediary nodes
	bs := bstore.NewBlockstore(syncds.MutexWrap(ds.NewMapDatastore()))
	bsrv := bserv.New(bs, offline.Exchange(bs))
	return merkledag.NewDAGService(bsrv)
}

// from core/commands/object.go
func getOutput(dagnode *merkledag.Node) (*Object, error) {
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
