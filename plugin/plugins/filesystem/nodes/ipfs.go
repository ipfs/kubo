package fsnodes

import (
	"context"
	"fmt"
	"io"

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

	qids := make([]p9.QID, 0, len(names))

	walkedNode := &IPFS{} // [walk(5)] id represents fid, walkedNode represents newfid
	*walkedNode = *id

	var err error
	for _, name := range names {
		//TODO: timerctx; don't want to hang forever
		if walkedNode.Path, err = id.core.ResolvePath(id.Ctx, corepath.Join(walkedNode.Path, name)); err != nil {
			return qids, nil, err
		}

		ipldNode, err := id.core.Dag().Get(id.Ctx, walkedNode.Path.Cid())
		if err != nil {
			return qids, nil, err
		}

		//TODO: this is too opague; we want core path => qid, dirent isn't necessary
		dirEnt := &p9.Dirent{}
		if err = ipldStat(dirEnt, ipldNode); err != nil {
			return qids, nil, err
		}

		walkedNode.Qid = dirEnt.QID
		//
		qids = append(qids, walkedNode.Qid)
	}

	id.Logger.Debugf("Walk ret %v, %v", qids, walkedNode)
	return qids, walkedNode, err
}

func (id *IPFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	id.Logger.Debugf("Open %q", id.Path)

	// set up  handle amenities
	var handleContext context.Context
	handleContext, cancel := context.WithCancel(id.Ctx)
	id.cancel = cancel

	// handle directories
	if id.meta.Mode.IsDir() {
		c, err := id.core.Unixfs().Ls(handleContext, id.Path)
		if err != nil {
			id.Logger.Errorf("hit\n%q\n%#v\n", id.Path, err)
			cancel()
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
		id.Logger.Errorf("hit\n%q\n%#v\n", id.Path, err)
		cancel()
		return id.Qid, 0, err
	}

	var ok bool
	if id.file, ok = apiNode.(files.File); !ok {
		cancel()
		return id.Qid, 0, fmt.Errorf("%q does not appear to be a file: %T", id.Path.String(), apiNode)
	}

	return id.Qid, ipfsBlockSize, nil
}

func (id *IPFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	id.Logger.Debugf("Readdir %d %d", offset, count)

	if id.directory == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", id.Path.String())
	}

	if offset < 0 {
		return nil, fmt.Errorf("offset %d can't be negative", offset)
	}

	if id.directory.eos {
		if offset == id.directory.cursor-1 {
			return nil, nil // valid end of stream request
		}
	}

	if id.directory.eos && offset == id.directory.cursor-1 {
		return nil, nil // valid end of stream request
	}

	if offset < id.directory.cursor {
		return nil, fmt.Errorf("read offset %d is behind current entry %d, seeking backwards in directory streams is not supported", offset, id.directory.cursor)
	}

	ents := make([]p9.Dirent, 0)
out:
	for len(ents) < int(count) {
		select {
		case entry, open := <-id.directory.entryChan:
			if !open {
				id.directory.eos = true
				break out
			}
			if entry.Err != nil {
				id.directory = nil
				return nil, entry.Err
			}

			if offset <= id.directory.cursor {
				nineEnt, err := coreEntTo9Ent(entry)
				if err != nil {
					id.directory = nil
					return nil, err
				}
				nineEnt.Offset = id.directory.cursor
				ents = append(ents, nineEnt)
			}

			id.directory.cursor++

		case <-id.Ctx.Done():
			id.directory = nil
			return ents, id.Ctx.Err()
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
