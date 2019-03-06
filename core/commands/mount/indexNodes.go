package fusemount

import (
	"context"
	"fmt"
	dag "gx/ipfs/QmPJNbVw8o3ohC43ppSXyNXwYKsWShG4zygnirHptfbHri/go-merkledag"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	ipld "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
	"os"
	gopath "path"
	"runtime"
	"strings"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"
)

type recordBase struct {
	sync.RWMutex
	path string

	metadata fuse.Stat_t
	//handles  *[]uint64
	ioHandles nodeHandles
	//openFunc  nodeOpenFunc
}

type nodeOpenFunc func(ctx context.Context) (io interface{}, handle uint64, err error)

type ipfsNode struct {
	recordBase
	core coreiface.CoreAPI
}

type mfsNode struct {
	recordBase
	root *mfs.Root
	//fd mfs.FileDescriptor
}

type ipnsNode struct {
	ipfsNode
	mfsNode

	/* FIXME
	would be nice to have a union type, comprised of (ipfsNode, mfsNode)
	 where we can modify the vtable
	 so we don't have to generate a wrapper shim
	 i.e. make this compiler legal:
	 `node.Create()` == node.(ipfsNode, mfsNode).Create()
	 otherwise, go generate

	 if node.haskey { n.vtable = mfs }; else { n.vtable = core }
	*/
	key         coreiface.Key // may be nil for read only IO access
	fsRootIndex nameRootIndex // for shared mfs root object storage at FS level
}

type ipnsSubroot struct {
	ipnsNode
}

type keyNode struct {
	filesAPINode
	key coreiface.Key // TODO: api returns static object, we need to maintain it
	// fetch it in real time via api when needed, store string instead
}

type filesAPINode struct {
	mfsNode
}

func (nn *ipnsNode) String() string {
	if nn.key != nil {
		return gopath.Join(nn.key.Path(), nn.path)
	}
	return nn.path
}

func (_ *ipnsNode) NameSpace() string {
	return "ipns"
}

func (_ *filesAPINode) Namespace() string {
	return filesNamespace
}

func (fn *filesAPINode) String() string {
	return gopath.Join("/", fn.Namespace(), fn.path)
}

func (rb *recordBase) String() string {
	return rb.path
}

func (rb *recordBase) Handles() nodeHandles {
	return rb.ioHandles
}

func (rb *recordBase) Stat() (*fuse.Stat_t, error) {
	if rb.metadata.Mode == 0 {
		return nil, errNotInitialized
	}
	return &rb.metadata, nil
}

func (rb *recordBase) Remove(_ context.Context) (int, error) {
	return -fuse.EROFS, errReadOnly
}

func (fn *filesAPINode) Remove(_ context.Context) (int, error) {
	return mfsRemove(fn.root, fn.path[len(filesRootPath):])
}

func (mn *mfsNode) Remove(_ context.Context) (int, error) {
	return mfsRemove(mn.root, mn.path)
}

func (rb *recordBase) Create(_ context.Context, _ FsType) (int, error) {
	return -fuse.EROFS, errReadOnly
}

func (nr *ipnsSubroot) Create(ctx context.Context, nodeType FsType) (int, error) {
	keyComponent := strings.Split(nr.path, "/")[2]
	_, err := resolveKeyName(ctx, nr.core.Key(), keyComponent)
	switch err {
	case nil:
		return -fuse.EEXIST, os.ErrExist
	case errNoKey:
		break
	default:
		return -fuse.EIO, err
	}

	//NOTE: we rely on API to not clobber
	nr.key, err = nr.core.Key().Generate(ctx, keyComponent)
	if err != nil {
		log.Errorf("DBG: {%T}%#v", err, err)
		return -fuse.EIO, fmt.Errorf("could not generate key %q: %s", keyComponent, err)
	}

	ipldNode, err := emptyNode(ctx, nr.core.Dag(), nodeType)
	if err != nil {
		return -fuse.EIO, fmt.Errorf("could not generate ipld node for %q: %s", keyComponent, err)
	}

	if err = ipnsDelayedPublish(ctx, nr.key, ipldNode); err != nil {
		return -fuse.EIO, fmt.Errorf("could not publish to key %q: %s", keyComponent, err)
	}
	return fuseSuccess, nil
}

func (nn *ipnsNode) Create(ctx context.Context, nodeType FsType) (int, error) {
	nn.fsRootIndex.Lock()
	defer nn.fsRootIndex.Unlock()

	if nn.key == nil {
		return -fuse.EROFS, errReadOnly
	}

	//TODO: fetch mroot
	//mroot, err :=
	if err != nil {
	}
	// nn.root = mroot
	return nn.mfsRoot.Create(ctx, nodeType)

	if nn.root, err = nn.fsRootIndex.Request(nn.key.Name()); err != nil {
		return -fuse.EIO, err
	}
	nn.fsRootIndex.Release(nn.key.Name())
	//return mfsMknod(mroot, subPath)
	return nn.mfsNode.Create(ctx)

	_, keyName := gopath.Split(nn.path)
	newRootNode, err := emptyNode(fs.ctx, fs.core.Dag(), unixfs.TFile, nil)
	if err != nil {
		return -fuse.EIO, fmt.Errorf("could not generate unixdir %q: %s", keyName, err)
	}

	err = fs.ipnsDelayedPublish(coreKey, newRootNode)
	if err != nil {
		return -fuse.EIO, fmt.Errorf("could not publish to key %q: %s", keyName, err)
	}
	return fuseSuccess, nil

	//*ipnsNode:
	return fs.ipnsMknod(path)

}

func (mn *mfsNode) Create(ctx context.Context, nodeType FsType) (int, error) {
	//mfs lookup
	if _, err := mfs.Lookup(mn.root, mn.path); err == nil {
		return -fuse.EEXIST, os.ErrExist
	}

	parentPath, childName := gopath.Split(mn.path)
	mfsParent, err := mfs.Lookup(mn.root, parentPath)
	if err != nil {
		return -fuse.ENOENT, err
	}
	parentDir, ok := mfsNode.(*mfs.Directory)
	if !ok {
		return -fuse.ENOTDIR, fmt.Errorf("%s is not a directory", parentPath)
	}

	var ipldNode *ipld.Node

	switch nodeType {
	case unixfs.TFile:
		dagFile := dag.NodeWithData(unixfs.FilePBData(nil, 0))
		dagFile.SetCidBuilder(parentDir.GetCidBuilder())
		ipldNode = dagFile
	case unixfs.TDirectory:
		//TODO: review mkdir opts + Mkdir POSIX specs (are intermediate paths allowed by default?)
		if err = mfs.Mkdir(mn.root, mn.path, mfs.MkdirOpts{Flush: mfsSync}); err != nil {
			if err == mfs.ErrDirExists || err == os.ErrExist {
				return -fuse.EEXIST, os.ErrExist
			}
			return -fuse.EACCES, err
		}
		return fuseSuccess, nil
	default:
		return -fuse.EINVAL, errUnexpected
	}

	if err = mfs.PutNode(mn.root, mn.path, ipldNode); err != nil {
		return -fuse.EIO, err
	}

	err = mfsDir.AddChild(fName, dagNode)
	if err != nil {
		return -fuse.EIO, err
	}

	return fuseSuccess, nil

}

func (nn *ipnsNode) Remove(ctx context.Context) (int, error) {
	keyName, subPath := ipnsSplit(nn.path)
	if subPath == "" {
		if nn.key == nil {
			return -fuse.EPERM, errNoKey
		}
		_, err := nn.core.Key().Remove(ctx, keyName)
		if err != nil {
			return -fuse.EIO, fmt.Errorf("could not remove key %q: %s", keyName, err)
		}
		nn.key = nil
		return fuseSuccess, nil
	}

	nn.fsRootIndex.Lock()
	defer nn.fsRootIndex.Unlock()
	mroot, err := nn.fsRootIndex.Request(keyName)
	if err != nil {
		return -fuse.EIO, err
	}
	nn.fsRootIndex.Release(keyName)
	return mfsRemove(mroot, subPath)
}

//TODO: make a note somewhere that generic functions assume valid structs; define what "valid" means
func (rb *recordBase) Namespace() string {
	i := strings.IndexRune(rb.path[1:], '/')
	if i == -1 {
		return "root"
	}
	return rb.path[1:i]
}

func (*recordBase) Mutable() bool {
	return false
}

func (*ipnsNode) Mutable() bool {
	return true
}

// pedantic way of saying Unix permissions 0777 and 0555
const IRWXA = fuse.S_IRWXU | fuse.S_IRWXG | fuse.S_IRWXO
const IRXA = IRWXA &^ (fuse.S_IWUSR | fuse.S_IWGRP | fuse.S_IWOTH)

//TODO: document this upstream (cgofuse)
/* fuse.Getcontext only contains (useful) data in callstack under:
- Mknod
- Mkdir
- Getattr
- Open
- OpenDir
- Create
*/

func (rb *recordBase) InitMetadata(_ context.Context) (*fuse.Stat_t, error) {
	now := fuse.Now()
	rb.metadata.Birthtim, rb.metadata.Atim, rb.metadata.Mtim, rb.metadata.Ctim = now, now, now, now //!!!!
	rb.metadata.Mode |= IRXA
	return &rb.metadata, nil
}

func (in *ipfsNode) InitMetadata(ctx context.Context) (*fuse.Stat_t, error) {
	fStat, err := in.recordBase.InitMetadata(ctx)
	if err != nil {
		return fStat, err
	}

	corePath, err := coreiface.ParsePath(in.path)
	if err != nil {
		return fStat, err
	}

	ipldNode, err := in.core.ResolveNode(ctx, corePath)
	if err != nil {
		return fStat, err
	}
	err = ipldStat(fStat, ipldNode)
	return fStat, err
}

func (mn *mfsNode) InitMetadata(_ context.Context) (*fuse.Stat_t, error) {
	fStat, err := mn.recordBase.InitMetadata(nil)
	if err != nil {
		return fStat, err
	}

	err = mfsStat(fStat, mn.root, mn.path)
	return fStat, err
}

func (nr *ipnsSubroot) InitMetadata(ctx context.Context) (*fuse.Stat_t, error) {
	fStat, err := in.recordBase.InitMetadata(ctx)
	if err != nil {
		return fStat, err
	}

	ipldNode, err := resolveIpns(ctx, nr.String(), nr.core)
	if err != nil {
		return fStat, err
	}

	if err = ipldStat(fStat, ipldNode); err != nil {
		return fStat, err
	}

	if nn.key != nil && fStat.Mode&fuse.S_IFMT == fuse.S_IFDIR {
		//init MFS here?
		nr.path = nr.path[:1] // mfs relative
	}

	/*
		// wrap ipld node as mfs root construct
		if nn.root, err = mfsFromKey(ctx, coreKey.Name(), nn.core); err != nil {
			return -fuse.EACCES, err
		}
		nn.path = nn.path[:1] // promote path to root "/"
		nn.key = coreKey      // store key on fusePath

		nn.fsRootIndex.Register(coreKey.Name(), mroot)
		runtime.SetFinalizer(nn, ipnsNodeReleaseRoot)

		return fuseSuccess, nil

	*/
}

func (nn *ipnsNode) InitMetadata(ctx context.Context) (*fuse.Stat_t, error) {
	/* TODO: move the TTL out of stat, into bacgkround thread; refresh things at the FS level
	    last := time.Unix(nn.metadata.mtim.Sec, nn.metadata.mtim.Nsec)
	    if time.Since(last) < ipnsTTL && in.metadata.Mode != 0 {
		return &in.metadata, nil
	    }
	*/

	if nn.key == nil {
		return nn.ipfsNode.InitMetadata(ctx)
	}

	// if we own the key; check fs-level mfs index instead of IPFS
	var err error
	if nn.root, err = mfsFromNode(ctx, nn); err != nil {
		return nn.metadata, err
	}

	nn.fsRootIndex.Lock()
	defer nn.fsRootIndex.Unlock()
	nn.root, err = nn.fsRootIndex.Request(nn.key.Name())
	switch err {
	case errNotInitialized:
		if nn.root, err = mfsFromKey(ctx, nn.key, nn.core); err != nil {
			return nil, err
		}

		nn.fsRootIndex.Register(nn.key.Name(), nn.root)
		runtime.SetFinalizer(nn, ipnsNodeReleaseRoot)

	case nil:
		break
	default:
		return nil, err
	}

	err = mfsStat(&nn.metadata, mroot, subPath)
	return &nn.metadata, err
}

func (l *link) InitMetadata(_ context.Context) (*fuse.Stat_t, error) {
	l.metadata.Mode = fuse.S_IFLNK | IRXA
	l.metadata.Size = len(l.target)
}

func (rb *recordBase) typeCheck(nodeType FsType) (err error) {
	if !typeCheck(rb.metadata.Mode, nodeType) {
		return errIOType
	}
	return nil
}

func (in *ipfsNode) YieldIo(ctx context.Context, nodeType FsType) (io interface{}, err error) {
	if err := in.recordBase.typeCheck(nodeType); err != nil {
		return nil, err
	}

	switch nodeType {
	case unixfs.TFile:
		return coreYieldFileIO(ctx, in, in.core.Unixfs())
	case unixfs.TDirectory:
		return coreYieldDirIO(ctx, in, in.core, entryTimeout)

		/* TODO
		case unixfs.TSymlink:
			ipldNode, err := in.core.ResolveNode(ctx, in)
			if err != nil {
				return nil, err
			}
			target, err := ipldReadLink(ipldNode)
			if err != nil {
				return nil, err
			}
			return &link{target: target}
		*/
	default:
		return nil, errUnexpected
	}
}

func (nn *ipnsNode) YieldIo(ctx context.Context, nodeType FsType) (interface{}, error) {
	if err := nn.recordBase.typeCheck(nodeType); err != nil {
		return nil, err
	}

	if nn.key == nil { // use core for non-keyed paths
		return nn.ipfsNode.YieldIo(ctx, nodeType)
	}

	// check that our key is still valid
	if err = checkAPIKeystore(ctx, nn.core.Key(), nn.key); err != nil {
		nn.key = nil
		return nil, err
	}

	if nn.path == "/" {
		switch nn.metadata.Mode & fuse.S_IFMT {
		case fuse.S_IFREG:
			return keyYieldFileIO(ctx, nn.key, nn.core)
		case fuse.S_IFLNK:
			var (
				ipldNode ipld.Node
				target   string
				lnk      *link
			)
			if ipldNode, err = nn.core.ResolveNode(ctx, nn.key.Path()); err != nil {
				goto linkEnd
			}

			if target, err = ipldReadLink(ipldNode); err != nil {
				goto linkEnd
			}
			lnk = &link{target: target}
			_, err = lnk.InitMetadata(ctx)

		linkEnd:
			return lnk, err

		case fuse.S_IFDIR:
			// fallback to MFS IO handler to list out root contents
			break
		default:
			return nil, errUnexpected
		}
	}

	//handle other nodes via MFS
	nn.fsRootIndex.Lock()
	defer nn.fsRootIndex.Unlock()
	nn.root, err = nn.fsRootIndex.Request(keyName)
	switch err {
	case nil:
		break
	default:
		return nil, err
	case errNotInitialized:
		nn.root, err = ipnsToMFSRoot(ctx, key.Path(), nn.core)
		if err != nil {
			return nil, err
		}

		nn.fsRootIndex.Register(keyName, mroot)
		if nn.root, err = nn.fsRootIndex.Request(keyName); err != nil {
			return nil, err
		}
	}

	nnIO, err := nn.mfsNode.YieldIo(ctx)
	if err != nil {
		runtime.SetFinalizer(nnIO, ipnsKeyRootFree)
	}
	return nnIO, err
}

func (mn *mfsNode) YieldIo(ctx context.Context, nodeType FsType) (interface{}, error) {
	if err := in.recordBase.typeCheck(nodeType); err != nil {
		return nil, err
	}

	switch nodeType {
	case unixfs.TFile:
		return mfsYieldFileIO(mn.root, mn.path)
	case unixfs.TDirectory:
		ctx = context.WithValue(ctx, dagKey{}, mn.core.Dag()) //TODO: [2019.03.26] see note inside mfsSubNodes
		return mfsYieldDirIO(ctx, mn.root, mn.path, timeoutGrace, mn.core.Dag())
	case unixfs.TSymlink:
		fallthrough
	default:
		return nil, errUnexpected
	}
}

func (rb *recordBase) DestroyIo(fh uint64, nodeType FsType) (ret int, err error) {
	io, ok := rb.ioHandles[fh]
	if !ok {
		return -fuse.EBADF, fmt.Errorf("handle %X for %q is valid for record but not IO", fh, rb.path)
	}

	//FIXME: we need to set the finalizer for IO objects at initialization to call close in the rare event we error on these
	switch nodeType {
	case unixfs.TFile:
		if fio, ok := io.(*FsFile); !ok {
			ret = -fuse.EIO
			err = fmt.Errorf("handle %X for %q exists but type is mismatched{%T}", fh, rb.path, io)
		} else {
			ret = fio.Close()
		}
	case unixfs.TDirectory:
		if dio, ok := io.(*FsDirectory); !ok {
			ret = -fuse.EACCES
			err = fmt.Errorf("handle %X for %q exists but type is mismatched{%T}", fh, rb.path, io)
		} else {
			ret = dio.Close()
		}
	default:
		ret = -fuse.EBADF
		err = fmt.Errorf("handle %X for %q exists but type requested was unexpected{%#v}", fh, rb.path, nodeType)
	}

	// invalidate/free handle regardless of grace
	delete(rb.ioHandles, fh)
	return
}

/*
switch nodeType {
	case unixfs.TFile:
	case unixfs.TDirectory:
	case unixfs.TSymlink:
	}
*/

func typeCheck(pMode uint32, iType FsType) bool {
	pMode &= fuse.S_IFMT
	switch iType {
	case unixfs.TFile:
		if pMode == fuse.S_IFREG {
			return true
		}
	case unixfs.TDirectory, unixfs.THAMTShard:
		if pMode == fuse.S_IFDIR {
			return true
		}
	case unixfs.TSymlink:
		if pMode == fuse.S_IFLNK {
			return true
		}
	}
	return false
}

// link's target replaces its original path at the FS level
// link's metadata remains separate from target's
type link struct {
	fusePath
	metadata fuse.Stat_t
	target   string
}

func (ln *link) String() string {
	return ln.target
}

func (ln *link) Target() string {
	return ln.target
}

//TODO: log these to test
func ipnsNodeReleaseRoot(nn *ipnsNode) {
	nn.fsRootIndex.Lock()
	defer nn.fsRootIndex.Unlock()
	nn.fsRootIndex.Release(nn.key.Name())
}

//XXX
func ipnsIoReleaseRoot(io FsDirectory) {
	fp := io.Record()
	nn, ok := fp.(*ipnsNode)
	if !ok {
		return
	}
	nn.fsRootIndex.Lock()
	defer nn.fsRootIndex.Unlock()
	nn.fsRootIndex.Release(nn.key.Name())
}
