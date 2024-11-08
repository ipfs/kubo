package coreunix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"strconv"
	"time"

	bstore "github.com/ipfs/boxo/blockstore"
	chunker "github.com/ipfs/boxo/chunker"
	"github.com/ipfs/boxo/files"
	posinfo "github.com/ipfs/boxo/filestore/posinfo"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/ipld/unixfs/importer/balanced"
	ihelper "github.com/ipfs/boxo/ipld/unixfs/importer/helpers"
	"github.com/ipfs/boxo/ipld/unixfs/importer/trickle"
	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/boxo/path"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/kubo/core/coreiface"

	"github.com/ipfs/kubo/tracing"
)

var log = logging.Logger("coreunix")

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

var liveCacheSize = uint64(256 << 10)

type Link struct {
	Name, Hash string
	Size       uint64
}

type syncer interface {
	Sync() error
}

// NewAdder Returns a new Adder used for a file add operation.
func NewAdder(ctx context.Context, p pin.Pinner, bs bstore.GCLocker, ds ipld.DAGService) (*Adder, error) {
	bufferedDS := ipld.NewBufferedDAG(ctx, ds)

	return &Adder{
		ctx:        ctx,
		pinning:    p,
		gcLocker:   bs,
		dagService: ds,
		bufferedDS: bufferedDS,
		Progress:   false,
		Pin:        true,
		Trickle:    false,
		Chunker:    "",
	}, nil
}

// Adder holds the switches passed to the `add` command.
type Adder struct {
	ctx        context.Context
	pinning    pin.Pinner
	gcLocker   bstore.GCLocker
	dagService ipld.DAGService
	bufferedDS *ipld.BufferedDAG
	Out        chan<- interface{}
	Progress   bool
	Pin        bool
	Trickle    bool
	RawLeaves  bool
	Silent     bool
	NoCopy     bool
	Chunker    string
	mroot      *mfs.Root
	unlocker   bstore.Unlocker
	tempRoot   cid.Cid
	CidBuilder cid.Builder
	liveNodes  uint64

	PreserveMode  bool
	PreserveMtime bool
	FileMode      os.FileMode
	FileMtime     time.Time
}

func (adder *Adder) mfsRoot() (*mfs.Root, error) {
	if adder.mroot != nil {
		return adder.mroot, nil
	}
	rnode := unixfs.EmptyDirNode()
	err := rnode.SetCidBuilder(adder.CidBuilder)
	if err != nil {
		return nil, err
	}
	mr, err := mfs.NewRoot(adder.ctx, adder.dagService, rnode, nil)
	if err != nil {
		return nil, err
	}
	adder.mroot = mr
	return adder.mroot, nil
}

// SetMfsRoot sets `r` as the root for Adder.
func (adder *Adder) SetMfsRoot(r *mfs.Root) {
	adder.mroot = r
}

// Constructs a node from reader's data, and adds it. Doesn't pin.
func (adder *Adder) add(reader io.Reader) (ipld.Node, error) {
	chnk, err := chunker.FromString(reader, adder.Chunker)
	if err != nil {
		return nil, err
	}

	params := ihelper.DagBuilderParams{
		Dagserv:     adder.bufferedDS,
		RawLeaves:   adder.RawLeaves,
		Maxlinks:    ihelper.DefaultLinksPerBlock,
		NoCopy:      adder.NoCopy,
		CidBuilder:  adder.CidBuilder,
		FileMode:    adder.FileMode,
		FileModTime: adder.FileMtime,
	}

	db, err := params.New(chnk)
	if err != nil {
		return nil, err
	}
	var nd ipld.Node
	if adder.Trickle {
		nd, err = trickle.Layout(db)
	} else {
		nd, err = balanced.Layout(db)
	}
	if err != nil {
		return nil, err
	}

	return nd, adder.bufferedDS.Commit()
}

// RootNode returns the mfs root node
func (adder *Adder) curRootNode() (ipld.Node, error) {
	mr, err := adder.mfsRoot()
	if err != nil {
		return nil, err
	}
	root, err := mr.GetDirectory().GetNode()
	if err != nil {
		return nil, err
	}

	// if one root file, use that hash as root.
	if len(root.Links()) == 1 {
		nd, err := root.Links()[0].GetNode(adder.ctx, adder.dagService)
		if err != nil {
			return nil, err
		}

		root = nd
	}

	return root, err
}

// Recursively pins the root node of Adder and
// writes the pin state to the backing datastore.
func (adder *Adder) PinRoot(ctx context.Context, root ipld.Node) error {
	ctx, span := tracing.Span(ctx, "CoreUnix.Adder", "PinRoot")
	defer span.End()

	if !adder.Pin {
		return nil
	}

	rnk := root.Cid()

	err := adder.dagService.Add(ctx, root)
	if err != nil {
		return err
	}

	if adder.tempRoot.Defined() {
		err := adder.pinning.Unpin(ctx, adder.tempRoot, true)
		if err != nil {
			return err
		}
		adder.tempRoot = rnk
	}

	err = adder.pinning.PinWithMode(ctx, rnk, pin.Recursive, "")
	if err != nil {
		return err
	}

	return adder.pinning.Flush(ctx)
}

func (adder *Adder) outputDirs(path string, fsn mfs.FSNode) error {
	switch fsn := fsn.(type) {
	case *mfs.File:
		return nil
	case *mfs.Directory:
		names, err := fsn.ListNames(adder.ctx)
		if err != nil {
			return err
		}

		for _, name := range names {
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

func (adder *Adder) addNode(node ipld.Node, path string) error {
	// patch it into the root
	if path == "" {
		path = node.Cid().String()
	}

	if pi, ok := node.(*posinfo.FilestoreNode); ok {
		node = pi.Node
	}

	mr, err := adder.mfsRoot()
	if err != nil {
		return err
	}
	dir := gopath.Dir(path)
	if dir != "." {
		opts := mfs.MkdirOpts{
			Mkparents:  true,
			Flush:      false,
			CidBuilder: adder.CidBuilder,
		}
		if err := mfs.Mkdir(mr, dir, opts); err != nil {
			return err
		}
	}

	if err := mfs.PutNode(mr, path, node); err != nil {
		return err
	}

	if !adder.Silent {
		return outputDagnode(adder.Out, path, node)
	}
	return nil
}

// AddAllAndPin adds the given request's files and pin them.
func (adder *Adder) AddAllAndPin(ctx context.Context, file files.Node) (ipld.Node, error) {
	ctx, span := tracing.Span(ctx, "CoreUnix.Adder", "AddAllAndPin")
	defer span.End()

	if adder.Pin {
		adder.unlocker = adder.gcLocker.PinLock(ctx)
	}
	defer func() {
		if adder.unlocker != nil {
			adder.unlocker.Unlock(ctx)
		}
	}()

	if err := adder.addFileNode(ctx, "", file, true); err != nil {
		return nil, err
	}

	// get root
	mr, err := adder.mfsRoot()
	if err != nil {
		return nil, err
	}
	var root mfs.FSNode
	rootdir := mr.GetDirectory()
	root = rootdir

	err = root.Flush()
	if err != nil {
		return nil, err
	}

	// if adding a file without wrapping, swap the root to it (when adding a
	// directory, mfs root is the directory)
	_, dir := file.(files.Directory)
	var name string
	if !dir {
		children, err := rootdir.ListNames(adder.ctx)
		if err != nil {
			return nil, err
		}

		if len(children) == 0 {
			return nil, fmt.Errorf("expected at least one child dir, got none")
		}

		// Replace root with the first child
		name = children[0]
		root, err = rootdir.Child(name)
		if err != nil {
			return nil, err
		}
	}

	err = mr.Close()
	if err != nil {
		return nil, err
	}

	nd, err := root.GetNode()
	if err != nil {
		return nil, err
	}

	// output directory events
	err = adder.outputDirs(name, root)
	if err != nil {
		return nil, err
	}

	if asyncDagService, ok := adder.dagService.(syncer); ok {
		err = asyncDagService.Sync()
		if err != nil {
			return nil, err
		}
	}

	if !adder.Pin {
		return nd, nil
	}
	return nd, adder.PinRoot(ctx, nd)
}

func (adder *Adder) addFileNode(ctx context.Context, path string, file files.Node, toplevel bool) error {
	ctx, span := tracing.Span(ctx, "CoreUnix.Adder", "AddFileNode")
	defer span.End()

	defer file.Close()

	err := adder.maybePauseForGC(ctx)
	if err != nil {
		return err
	}

	if adder.PreserveMtime {
		adder.FileMtime = file.ModTime()
	}

	if adder.PreserveMode {
		adder.FileMode = file.Mode()
	}

	if adder.liveNodes >= liveCacheSize {
		// TODO: A smarter cache that uses some sort of lru cache with an eviction handler
		mr, err := adder.mfsRoot()
		if err != nil {
			return err
		}
		if err := mr.FlushMemFree(adder.ctx); err != nil {
			return err
		}

		adder.liveNodes = 0
	}
	adder.liveNodes++

	switch f := file.(type) {
	case files.Directory:
		return adder.addDir(ctx, path, f, toplevel)
	case *files.Symlink:
		return adder.addSymlink(path, f)
	case files.File:
		return adder.addFile(path, f)
	default:
		return errors.New("unknown file type")
	}
}

func (adder *Adder) addSymlink(path string, l *files.Symlink) error {
	sdata, err := unixfs.SymlinkData(l.Target)
	if err != nil {
		return err
	}

	if !adder.FileMtime.IsZero() {
		fsn, err := unixfs.FSNodeFromBytes(sdata)
		if err != nil {
			return err
		}

		fsn.SetModTime(adder.FileMtime)
		if sdata, err = fsn.GetBytes(); err != nil {
			return err
		}
	}

	dagnode := dag.NodeWithData(sdata)
	err = dagnode.SetCidBuilder(adder.CidBuilder)
	if err != nil {
		return err
	}
	err = adder.dagService.Add(adder.ctx, dagnode)
	if err != nil {
		return err
	}

	return adder.addNode(dagnode, path)
}

func (adder *Adder) addFile(path string, file files.File) error {
	// if the progress flag was specified, wrap the file so that we can send
	// progress updates to the client (over the output channel)
	var reader io.Reader = file
	if adder.Progress {
		rdr := &progressReader{file: reader, path: path, out: adder.Out}
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
	return adder.addNode(dagnode, path)
}

func (adder *Adder) addDir(ctx context.Context, path string, dir files.Directory, toplevel bool) error {
	log.Infof("adding directory: %s", path)

	// if we need to store mode or modification time then create a new root which includes that data
	if toplevel && (adder.FileMode != 0 || !adder.FileMtime.IsZero()) {
		nd := unixfs.EmptyDirNodeWithStat(adder.FileMode, adder.FileMtime)
		err := nd.SetCidBuilder(adder.CidBuilder)
		if err != nil {
			return err
		}
		mr, err := mfs.NewRoot(ctx, adder.dagService, nd, nil)
		if err != nil {
			return err
		}
		adder.SetMfsRoot(mr)
	}

	if !(toplevel && path == "") {
		mr, err := adder.mfsRoot()
		if err != nil {
			return err
		}
		err = mfs.Mkdir(mr, path, mfs.MkdirOpts{
			Mkparents:  true,
			Flush:      false,
			CidBuilder: adder.CidBuilder,
			Mode:       adder.FileMode,
			ModTime:    adder.FileMtime,
		})
		if err != nil {
			return err
		}
	}

	it := dir.Entries()
	for it.Next() {
		fpath := gopath.Join(path, it.Name())
		err := adder.addFileNode(ctx, fpath, it.Node(), false)
		if err != nil {
			return err
		}
	}

	return it.Err()
}

func (adder *Adder) maybePauseForGC(ctx context.Context) error {
	ctx, span := tracing.Span(ctx, "CoreUnix.Adder", "MaybePauseForGC")
	defer span.End()

	if adder.unlocker != nil && adder.gcLocker.GCRequested(ctx) {
		rn, err := adder.curRootNode()
		if err != nil {
			return err
		}

		err = adder.PinRoot(ctx, rn)
		if err != nil {
			return err
		}

		adder.unlocker.Unlock(ctx)
		adder.unlocker = adder.gcLocker.PinLock(ctx)
	}
	return nil
}

// outputDagnode sends dagnode info over the output channel
func outputDagnode(out chan<- interface{}, name string, dn ipld.Node) error {
	if out == nil {
		return nil
	}

	o, err := getOutput(dn)
	if err != nil {
		return err
	}

	out <- &coreiface.AddEvent{
		Path: o.Path,
		Name: name,
		Size: o.Size,
	}

	return nil
}

// from core/commands/object.go
func getOutput(dagnode ipld.Node) (*coreiface.AddEvent, error) {
	c := dagnode.Cid()
	s, err := dagnode.Size()
	if err != nil {
		return nil, err
	}

	output := &coreiface.AddEvent{
		Path: path.FromCid(c),
		Size: strconv.FormatUint(s, 10),
	}

	return output, nil
}

type progressReader struct {
	file         io.Reader
	path         string
	out          chan<- interface{}
	bytes        int64
	lastProgress int64
}

func (i *progressReader) Read(p []byte) (int, error) {
	n, err := i.file.Read(p)

	i.bytes += int64(n)
	if i.bytes-i.lastProgress >= progressReaderIncrement || err == io.EOF {
		i.lastProgress = i.bytes
		i.out <- &coreiface.AddEvent{
			Name:  i.path,
			Bytes: i.bytes,
		}
	}

	return n, err
}

type progressReader2 struct {
	*progressReader
	files.FileInfo
}

func (i *progressReader2) Read(p []byte) (int, error) {
	return i.progressReader.Read(p)
}
