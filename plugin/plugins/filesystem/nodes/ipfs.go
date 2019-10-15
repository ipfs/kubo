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
	id := &IPFS{IPFSBase: newIPFSBase(ctx, "/ipfs", core, ops...)}
	id.Qid.Type = p9.TypeDir
	id.meta.Mode, id.metaMask.Mode = p9.ModeDirectory|IRXA, true
	return id
}

func (id *IPFS) Fork() (walkRef, error) {
	base, err := id.IPFSBase.Fork()
	if err != nil {
		return nil, err
	}
	return &IPFS{IPFSBase: base}, nil
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
		qid             = *id.Qid
	)
	defer cancel()

	corePath, err := id.core.ResolvePath(callCtx, id.CorePath())
	if err != nil {
		return qid, filled, attr, err
	}

	filled, err = coreAttr(callCtx, &attr, corePath, id.core, req)
	if err != nil {
		id.Logger.Error(err)
		return qid, filled, attr, err
	}

	if req.RDev {
		attr.RDev, filled.RDev = dIPFS, true
	}

	if req.Mode {
		attr.Mode |= IRXA
		qid.Type = attr.Mode.QIDType()
		id.Qid.Type = attr.Mode.QIDType()
	}

	return qid, filled, attr, err
}

func (id *IPFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	id.Logger.Debugf("Walk names: %v", names)
	id.Logger.Debugf("Walk myself: %q:{%d}", id.String(), id.NinePath())

	return walker(id, names)
}

func (id *IPFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	id.Logger.Debugf("Open: %s", id.String())

	qid := *id.Qid

	// handle directories
	if qid.Type == p9.TypeDir {
		c, err := id.core.Unixfs().Ls(id.operationsCtx, id.CorePath())
		if err != nil {
			//id.operationsCancel()
			return qid, 0, err
		}

		id.directory = &directoryStream{
			entryChan: c,
		}
		return qid, 0, nil
	}

	// handle files
	apiNode, err := id.core.Unixfs().Get(id.operationsCtx, id.CorePath())
	if err != nil {
		//id.operationsCancel()
		return qid, 0, err
	}

	fileNode, ok := apiNode.(files.File)
	if !ok {
		//id.operationsCancel()
		return qid, 0, fmt.Errorf("%q does not appear to be a file: %T", id.String(), apiNode)
	}
	id.file = fileNode
	s, err := id.file.Size()
	if err != nil {
		//id.operationsCancel()
		return qid, 0, err
	}
	id.meta.Size, id.metaMask.Size = uint64(s), true

	return qid, ipfsBlockSize, nil
}

func (id *IPFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	id.Logger.Debugf("Readdir %q %d %d", id.String(), offset, count)

	if id.directory == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", id.String())
	}

	if id.directory.err != nil { // previous request must have failed
		return nil, id.directory.err
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
				return ents, nil
			}
			if entry.Err != nil {
				id.directory.err = entry.Err
				return nil, entry.Err
			}

			// we consumed an entry
			id.directory.cursor++

			// skip processing the entry if its below the request offset
			if offset > id.directory.cursor {
				continue
			}
			nineEnt, err := coreEntTo9Ent(entry)
			if err != nil {
				id.directory.err = err
				return nil, err
			}
			nineEnt.Offset = id.directory.cursor
			ents = append(ents, nineEnt)

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
	//id.Logger.Debugf(readAtFmt, offset, id.meta.Size, id.String())

	if id.file == nil {
		err := fmt.Errorf("file is not open for reading")
		id.Logger.Errorf(readAtFmtErr, offset, id.meta.Size, id.String(), err)
		return 0, err
	}

	if offset >= id.meta.Size {
		//NOTE [styx]: If the offset field is greater than or equal to the number of bytes in the file, a count of zero will be returned.
		return 0, io.EOF
	}

	if _, err := id.file.Seek(int64(offset), io.SeekStart); err != nil {
		//id.operationsCancel()
		id.Logger.Errorf(readAtFmtErr, offset, id.meta.Size, id.String(), err)
		return 0, err
	}

	readBytes, err := id.file.Read(p)
	if err != nil && err != io.EOF {
		id.Logger.Errorf(readAtFmtErr, offset, id.meta.Size, id.String(), err)
		//id.operationsCancel()
		return 0, err
	}

	return readBytes, err
}

func (id *IPFS) Close() error {
	var lastErr error

	//TODO: timeout and cancel the context if Close takes too long
	if id.file != nil {
		if err := id.file.Close(); err != nil {
			id.Logger.Error(err)
			lastErr = err
		}
		id.file = nil
	}
	id.directory = nil

	lastErr = id.IPFSBase.Close()
	if lastErr != nil {
		id.Logger.Error(lastErr)
	}

	return lastErr
}

// IPFS appends "name" to its current path, and returns itself
func (id *IPFS) Step(name string) (walkRef, error) {
	return id.step(id, name)
}

func (id *IPFS) QID() (p9.QID, error) {
	if id.modified {
		callCtx, cancel := id.callCtx()
		defer cancel()

		qid, err := coreToQid(callCtx, id.CorePath(), id.core)
		if err != nil {
			return qid, err
		}
		id.modified = false
		id.Qid = &qid
	}

	return *id.Qid, nil
}

func (id *IPFS) Backtrack() (walkRef, error) {
	return id.IPFSBase.backtrack(id)
}
