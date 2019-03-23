package coreunix

import (
	"context"
	"errors"
	"fmt"
	"io"
	gopath "path"
	"strconv"

	"github.com/ipfs/go-ipfs/pin"

	"github.com/ipfs/go-cid"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	chunker "github.com/ipfs/go-ipfs-chunker"
	files "github.com/ipfs/go-ipfs-files"
	posinfo "github.com/ipfs/go-ipfs-posinfo"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	dag "github.com/ipfs/go-merkledag"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-unixfs"
	"github.com/ipfs/go-unixfs/importer/balanced"
	ihelper "github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipfs/go-unixfs/importer/trickle"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var log = logging.Logger("coreunix")

// how many bytes of progress to wait before sending a progress update message
const progressReaderIncrement = 1024 * 256

var liveCacheSize = uint64(256 << 10)

type Link struct {
	Name, Hash string
	Size       uint64
}

type noopDagService struct{}

func (noopDagService) Add(_ context.Context, _ ipld.Node) error       { return nil }
func (noopDagService) AddMany(_ context.Context, _ []ipld.Node) error { return nil }
func (noopDagService) Get(_ context.Context, _ cid.Cid) (ipld.Node, error) {
	return nil, ipld.ErrNotFound
}
func (noopDagService) GetMany(_ context.Context, _ []cid.Cid) <-chan *ipld.NodeOption {
	ch := make(chan *ipld.NodeOption)
	close(ch)
	return ch
}
func (noopDagService) Remove(_ context.Context, _ cid.Cid) error       { return nil }
func (noopDagService) RemoveMany(_ context.Context, _ []cid.Cid) error { return nil }

// NewAdder Returns a new Adder used for a file add operation.
//
// * If the pinner is non-nil, the adder will recursively pin the root.
// * If the bs is non-nil, the adder will lock the garbage collector when
//   pinning.
// * If the dagservice is nil, all intermediate nodes but the root will be
//   discarded (useful for hashing).
func NewAdder(p pin.Pinner, bs bstore.GCLocker, ds ipld.DAGService) (*Adder, error) {
	return &Adder{
		pinning:    p,
		gcLocker:   bs,
		dagService: ds,
		Progress:   false,
		Trickle:    false,
		Chunker:    "",
	}, nil
}

// Adder holds the switches passed to the `add` command.
type Adder struct {
	pinning    pin.Pinner
	gcLocker   bstore.GCLocker
	dagService ipld.DAGService

	Progress   bool
	Trickle    bool
	RawLeaves  bool
	Silent     bool
	NoCopy     bool
	Chunker    string
	CidBuilder cid.Builder
	Out        chan<- interface{}
}

// The base "adder" job.
type adderJob struct {
	*Adder
	ctx        context.Context
	bufferedDS *ipld.BufferedDAG
	unlocker   bstore.Unlocker
	tempRoot   cid.Cid
}

// The adder job for adding directories.
type dirAdderJob struct {
	*adderJob
	mroot     *mfs.Root
	liveNodes uint64
}

func newDirAdderJob(job *adderJob) *dirAdderJob {
	rnode := unixfs.EmptyDirNode()
	rnode.SetCidBuilder(job.CidBuilder)
	ds := job.dagService
	if ds == nil {
		ds = mdutils.Mock()
	}

	mr, err := mfs.NewRoot(job.ctx, ds, rnode, nil)
	if err != nil {
		// impossible.
		panic(err)
	}
	return &dirAdderJob{
		adderJob: job,
		mroot:    mr,
	}
}

// Constructs a node from reader's data, and adds it. Doesn't pin.
func (adder *adderJob) add(reader io.Reader) (ipld.Node, error) {
	chnk, err := chunker.FromString(reader, adder.Chunker)
	if err != nil {
		return nil, err
	}

	// Make sure all added nodes are written when done.
	defer adder.bufferedDS.Commit()

	params := ihelper.DagBuilderParams{
		Dagserv:    adder.bufferedDS,
		RawLeaves:  adder.RawLeaves,
		Maxlinks:   ihelper.DefaultLinksPerBlock,
		NoCopy:     adder.NoCopy,
		CidBuilder: adder.CidBuilder,
	}

	db, err := params.New(chnk)
	if err != nil {
		return nil, err
	}
	if adder.Trickle {
		return trickle.Layout(db)
	}

	return balanced.Layout(db)
}

// Recursively pins the root node of Adder and
// writes the pin state to the backing datastore.
func (adder *adderJob) pinRoot(root ipld.Node) error {
	if adder.pinning == nil || adder.dagService == nil {
		return nil
	}

	rnk := root.Cid()

	err := adder.dagService.Add(adder.ctx, root)
	if err != nil {
		return err
	}

	if adder.tempRoot.Defined() {
		err := adder.pinning.Unpin(adder.ctx, adder.tempRoot, true)
		if err != nil {
			return err
		}
		adder.tempRoot = rnk
	}

	adder.pinning.PinWithMode(rnk, pin.Recursive)
	return adder.pinning.Flush()
}

func (adder *dirAdderJob) outputDirs(path string, fsn mfs.FSNode) error {
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

func (adder *dirAdderJob) linkNode(node ipld.Node, path string) error {
	if pi, ok := node.(*posinfo.FilestoreNode); ok {
		node = pi.Node
	}

	dir := gopath.Dir(path)
	if dir != "." {
		opts := mfs.MkdirOpts{
			Mkparents:  true,
			Flush:      false,
			CidBuilder: adder.CidBuilder,
		}
		if err := mfs.Mkdir(adder.mroot, dir, opts); err != nil {
			return err
		}
	}

	if err := mfs.PutNode(adder.mroot, path, node); err != nil {
		return err
	}

	return nil
}

// AddAllAndPin adds the given request's files and pin them (if applicable).
func (adder *Adder) Add(ctx context.Context, file files.Node) (ipld.Node, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	baseJob := &adderJob{
		Adder: adder,
		ctx:   ctx,
	}
	ds := adder.dagService
	if ds == nil {
		ds = noopDagService{}
	}
	baseJob.bufferedDS = ipld.NewBufferedDAG(ctx, ds)

	if adder.pinning != nil && adder.gcLocker != nil {
		baseJob.unlocker = adder.gcLocker.PinLock()
	}
	defer func() {
		if baseJob.unlocker != nil {
			baseJob.unlocker.Unlock()
		}
	}()

	var (
		nd  ipld.Node
		err error
	)
	switch file := file.(type) {
	case *files.Symlink:
		nd, err = baseJob.addSymlink("", file)
	case files.File:
		nd, err = baseJob.addFile("", file)
	case files.Directory:
		job := newDirAdderJob(baseJob)

		// Add the directory
		err = job.addFileNode("", file)
		if err != nil {
			break
		}

		// get root
		root := job.mroot.GetDirectory()
		nd, err = root.GetNode()
		if err != nil {
			break
		}

		// output directory events
		err = job.outputDirs("", root)
	}
	if err != nil {
		return nil, err
	}
	return nd, baseJob.pinRoot(nd)
}

func (adder *dirAdderJob) addFileNode(path string, file files.Node) error {
	err := adder.maybePauseForGC()
	if err != nil {
		return err
	}

	if adder.liveNodes >= liveCacheSize {
		// TODO: A smarter cache that uses some sort of lru cache with an eviction handler
		if err := adder.mroot.FlushMemFree(adder.ctx); err != nil {
			return err
		}

		adder.liveNodes = 0
	}
	adder.liveNodes++

	switch f := file.(type) {
	case files.Directory:
		return adder.addDir(path, f)
	case *files.Symlink:
		return adder.addSymlink(path, f)
	case files.File:
		return adder.addFile(path, f)
	default:
		return errors.New("unknown file type")
	}
}

func (adder *adderJob) addSymlink(path string, l *files.Symlink) (ipld.Node, error) {
	sdata, err := unixfs.SymlinkData(l.Target)
	if err != nil {
		return nil, err
	}

	dagnode := dag.NodeWithData(sdata)
	dagnode.SetCidBuilder(adder.CidBuilder)
	if adder.dagService != nil {
		err = adder.dagService.Add(adder.ctx, dagnode)
	}
	if err == nil && !adder.Silent {
		err = outputDagnode(adder.Out, path, dagnode)
	}
	return dagnode, err
}

func (adder *dirAdderJob) addSymlink(path string, l *files.Symlink) error {
	dagnode, err := adder.adderJob.addSymlink(path, l)
	if err != nil {
		return err
	}

	return adder.linkNode(dagnode, path)
}

func (adder *adderJob) addFile(path string, file files.File) (ipld.Node, error) {
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

	nd, err := adder.add(reader)
	if err == nil && !adder.Silent {
		err = outputDagnode(adder.Out, path, nd)
	}

	return nd, err
}

func (adder *dirAdderJob) addFile(path string, file files.File) error {
	dagnode, err := adder.adderJob.addFile(path, file)
	if err != nil {
		return err
	}

	// patch it into the root
	return adder.linkNode(dagnode, path)
}

func (adder *dirAdderJob) addDir(path string, dir files.Directory) error {
	log.Infof("adding directory: %s", path)

	if path != "" {
		opts := mfs.MkdirOpts{
			Mkparents:  true,
			Flush:      false,
			CidBuilder: adder.CidBuilder,
		}
		err := mfs.Mkdir(adder.mroot, path, opts)
		if err != nil {
			return err
		}
	}

	it := dir.Entries()
	for it.Next() {
		fpath := gopath.Join(path, it.Name())
		node := it.Node()
		err1 := adder.addFileNode(fpath, node)
		err2 := node.Close()
		switch {
		case err1 != nil:
			return err1
		case err2 != nil:
			return err2
		}
	}

	return it.Err()
}

func (adder *dirAdderJob) maybePauseForGC() error {
	if adder.unlocker != nil && adder.gcLocker.GCRequested() {
		rn, err := adder.mroot.GetDirectory().GetNode()
		if err != nil {
			return err
		}

		err = adder.pinRoot(rn)
		if err != nil {
			return err
		}

		adder.unlocker.Unlock()
		adder.unlocker = adder.gcLocker.PinLock()
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
		Path: coreiface.IpfsPath(c),
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
