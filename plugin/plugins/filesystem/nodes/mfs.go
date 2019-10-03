package fsnodes

import (
	"context"
	"fmt"
	"io"
	gopath "path"
	"time"

	"github.com/djdv/p9/p9"
	cid "github.com/ipfs/go-cid"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-mfs"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var _ p9.File = (*MFS)(nil)
var _ walkRef = (*MFS)(nil)

// TODO: break this up into 2 file systems?
// MFS + MFS Overlay?
// TODO: docs
type MFS struct {
	IPFSBase
	MFSFileMeta

	//ref   uint                 //TODO: rename, root refcount
	//key   coreiface.Key        // optional value, if set, publish to IPNS key on MFS change
	//roots map[string]*mfs.Root //share the same mfs root across calls
	mroot *mfs.Root
}

type MFSFileMeta struct {
	//TODO: re-use base path for this
	//relativePath string
	//rootName     string
	file      mfs.FileDescriptor
	directory *mfs.Directory
}

func MFSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	options := nodeopts.AttachOps(ops...)

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

func (md *MFS) Derive() walkRef {
	newFid := &MFS{
		IPFSBase: md.IPFSBase.Derive(),
		//MFSFileMeta: md.MFSFileMeta,
		mroot: md.mroot,
	}

	return newFid
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

	callCtx, cancel := md.callCtx()
	defer cancel()

	filled, err := mfsAttr(callCtx, md.meta, md.mroot, p9.AttrMaskAll, md.Trail...)
	if err != nil {
		return md.Qid, filled, *md.meta, err
	}

	md.meta.Mode |= IRXA | 0220
	if req.RDev {
		md.meta.RDev, filled.RDev = dMemory, true
	}

	return md.Qid, filled, *md.meta, nil
}

func (md *MFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	md.Logger.Debugf("Walk names: %v", names)
	md.Logger.Debugf("Walk myself: %q:{%d}", md.StringPath(), md.NinePath())

	md.Logger.Debugf("%p", md)
	return walker(md, names)
}

func (md *MFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	md.Logger.Debugf("Open %q {Mode:%v OSFlags:%v, String:%s}", md.StringPath(), mode.Mode(), mode.OSFlags(), mode.String())
	md.Logger.Debugf("%p", md)

	if md.mroot == nil {
		return md.Qid, 0, fmt.Errorf("TODO: message; root not set")
	}

	//TODO: current: lookup -> mfsnoode -> md.file | md.directory = mfsNode.open(fsctx)

	mNode, err := mfs.Lookup(md.mroot, gopath.Join(md.Trail...))
	if err != nil {
		return md.Qid, 0, err
	}

	// handle directories
	if md.meta.Mode.IsDir() {
		dir, ok := mNode.(*mfs.Directory)
		if !ok {
			return md.Qid, 0, fmt.Errorf("type mismatch %q is %T not a directory", md.StringPath(), mNode)
		}
		md.directory = dir
	} else {
		mFile, ok := mNode.(*mfs.File)
		if !ok {
			return md.Qid, 0, fmt.Errorf("type mismatch %q is %T not a file", md.StringPath(), mNode)
		}

		openFile, err := mFile.Open(mfs.Flags{Read: true, Write: true})
		if err != nil {
			return md.Qid, 0, err
		}
		s, err := openFile.Size()
		if err != nil {
			return md.Qid, 0, err
		}

		md.file = openFile
		md.meta.Size, md.metaMask.Size = uint64(s), true
	}

	return md.Qid, ipfsBlockSize, nil
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

	if offset >= md.meta.Size {
		//NOTE [styx]: If the offset field is greater than or equal to the number of bytes in the file, a count of zero will be returned.
		return 0, io.EOF
	}

	if _, err := md.file.Seek(int64(offset), io.SeekStart); err != nil {
		md.Logger.Errorf(readAtFmtErr, offset, md.meta.Size, md.StringPath(), err)
		return 0, err
	}

	//TODO: remove, debug

	nbytes, err := md.file.Read(p)
	if err != nil {
		md.Logger.Errorf(readAtFmtErr, offset, md.meta.Size, md.StringPath(), err)
	}

	return nbytes, err
	//

	//return md.file.Read(p)
}

func (md *MFS) SetAttr(valid p9.SetAttrMask, attr p9.SetAttr) error {
	md.Logger.Debugf("SetAttr %v %v", valid, attr)
	md.Logger.Debugf("%p", md)

	if valid.Size && attr.Size < md.meta.Size {
		if md.file == nil {
			err := fmt.Errorf("file %q is not open, cannot change size", md.StringPath())
			md.Logger.Error(err)
			return err
		}

		if err := md.file.Truncate(int64(attr.Size)); err != nil {
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

	//TODO: remove, debug

	nbytes, err := md.file.WriteAt(p, int64(offset))
	if err != nil {
		md.Logger.Errorf(readAtFmtErr, offset, md.meta.Size, md.StringPath(), err)
	}

	return nbytes, err
	//

	//return md.file.WriteAt(p, int64(offset))
}

func (md *MFS) Close() error {
	md.Logger.Debugf("closing: %q:{%d}", md.StringPath(), md.Qid.Path)
	md.Logger.Debugf("%p", md)

	var lastErr error
	if err := md.Base.Close(); err != nil {
		md.Logger.Error(err)
		lastErr = err
	}

	if md.file != nil {
		if err := md.file.Close(); err != nil {
			md.Logger.Error(err)
			lastErr = err
		}
	}

	return lastErr
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

func (md *MFS) Backtrack() (walkRef, error) {
	// if we're the root
	if len(md.Trail) == 0 {
		// return our parent, or ourselves if we don't have one
		if md.parent != nil {
			return md.parent, nil
		}
		return md, nil
	}

	// otherwise step back
	tLen := len(md.Trail)
	breadCrumb := make([]string, tLen)
	copy(breadCrumb, md.Trail)

	md.Trail = md.Trail[:tLen-1]

	// reset QID
	callCtx, cancel := md.callCtx()
	defer cancel()

	qid, err := coreToQid(callCtx, md.CorePath(), md.core)
	if err != nil {
		// recover path on failure
		md.Trail = breadCrumb
		return nil, err
	}

	// set on success; we stepped back
	md.Qid = qid

	return md, nil
}

func mfsAttr(ctx context.Context, attr *p9.Attr, mroot *mfs.Root, req p9.AttrMask, names ...string) (p9.AttrMask, error) {
	mfsNode, err := mfs.Lookup(mroot, gopath.Join(names...))
	if err != nil {
		return p9.AttrMask{}, err
	}

	ipldNode, err := mfsNode.GetNode()
	if err != nil {
		return p9.AttrMask{}, err
	}

	return ipldStat(ctx, attr, ipldNode, req)
}

func mfsToQid(ctx context.Context, mroot *mfs.Root, names ...string) (p9.QID, error) {
	mNode, err := mfs.Lookup(mroot, gopath.Join(names...))
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

func (md *MFS) Step(name string) (walkRef, error) {
	callCtx, cancel := md.callCtx()
	defer cancel()

	breadCrumb := append(md.Trail, name)
	qid, err := mfsToQid(callCtx, md.mroot, breadCrumb...)
	if err != nil {
		return nil, err
	}

	// set on success; we stepped
	md.Trail = breadCrumb
	md.Qid = qid
	return md, nil
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

func cidToMFSRoot(ctx context.Context, rootCid *cid.Cid, core coreiface.CoreAPI, publish mfs.PubFunc) (*mfs.Root, error) {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ipldNode, err := core.Dag().Get(callCtx, *rootCid)
	if err != nil {
		return nil, err
	}

	pbNode, ok := ipldNode.(*dag.ProtoNode)
	if !ok {
		return nil, fmt.Errorf("%q has incompatible type %T", rootCid.String(), ipldNode)
	}

	return mfs.NewRoot(ctx, core.Dag(), pbNode, publish)
}
