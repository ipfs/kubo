package fsnodes

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/djdv/p9/p9"
	files "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

// IPFS exposes the IPFS API over a p9.File interface
// Walk does not expect a namespace, only its path argument
// e.g. `ipfs.Walk([]string("Qm...", "subdir")` not `ipfs.Walk([]string("ipfs", "Qm...", "subdir")`
type IPFS struct {
	IPFSBase
	ipfsHandle
}

func IPFSAttacher(ctx context.Context, core coreiface.CoreAPI) *IPFS {
	id := &IPFS{IPFSBase: newIPFSBase(ctx, rootPath("/ipfs"), p9.TypeDir,
		core, logging.Logger("IPFS"))}
	id.meta, id.metaMask = defaultRootAttr()
	return id
}

func (id *IPFS) Attach() (p9.File, error) {
	id.Logger.Debugf("Attach")
	//TODO: check core connection here
	return id, nil
}

func (id *IPFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	id.Logger.Debugf("GetAttr")
	id.Logger.Debugf("GetAttr path: %v", id.Path)

	if id.Path.Namespace() == nRoot { // metadata should have been initialized by attacher, don't consult CoreAPI
		return id.Qid, id.metaMask, id.meta, nil
	}

	if err := coreGetAttr(id.Ctx, &id.meta, req, id.core, id.Path); err != nil {
		return p9.QID{}, p9.AttrMask{}, p9.Attr{}, err
	}
	id.Qid.Type = id.meta.Mode.QIDType()

	metaClone := id.meta
	metaClone.Filter(req)

	return id.Qid, req, metaClone, nil
}

func (id *IPFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	id.Logger.Debugf("Walk names %v", names)
	id.Logger.Debugf("Walk myself: %s:%v", id.Path, id.Qid)

	if shouldClone(names) {
		id.Logger.Debugf("Walk cloned")
		return []p9.QID{id.Qid}, id, nil
	}

	var (
		qids   = make([]p9.QID, 0, len(names))
		newFid = &IPFS{IPFSBase: newIPFSBase(id.Ctx, id.Path, 0, id.core, id.Logger)}
		err    error
	)

	for _, name := range names {
		callCtx, cancel := context.WithTimeout(id.Ctx, 30*time.Second)
		defer cancel()

		if newFid.Path, err = id.core.ResolvePath(callCtx, corepath.Join(newFid.Path, name)); err != nil {
			cancel()
			//TODO: switch off error, return appropriate errno (likely ENOENT)
			return qids, nil, err
		}

		ipldNode, err := id.core.Dag().Get(callCtx, newFid.Path.Cid())
		if err != nil {
			cancel()
			return qids, nil, err
		}

		//TODO: this is too opaque; we want core path => qid, Dirent isn't /really/ necessary
		dirEnt := &p9.Dirent{}
		if err = ipldStat(dirEnt, ipldNode); err != nil {
			cancel()
			return qids, nil, err
		}

		newFid.Qid = dirEnt.QID
		//
		qids = append(qids, newFid.Qid)
	}

	id.Logger.Debugf("Walk ret %v, %v", qids, newFid)
	return qids, newFid, err
}

func (id *IPFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	id.Logger.Debugf("Open %q", id.Path)

	// set up  handle amenities
	var handleContext context.Context
	handleContext, id.operationsCancel = context.WithCancel(id.Ctx)

	// handle directories
	if id.meta.Mode.IsDir() {
		c, err := id.core.Unixfs().Ls(handleContext, id.Path)
		if err != nil {
			id.operationsCancel()
			return id.Qid, 0, err
		}

		id.directory = &directoryStream{
			entryChan: c,
		}
		return id.Qid, 0, nil
	}

	// handle files
	apiNode, err := id.core.Unixfs().Get(handleContext, id.Path)
	if err != nil {
		id.operationsCancel()
		return id.Qid, 0, err
	}

	fileNode, ok := apiNode.(files.File)
	if !ok {
		id.operationsCancel()
		return id.Qid, 0, fmt.Errorf("%q does not appear to be a file: %T", id.Path.String(), apiNode)
	}
	id.file = fileNode

	return id.Qid, ipfsBlockSize, nil
}

func (id *IPFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	id.Logger.Debugf("Readdir %d %d", offset, count)

	if id.directory == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", id.Path.String())
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
				id.operationsCancel()
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

		case <-id.Ctx.Done():
			id.directory.err = id.Ctx.Err()
			return ents, id.directory.err
		}
	}

	id.Logger.Debugf("Readdir returning [%d]%v\n", len(ents), ents)
	return ents, nil
}

func (id *IPFS) ReadAt(p []byte, offset uint64) (int, error) {
	id.Logger.Debugf("ReadAt")

	if id.file == nil {
		return -1, fmt.Errorf("file %q is not open for reading", id.Path.String())
	}

	if fileBound, err := id.file.Size(); err == nil {
		if int64(offset) >= fileBound {
			//NOTE [styx]: If the offset field is greater than or equal to the number of bytes in the file, a count of zero will be returned.
			return 0, io.EOF
		}
	}

	if offset != 0 {
		if _, err := id.file.Seek(int64(offset), io.SeekStart); err != nil {
			return -1, fmt.Errorf("Read - seek error: %s", err)
		}
	}

	readBytes, err := id.file.Read(p)
	if err != nil && err != io.EOF {
		return -1, err
	}
	return readBytes, err
}
