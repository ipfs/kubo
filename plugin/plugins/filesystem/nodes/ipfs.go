package fsnodes

import (
	"context"
	"fmt"
	"io"

	"github.com/djdv/p9/p9"
	files "github.com/ipfs/go-ipfs-files"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

var _ p9.File = (*IPFS)(nil)
var _ walkRef = (*IPFS)(nil)

// IPFS exposes the IPFS API over a p9.File interface
// Walk does not expect a namespace, only its path argument
// e.g. `ipfs.Walk([]string("Qm...", "subdir")` not `ipfs.Walk([]string("ipfs", "Qm...", "subdir")`
type IPFS struct {
	IPFSBase
	IPFSFileMeta
}

type IPFSFileMeta struct {
	// operation handle storage
	file      files.File
	directory *directoryStream
}

func IPFSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	return &IPFS{IPFSBase: newIPFSBase(ctx, "/ipfs", core, ops...)}
}

func (id *IPFS) Derive() walkRef {
	return &IPFS{IPFSBase: id.IPFSBase.Derive()}
}

func (id *IPFS) Attach() (p9.File, error) {
	id.Logger.Debugf("Attach")

	newFid := new(IPFS)
	*newFid = *id

	return newFid, nil
}

func coreAttr(ctx context.Context, attr *p9.Attr, path corepath.Resolved, core coreiface.CoreAPI, req p9.AttrMask) (p9.AttrMask, error) {
	ipldNode, err := core.Dag().Get(ctx, path.Cid())
	if err != nil {
		return p9.AttrMask{}, err
	}

	return ipldStat(ctx, attr, ipldNode, req)
}

func (id *IPFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	id.Logger.Debugf("GetAttr")

	// TODO: we need to re use storage on id
	// merge existing members with request members fetched via request
	// rather than generating a new copy each time
	var (
		attr            p9.Attr
		filled          p9.AttrMask
		callCtx, cancel = id.callCtx()
	)
	defer cancel()

	corePath, err := id.core.ResolvePath(callCtx, id.CorePath())
	if err != nil {
		return id.Qid, filled, attr, err
	}

	filled, err = coreAttr(callCtx, &attr, corePath, id.core, req)
	if err != nil {
		id.Logger.Error(err)
		return id.Qid, filled, attr, err
	}

	if req.RDev {
		attr.RDev, filled.RDev = dIPFS, true
	}

	attr.Mode |= IRXA

	return id.Qid, filled, attr, err
}

func (id *IPFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	id.Logger.Debugf("Walk names: %v", names)
	id.Logger.Debugf("Walk myself: %q:{%d}", id.StringPath(), id.NinePath())

	return walker(id, names)
}

func (id *IPFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	id.Logger.Debugf("Open: %s", id.StringPath())

	// set up  handle amenities
	//var handleContext context.Context
	//handleContext, id.operationsCancel = context.WithCancel(id.filesystemCtx)

	// handle directories
	if id.Qid.Type == p9.TypeDir {
		//		c, err := id.core.Unixfs().Ls(handleContext, id.CorePath())
		c, err := id.core.Unixfs().Ls(id.filesystemCtx, id.CorePath())
		if err != nil {
			//id.operationsCancel()
			return id.Qid, 0, err
		}

		id.directory = &directoryStream{
			entryChan: c,
		}
		return id.Qid, 0, nil
	}

	// handle files
	apiNode, err := id.core.Unixfs().Get(id.filesystemCtx, id.CorePath())
	if err != nil {
		//id.operationsCancel()
		return id.Qid, 0, err
	}

	fileNode, ok := apiNode.(files.File)
	if !ok {
		//id.operationsCancel()
		return id.Qid, 0, fmt.Errorf("%q does not appear to be a file: %T", id.StringPath(), apiNode)
	}
	id.file = fileNode
	s, err := id.file.Size()
	if err != nil {
		//id.operationsCancel()
		return id.Qid, 0, err
	}
	id.meta.Size, id.metaMask.Size = uint64(s), true

	return id.Qid, ipfsBlockSize, nil
}

func (id *IPFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	id.Logger.Debugf("Readdir %q %d %d", id.StringPath(), offset, count)

	if id.directory == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", id.StringPath())
	}

	if id.directory.err != nil { // previous request must have failed
		return nil, id.directory.err
	}

	if id.directory.eos {
		if offset == id.directory.cursor-1 {
			return nil, nil // this is the only exception to offset being behind the cursor
		}
	}

	if offset < id.directory.cursor {
		return nil, fmt.Errorf("read offset %d is behind current entry %d, seeking backwards in directory streams is not supported", offset, id.directory.cursor)
	}

	ents := make([]p9.Dirent, 0)

	for len(ents) < int(count) {
		select {
		case entry, open := <-id.directory.entryChan:
			if !open {
				//id.operationsCancel()
				id.directory.eos = true
				return ents, nil
			}
			if entry.Err != nil {
				id.directory.err = entry.Err
				return nil, entry.Err
			}

			if offset <= id.directory.cursor {
				nineEnt, err := coreEntTo9Ent(entry)
				if err != nil {
					id.directory.err = err
					return nil, err
				}
				nineEnt.Offset = id.directory.cursor
				ents = append(ents, nineEnt)
			}

			id.directory.cursor++

		case <-id.filesystemCtx.Done():
			id.directory.err = id.filesystemCtx.Err()
			id.Logger.Error(id.directory.err)
			return ents, id.directory.err
		}
	}

	id.Logger.Debugf("Readdir returning [%d]%v\n", len(ents), ents)
	return ents, nil
}

func (id *IPFS) ReadAt(p []byte, offset uint64) (int, error) {
	const (
		readAtFmt    = "ReadAt {%d/%d}%q"
		readAtFmtErr = readAtFmt + ": %s"
	)
	//id.Logger.Debugf(readAtFmt, offset, id.meta.Size, id.StringPath())

	if id.file == nil {
		err := fmt.Errorf("file is not open for reading")
		id.Logger.Errorf(readAtFmtErr, offset, id.meta.Size, id.StringPath(), err)
		return 0, err
	}

	if offset >= id.meta.Size {
		//NOTE [styx]: If the offset field is greater than or equal to the number of bytes in the file, a count of zero will be returned.
		return 0, io.EOF
	}

	if _, err := id.file.Seek(int64(offset), io.SeekStart); err != nil {
		//id.operationsCancel()
		id.Logger.Errorf(readAtFmtErr, offset, id.meta.Size, id.StringPath(), err)
		return 0, err
	}

	readBytes, err := id.file.Read(p)
	if err != nil && err != io.EOF {
		id.Logger.Errorf(readAtFmtErr, offset, id.meta.Size, id.StringPath(), err)
		//id.operationsCancel()
		return 0, err
	}

	return readBytes, err
}

func (id *IPFS) Close() error {
	lastErr := id.IPFSBase.Close()
	if lastErr != nil {
		id.Logger.Error(lastErr)
	}

	if id.file != nil {
		if err := id.file.Close(); err != nil {
			id.Logger.Error(err)
			lastErr = err
		}
		id.file = nil
	}
	id.directory = nil

	return lastErr
}

func coreToQid(ctx context.Context, path corepath.Path, core coreiface.CoreAPI) (p9.QID, error) {
	var qid p9.QID
	// translate from abstract path to CoreAPI resolved path
	resolvedPath, err := core.ResolvePath(ctx, path)
	if err != nil {
		return qid, err
	}

	// inspected to derive 9P QID
	attr := new(p9.Attr)
	_, err = coreAttr(ctx, attr, resolvedPath, core, p9.AttrMask{Mode: true})
	if err != nil {
		return qid, err
	}

	qid.Type = attr.Mode.QIDType()
	qid.Path = cidToQPath(resolvedPath.Cid())
	return qid, nil
}

// IPFS appends "name" to its current path, and returns itself
func (id *IPFS) Step(name string) (walkRef, error) {
	callCtx, cancel := id.callCtx()
	defer cancel()

	qid, err := coreToQid(callCtx, id.CorePath(name), id.core)
	if err != nil {
		return nil, err
	}

	// we stepped successfully, so set up a newFid to return
	newFid := id.Derive().(*IPFS)
	newFid.Trail = append(newFid.Trail, name)
	newFid.Qid = qid

	return newFid, nil
}

func (id *IPFS) Backtrack() (walkRef, error) {
	// if we're the root
	if len(id.Trail) == 0 {
		// return our parent, or ourselves if we don't have one
		if id.parent != nil {
			return id.parent, nil
		}
		return id, nil
	}

	// otherwise step back
	tLen := len(id.Trail)
	breadCrumb := make([]string, tLen)
	copy(breadCrumb, id.Trail)

	id.Trail = id.Trail[:tLen-1]

	// reset QID
	callCtx, cancel := id.callCtx()
	defer cancel()

	qid, err := coreToQid(callCtx, id.CorePath(), id.core)
	if err != nil {
		// recover path on failure
		id.Trail = breadCrumb
		return nil, err
	}

	// set on success; we stepped back
	id.Qid = qid

	return id, nil
}
