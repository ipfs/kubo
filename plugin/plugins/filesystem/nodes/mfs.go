package fsnodes

import (
	"context"
	"errors"
	"fmt"
	"io"
	gopath "path"
	"time"

	"github.com/hugelgupf/p9/p9"
	"github.com/hugelgupf/p9/unimplfs"
	cid "github.com/ipfs/go-cid"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	fsutils "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/utils"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-unixfs"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var _ p9.File = (*MFS)(nil)
var _ fsutils.WalkRef = (*MFS)(nil)

// TODO: break this up into 2 file systems?
// MFS + MFS Overlay?
// TODO: docs
type MFS struct {
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr

	IPFSBase
	MFSFileMeta

	//ref   uint                 //TODO: rename, root refcount
	//key   coreiface.Key        // optional value, if set, publish to IPNS key on MFS change
	//roots map[string]*mfs.Root //share the same mfs root across calls
	mroot *mfs.Root
}

type MFSFileMeta struct {
	//file      mfs.FileDescriptor
	openFlags p9.OpenFlags //TODO: move this to IPFSBase; use as open marker
	file      *mfs.File
	directory *mfs.Directory
}

func MFSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	options := nodeopts.AttachOps(ops...)

	if !options.MFSRoot.Defined() {
		panic("MFS root cid is required but was not defined in options")
	}

	mroot, err := cidToMFSRoot(ctx, options.MFSRoot, core, options.MFSPublish)
	if err != nil {
		panic(err)
	}

	md := &MFS{
		IPFSBase: newIPFSBase(ctx, "/ipld", core, ops...),
		mroot:    mroot,
	}
	md.meta.Mode, md.metaMask.Mode = p9.ModeDirectory|IRXA|0220, true

	return md
}

func (md *MFS) Fork() (fsutils.WalkRef, error) {
	base, err := md.IPFSBase.fork()
	if err != nil {
		return nil, err
	}

	newFid := &MFS{
		IPFSBase: base,
		mroot:    md.mroot,
	}
	return newFid, nil

}

func (md *MFS) Attach() (p9.File, error) {
	md.Logger.Debugf("Attach")

	newFid := new(MFS)
	*newFid = *md

	return newFid, nil
}

func (md *MFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	md.Logger.Debugf("GetAttr path: %s", md.StringPath())
	md.Logger.Debugf("%p", md)

	qid, err := md.QID()
	if err != nil {
		return qid, p9.AttrMask{}, p9.Attr{}, err
	}

	attr, filled, err := md.getAttr(req)
	if err != nil {
		return qid, filled, attr, err
	}

	if req.RDev {
		attr.RDev, filled.RDev = dMemory, true
	}

	if req.Mode {
		attr.Mode |= IRXA | 0220
	}

	*md.qid = qid

	return qid, filled, attr, nil
}

func (md *MFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	md.Logger.Debugf("Walk names: %v", names)
	md.Logger.Debugf("Walk myself: %q:{%d}", md.StringPath(), md.ninePath())

	return fsutils.Walker(md, names)
}

func (md *MFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	md.Logger.Debugf("Open %q {Mode:%v OSFlags:%v, String:%s}", md.StringPath(), mode.Mode(), mode.OSFlags(), mode.String())
	md.Logger.Debugf("%p", md)

	if md.mroot == nil {
		return *md.qid, 0, fmt.Errorf("TODO: message; root not set")
	}

	attr, _, err := md.getAttr(p9.AttrMask{Mode: true})
	if err != nil {
		return *md.qid, 0, err
	}

	switch {
	case attr.Mode.IsDir():
		dir, err := md.getDirectory()
		if err != nil {
			return *md.qid, 0, err
		}

		md.directory = dir

	case attr.Mode.IsRegular():
		mFile, err := md.getFile()
		if err != nil {
			return *md.qid, 0, err
		}

		md.file = mFile
	}

	md.openFlags = mode // TODO: convert to MFS native flags
	return *md.qid, ipfsBlockSize, nil
}

func (md *MFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	md.Logger.Debugf("Readdir %d %d", offset, count)

	if md.directory == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", md.StringPath())
	}

	//TODO: resetable context; for { ...; ctx.reset() }
	callCtx, cancel := context.WithCancel(md.filesystemCtx)
	defer cancel()

	ents := make([]p9.Dirent, 0)

	var index uint64
	var done bool
	err := md.directory.ForEachEntry(callCtx, func(nl mfs.NodeListing) error {
		if done {
			cancel()
			return nil
		}

		if index < offset {
			index++ //skip
			return nil
		}

		ent, err := mfsEntTo9Ent(nl)
		if err != nil {
			return err
		}

		ent.Offset = index + 1

		ents = append(ents, ent)
		if len(ents) == int(count) {
			done = true
		}

		index++
		return nil
	})

	return ents, err
}

func (md *MFS) ReadAt(p []byte, offset uint64) (int, error) {
	const (
		readAtFmt    = "ReadAt {%d/%d}%q"
		readAtFmtErr = readAtFmt + ": %s"
	)

	if md.file == nil {
		err := fmt.Errorf("file is not open for reading")
		md.Logger.Errorf(readAtFmtErr, offset, md.meta.Size, md.StringPath(), err)
		return 0, err
	}

	attr, _, err := md.getAttr(p9.AttrMask{Size: true})
	if err != nil {
		return 0, err
	}

	if offset >= attr.Size {
		//NOTE [styx]: If the offset field is greater than or equal to the number of bytes in the file, a count of zero will be returned.
		return 0, io.EOF
	}

	openFile, err := md.file.Open(mfs.Flags{Read: true})
	if err != nil {
		return 0, err
	}
	defer openFile.Close()

	if _, err := openFile.Seek(int64(offset), io.SeekStart); err != nil {
		md.Logger.Errorf(readAtFmtErr, offset, attr.Size, md.StringPath(), err)
		return 0, err
	}

	return openFile.Read(p)
}

func (md *MFS) SetAttr(valid p9.SetAttrMask, attr p9.SetAttr) error {
	md.Logger.Debugf("SetAttr %v %v", valid, attr)
	md.Logger.Debugf("%p", md)

	if valid.Size {
		var target *mfs.File

		if md.file != nil {
			target = md.file
		} else {
			mFile, err := md.getFile()
			if err != nil {
				return err
			}

			target = mFile
		}

		openFile, err := target.Open(mfs.Flags{Read: true, Write: true})
		if err != nil {
			return err
		}
		defer openFile.Close()

		if err := openFile.Truncate(int64(attr.Size)); err != nil {
			return err
		}
	}

	md.meta.Apply(valid, attr)
	return nil
}

func (md *MFS) WriteAt(p []byte, offset uint64) (int, error) {
	const (
		readAtFmt    = "WriteAt {%d/%d}%q"
		readAtFmtErr = readAtFmt + ": %s"
	)
	if md.file == nil {
		err := fmt.Errorf("file is not open for writing")
		md.Logger.Errorf(readAtFmtErr, offset, md.meta.Size, md.StringPath(), err)
		return 0, err
	}

	openFile, err := md.file.Open(mfs.Flags{Read: true, Write: true})
	if err != nil {
		return 0, err
	}
	defer openFile.Close()

	nbytes, err := openFile.WriteAt(p, int64(offset))
	if err != nil {
		md.Logger.Errorf(readAtFmtErr, offset, md.meta.Size, md.StringPath(), err)
		return nbytes, err
	}

	if err = openFile.Flush(); err != nil {
		return nbytes, err
	}

	return nbytes, nil

	//return md.file.WriteAt(p, int64(offset))
}

func (md *MFS) Close() error {
	md.Logger.Debugf("closing: %q:{%d}", md.StringPath(), md.ninePath())

	md.file = nil
	md.directory = nil
	return nil
}

/*
{
    Base: {
	coreNamespace: `/ipld`,
	Trail: []string{"folder", "file.txt"}
    }
    mroot: fromCid(`QmVuDpaFj55JnUH7UYxTAydx6ayrs2cB3Gb7cdPr61wLv5`)
}
=>
`/ipld/QmVuDpaFj55JnUH7UYxTAydx6ayrs2cB3Gb7cdPr61wLv5/folder/file.txt`
*/
func (md *MFS) StringPath() string {
	rootNode, err := md.mroot.GetDirectory().GetNode()
	if err != nil {
		panic(err)
	}
	return gopath.Join(append([]string{md.coreNamespace, rootNode.Cid().String()}, md.Trail...)...)
}

func (md *MFS) Step(name string) (fsutils.WalkRef, error) {

	// FIXME: [in general] Step should return ref, qid, error
	// obviate CheckWalk + QID and make this implicit via Step
	qid, err := md.QID()
	if err != nil {
		return nil, err
	}

	*md.qid = qid
	//

	return md.step(md, name)
}

/*
func (md *MFS) RootPath(keyName string, components ...string) (corepath.Path, error) {
	if keyName == "" {
		return nil, fmt.Errorf("no path key was provided")
	}

	rootCid, err := cid.Decode(keyName)
	if err != nil {
		return nil, err
	}

	return corepath.Join(corepath.IpldPath(rootCid), components...), nil
}

func (md *MFS) ResolvedPath(names ...string) (corepath.Path, error) {
	callCtx, cancel := md.callCtx()
	defer cancel()

	return md.core.ResolvePath(callCtx, md.KeyPath(names[0], names[1:]...))

	corePath = corepath.IpldPath(md.Tail[0])
	return md.core.ResolvePath(callCtx, corepath.Join(corePath, append(md.Tail[1:], names)...))
}
*/

func cidToMFSRoot(ctx context.Context, rootCid cid.Cid, core coreiface.CoreAPI, publish mfs.PubFunc) (*mfs.Root, error) {

	if !rootCid.Defined() {
		return nil, errors.New("root cid was not defined")
	}

	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ipldNode, err := core.Dag().Get(callCtx, rootCid)
	if err != nil {
		return nil, err
	}

	pbNode, ok := ipldNode.(*dag.ProtoNode)
	if !ok {
		return nil, fmt.Errorf("%q has incompatible type %T", rootCid.String(), ipldNode)
	}

	return mfs.NewRoot(ctx, core.Dag(), pbNode, publish)
}

func (md *MFS) CheckWalk() error                    { return md.Base.checkWalk() }
func (md *MFS) Backtrack() (fsutils.WalkRef, error) { return md.IPFSBase.backtrack(md) }
func (md *MFS) QID() (p9.QID, error) {
	mNode, err := mfs.Lookup(md.mroot, gopath.Join(md.Trail...))
	if err != nil {
		return p9.QID{}, err
	}

	t, err := mfsTypeToNineType(mNode.Type())
	if err != nil {
		return p9.QID{}, err
	}

	ipldNode, err := mNode.GetNode()
	if err != nil {
		return p9.QID{}, err
	}

	return p9.QID{
		Type: t,
		Path: cidToQPath(ipldNode.Cid()),
	}, nil
}

func (md *MFS) getNode() (ipld.Node, error) {
	mNode, err := mfs.Lookup(md.mroot, gopath.Join(md.Trail...))
	if err != nil {
		return nil, err
	}
	return mNode.GetNode()
}

func (md *MFS) getFile() (*mfs.File, error) {
	mNode, err := mfs.Lookup(md.mroot, gopath.Join(md.Trail...))
	if err != nil {
		return nil, err
	}

	mFile, ok := mNode.(*mfs.File)
	if !ok {
		return nil, fmt.Errorf("type mismatch %q is %T not a file", md.StringPath(), mNode)
	}

	return mFile, nil
}

func (md *MFS) getDirectory() (*mfs.Directory, error) {
	mNode, err := mfs.Lookup(md.mroot, gopath.Join(md.Trail...))
	if err != nil {
		return nil, err
	}

	dir, ok := mNode.(*mfs.Directory)
	if !ok {
		return nil, fmt.Errorf("type mismatch %q is %T not a directory", md.StringPath(), mNode)
	}
	return dir, nil
}

func (md *MFS) getAttr(req p9.AttrMask) (p9.Attr, p9.AttrMask, error) {
	var attr p9.Attr

	mfsNode, err := mfs.Lookup(md.mroot, gopath.Join(md.Trail...))
	if err != nil {
		return attr, p9.AttrMask{}, err
	}

	ipldNode, err := mfsNode.GetNode()
	if err != nil {
		return attr, p9.AttrMask{}, err
	}

	callCtx, cancel := md.callCtx()
	defer cancel()

	filled, err := ipldStat(callCtx, &attr, ipldNode, req)
	if err != nil {
		md.Logger.Error(err)
	}
	return attr, filled, err
}

func (md *MFS) Create(name string, flags p9.OpenFlags, permissions p9.FileMode, uid p9.UID, gid p9.GID) (p9.File, p9.QID, uint32, error) {
	callCtx, cancel := md.callCtx()
	defer cancel()

	emptyNode, err := emptyNode(callCtx, md.core.Dag())
	if err != nil {
		return nil, p9.QID{}, 0, err
	}

	err = mfs.PutNode(md.mroot, gopath.Join(append(md.Trail, name)...), emptyNode)
	if err != nil {
		return nil, p9.QID{}, 0, err
	}

	newFid, err := md.Fork()
	if err != nil {
		return nil, p9.QID{}, 0, err
	}

	newRef, err := newFid.Step(name)
	if err != nil {
		return nil, p9.QID{}, 0, err
	}

	qid, ioUnit, err := newRef.Open(flags)
	return newRef, qid, ioUnit, err
}

func emptyNode(ctx context.Context, dagAPI coreiface.APIDagService) (ipld.Node, error) {
	eFile := dag.NodeWithData(unixfs.FilePBData(nil, 0))
	if err := dagAPI.Add(ctx, eFile); err != nil {
		return nil, err
	}
	return eFile, nil
}

func (md *MFS) Mkdir(name string, permissions p9.FileMode, uid p9.UID, gid p9.GID) (p9.QID, error) {
	err := mfs.Mkdir(md.mroot, gopath.Join(append(md.Trail, name)...), mfs.MkdirOpts{Flush: true})
	if err != nil {
		return p9.QID{}, err
	}

	newFid, err := md.Fork()
	if err != nil {
		return p9.QID{}, err
	}
	newRef, err := newFid.Step(name)
	if err != nil {
		return p9.QID{}, err
	}

	return newRef.QID()
}

func (md *MFS) parentDir() (*mfs.Directory, error) {
	parent := gopath.Dir(gopath.Join(md.Trail...))

	mNode, err := mfs.Lookup(md.mroot, parent)
	if err != nil {
		return nil, err
	}

	dir, ok := mNode.(*mfs.Directory)
	if !ok {
		return nil, fmt.Errorf("type mismatch %q is %T not a directory", md.StringPath(), mNode)
	}
	return dir, nil
}

func (md *MFS) Mknod(name string, mode p9.FileMode, major uint32, minor uint32, uid p9.UID, gid p9.GID) (p9.QID, error) {
	callCtx, cancel := md.callCtx()
	defer cancel()

	emptyNode, err := emptyNode(callCtx, md.core.Dag())
	if err != nil {
		return p9.QID{}, err
	}

	err = mfs.PutNode(md.mroot, gopath.Join(append(md.Trail, name)...), emptyNode)
	if err != nil {
		return p9.QID{}, err
	}

	newFid, err := md.Fork()
	if err != nil {
		return p9.QID{}, err
	}
	newRef, err := newFid.Step(name)
	if err != nil {
		return p9.QID{}, err
	}

	return newRef.QID()
}
