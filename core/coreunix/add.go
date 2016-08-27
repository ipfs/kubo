package coreunix

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	gopath "path"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	"github.com/ipfs/go-ipfs/exchange/offline"
	importer "github.com/ipfs/go-ipfs/importer"
	"github.com/ipfs/go-ipfs/importer/chunk"
	mfs "github.com/ipfs/go-ipfs/mfs"
	"github.com/ipfs/go-ipfs/pin"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	syncds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/sync"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	"github.com/ipfs/go-ipfs/commands/files"
	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	unixfs "github.com/ipfs/go-ipfs/unixfs"
	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
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

func NewAdder(ctx context.Context, p pin.Pinner, bs bstore.GCBlockstore, ds dag.DAGService, useRoot bool) (*Adder, error) {
	adder := &Adder{
		ctx:        ctx,
		pinning:    p,
		blockstore: bs,
		dagService: ds,
		Progress:   false,
		Hidden:     true,
		Pin:        true,
		Trickle:    false,
		Wrap:       false,
		Chunker:    "",
	}

	if useRoot {
		mr, err := mfs.NewRoot(ctx, ds, unixfs.EmptyDirNode(), nil)
		if err != nil {
			return nil, err
		}
		adder.mr = mr
	}

	return adder, nil
}

// Internal structure for holding the switches passed to the `add` call
type Adder struct {
	ctx        context.Context
	pinning    pin.Pinner
	blockstore bstore.GCBlockstore
	dagService dag.DAGService
	Out        chan interface{}
	Progress   bool
	Hidden     bool
	Pin        bool
	Trickle    bool
	Silent     bool
	Wrap       bool
	Chunker    string
	FullName   bool
	root       *dag.Node
	mr         *mfs.Root
	unlocker   bs.Unlocker
	tempRoot   key.Key
}

func (adder *Adder) SetMfsRoot(r *mfs.Root) {
	adder.mr = r
}

// Perform the actual add & pin locally, outputting results to reader
func (adder Adder) add(reader io.Reader) (*dag.Node, error) {
	chnk, err := chunk.FromString(reader, adder.Chunker)
	if err != nil {
		return nil, err
	}

	if adder.Trickle {
		return importer.BuildTrickleDagFromReader(
			adder.dagService,
			chnk)
	}
	return importer.BuildDagFromReader(
		adder.dagService,
		chnk)
}

func (adder *Adder) RootNode() (*dag.Node, error) {
	if adder.mr == nil {
		return nil, nil
	}

	// for memoizing
	if adder.root != nil {
		return adder.root, nil
	}

	root, err := adder.mr.GetValue().GetNode()
	if err != nil {
		return nil, err
	}

	// if not wrapping, AND one root file, use that hash as root.
	if !adder.Wrap && len(root.Links) == 1 {
		root, err = root.Links[0].GetNode(adder.ctx, adder.dagService)
		if err != nil {
			return nil, err
		}
	}

	adder.root = root
	return root, err
}

func (adder *Adder) PinRoot() error {
	if adder.mr == nil {
		return nil
	}

	root, err := adder.RootNode()
	if err != nil {
		return err
	}
	if !adder.Pin {
		return nil
	}

	rnk, err := adder.dagService.Add(root)
	if err != nil {
		return err
	}

	if adder.tempRoot != "" {
		err := adder.pinning.Unpin(adder.ctx, adder.tempRoot, true)
		if err != nil {
			return err
		}
		adder.tempRoot = rnk
	}

	adder.pinning.PinWithMode(rnk, pin.Recursive)
	return adder.pinning.Flush()
}

func (adder *Adder) Finalize() (*dag.Node, error) {
	if adder.mr == nil && adder.Pin {
		err := adder.pinning.Flush()
		return nil, err
	} else if adder.mr == nil {
		return nil, nil
	}

	root := adder.mr.GetValue()

	// cant just call adder.RootNode() here as we need the name for printing
	rootNode, err := root.GetNode()
	if err != nil {
		return nil, err
	}

	var name string
	if !adder.Wrap {
		name = rootNode.Links[0].Name

		dir, ok := adder.mr.GetValue().(*mfs.Directory)
		if !ok {
			return nil, fmt.Errorf("root is not a directory")
		}

		root, err = dir.Child(name)
		if err != nil {
			return nil, err
		}
	}

	err = adder.outputDirs(name, root)
	if err != nil {
		return nil, err
	}

	err = adder.mr.Close()
	if err != nil {
		return nil, err
	}

	return root.GetNode()
}

func (adder *Adder) outputDirs(path string, fsn mfs.FSNode) error {
	switch fsn := fsn.(type) {
	case *mfs.File:
		return nil
	case *mfs.Directory:
		for _, name := range fsn.ListNames() {
			child, err := fsn.Child(name)
			if err != nil {
				return err
			}

			childpath := gopath.Join(path, name)
			err = adder.outputDirs(childpath, child)
			if err != nil {
				return err
			}

			fsn.Uncache(name)
		}
		nd, err := fsn.GetNode()
		if err != nil {
			return err
		}

		return outputDagnode(adder.Out, path, nd)
	default:
		return fmt.Errorf("unrecognized fsn type: %#v", fsn)
	}
}

// Add builds a merkledag from the a reader, pinning all objects to the local
// datastore. Returns a key representing the root node.
func Add(n *core.IpfsNode, r io.Reader) (string, error) {
	defer n.Blockstore.PinLock().Unlock()

	fileAdder, err := NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG, true)
	if err != nil {
		return "", err
	}

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
	n.Blockstore.PinLock().Unlock()

	stat, err := os.Lstat(root)
	if err != nil {
		return "", err
	}

	f, err := files.NewSerialFile(root, root, false, stat)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fileAdder, err := NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG, true)
	if err != nil {
		return "", err
	}

	err = fileAdder.addFile(f)
	if err != nil {
		return "", err
	}

	nd, err := fileAdder.Finalize()
	if err != nil {
		return "", err
	}

	k, err := nd.Key()
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
	fileAdder, err := NewAdder(n.Context(), n.Pinning, n.Blockstore, n.DAG, true)
	if err != nil {
		return "", nil, err
	}
	fileAdder.Wrap = true

	defer n.Blockstore.PinLock().Unlock()

	err = fileAdder.addFile(file)
	if err != nil {
		return "", nil, err
	}

	dagnode, err := fileAdder.Finalize()
	if err != nil {
		return "", nil, err
	}

	k, err := dagnode.Key()
	if err != nil {
		return "", nil, err
	}

	return gopath.Join(k.String(), filename), dagnode, nil
}

func (adder *Adder) pinOrAddNode(node *dag.Node, file files.File) error {
	path := file.FileName()
	if adder.Pin && adder.mr == nil {

		key, err := node.Key()
		if err != nil {
			return err
		}

		adder.pinning.PinWithMode(key, pin.Recursive)

	} else if adder.mr != nil {

		// patch it into the root
		if path == "" {
			key, err := node.Key()
			if err != nil {
				return err
			}

			path = key.B58String()
		}

		dir := gopath.Dir(path)
		if dir != "." {
			if err := mfs.Mkdir(adder.mr, dir, true, false); err != nil {
				return err
			}
		}

		if err := mfs.PutNode(adder.mr, path, node); err != nil {
			return err
		}

	}
	if !adder.Silent {
		if adder.FullName {
			return outputDagnode(adder.Out, file.FullPath(), node)
		} else {
			return outputDagnode(adder.Out, file.FileName(), node)
		}
	}
	return nil
}

// Add the given file while respecting the adder.
func (adder *Adder) AddFile(file files.File) error {
	adder.unlocker = adder.blockstore.PinLock()
	defer func() {
		adder.unlocker.Unlock()
	}()

	return adder.addFile(file)
}

func (adder *Adder) addFile(file files.File) error {
	err := adder.maybePauseForGC()
	if err != nil {
		return err
	}

	if file.IsDirectory() {
		return adder.addDir(file)
	}

	// case for symlink
	if s, ok := file.(*files.Symlink); ok {
		sdata, err := unixfs.SymlinkData(s.Target)
		if err != nil {
			return err
		}

		dagnode := dag.NodeWithData(sdata)
		_, err = adder.dagService.Add(dagnode)
		if err != nil {
			return err
		}

		return adder.pinOrAddNode(dagnode, s)
	}

	// case for regular file
	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if adder.Progress {
		rdr := &progressReader{file: file, out: adder.Out}
		if fi, ok := file.(files.FileInfo); ok {
			reader = &progressReader2{rdr, fi}
		} else {
			reader = rdr
		}
	}

	dagnode, err := adder.add(reader)
	if err != nil {
		return err
	}

	// patch it into the root
	return adder.pinOrAddNode(dagnode, file)
}

func (adder *Adder) addDir(dir files.File) error {
	if adder.mr == nil {
		return errors.New("Cananot add directories without mfs root")
	}

	log.Infof("adding directory: %s", dir.FileName())

	err := mfs.Mkdir(adder.mr, dir.FileName(), true, false)
	if err != nil {
		return err
	}

	for {
		file, err := dir.NextFile()
		if err != nil && err != io.EOF {
			return err
		}
		if file == nil {
			break
		}

		// Skip hidden files when adding recursively, unless Hidden is enabled.
		if files.IsHidden(file) && !adder.Hidden {
			log.Infof("%s is hidden, skipping", file.FileName())
			continue
		}
		err = adder.addFile(file)
		if err != nil {
			return err
		}
	}

	return nil
}

func (adder *Adder) maybePauseForGC() error {
	if adder.blockstore.GCRequested() {
		err := adder.PinRoot()
		if err != nil {
			return err
		}

		adder.unlocker.Unlock()
		adder.unlocker = adder.blockstore.PinLock()
	}
	return nil
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

// from core/commands/object.go
func getOutput(dagnode *dag.Node) (*Object, error) {
	key, err := dagnode.Key()
	if err != nil {
		return nil, err
	}

	output := &Object{
		Hash:  key.B58String(),
		Links: make([]Link, len(dagnode.Links)),
	}

	for i, link := range dagnode.Links {
		output.Links[i] = Link{
			Name: link.Name,
			//Hash: link.Hash.B58String(),
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

type progressReader2 struct {
	*progressReader
	files.FileInfo
}
