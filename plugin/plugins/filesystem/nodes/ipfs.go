package fsnodes

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/djdv/p9/p9"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

// IPFS exposes the IPFS API over a p9.File interface
// Walk does not expect a namespace, only its path argument
// e.g. `ipfs.Walk([]string("Qm...", "subdir")` not `ipfs.Walk([]string("ipfs", "Qm...", "subdir")`
type IPFS struct {
	IPFSBase
}

func IPFSAttacher(ctx context.Context, core coreiface.CoreAPI) *IPFS {
	id := &IPFS{IPFSBase: newIPFSBase(ctx, rootPath("/ipfs"), p9.TypeDir,
		core, logging.Logger("IPFS"))}
	id.meta, id.metaMask = defaultRootAttr()
	return id
}

func (id *IPFS) Attach() (p9.File, error) {
	id.Logger.Debugf("Attach")
	return id, nil
}

func (id *IPFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	id.Logger.Debugf("GetAttr path: %v", id.Path)

	// For IPFS, we set this up front in Walk
	return id.Qid, id.metaMask, id.meta, nil
}

func (id *IPFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	id.Logger.Debugf("Walk names %v", names)
	id.Logger.Debugf("Walk myself: %s:%v", id.Path, id.Qid)

	if shouldClone(names) {
		id.Logger.Debugf("Walk cloned")
		return []p9.QID{id.Qid}, id, nil
	}

	var (
		// returned
		qids   = make([]p9.QID, 0, len(names))
		newFid = &IPFS{IPFSBase: newIPFSBase(id.Ctx, id.Path, 0, id.core, id.Logger)}
		// temporary
		attr        = &p9.Attr{}
		requestType = p9.AttrMask{Mode: true}
		err         error
		// last-set value is used
		ipldNode ipld.Node
	)

	for _, name := range names {
		callCtx, cancel := context.WithTimeout(id.Ctx, 30*time.Second)
		defer cancel()

		corePath, err := id.core.ResolvePath(callCtx, corepath.Join(newFid.Path, name))
		if err != nil {
			cancel()
			//TODO: switch off error, return appropriate errno (ENOENT is going to be common here)
			// ref: https://github.com/hugelgupf/p9/pull/12#discussion_r324991695
			return qids, nil, err
		}

		newFid.Path = corePath

		ipldNode, err = id.core.Dag().Get(callCtx, newFid.Path.Cid())
		if err != nil {
			cancel()
			return qids, nil, err
		}

		err, _ = ipldStat(callCtx, attr, ipldNode, requestType)
		if err != nil {
			cancel()
			return qids, nil, err
		}

		newFid.Qid.Type = attr.Mode.QIDType()
		newFid.Qid.Path = cidToQPath(ipldNode.Cid())
		qids = append(qids, newFid.Qid)
	}

	// stat up front for our returned file
	err, filled := ipldStat(id.Ctx, &newFid.meta, ipldNode, p9.AttrMaskAll)
	if err != nil {
		return qids, nil, err
	}
	newFid.metaMask = filled
	newFid.meta.Mode |= IRXA
	newFid.meta.RDev, id.metaMask.RDev = dIPFS, true

	id.Logger.Debugf("Walk ret %v, %v", qids, newFid)
	return qids, newFid, err
}

func (id *IPFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	id.Logger.Debugf("Open %q", id.Path)

	// set up  handle amenities
	var handleContext context.Context
	handleContext, id.operationsCancel = context.WithCancel(id.Ctx)

	// handle directories
	if id.meta.Mode.IsDir() { //FIXME: meta is not being set anywhere
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
