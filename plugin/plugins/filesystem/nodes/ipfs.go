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
	directory *directoryStream
}

type directoryStream struct {
	entryChan <-chan coreiface.DirEntry
	cursor    uint64
}

func IPFSAttacher(ctx context.Context, core coreiface.CoreAPI) *IPFS {
	id := &IPFS{IPFSBase: newIPFSBase(ctx, newRootPath("/ipfs"), p9.TypeDir,
		core, logging.Logger("IPFS"))}
	id.meta, id.metaMask = defaultRootAttr()
	return id
}

func (id *IPFS) Attach() (p9.File, error) {
	id.Logger.Debugf("ID Attach")
	//TODO: check core connection here
	return id, nil
}

func (id *IPFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	id.Logger.Debugf("ID GetAttr")
	id.Logger.Debugf("ID GetAttr path: %v", id.Path)

	if id.Path.Namespace() == nRoot {
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
	id.Logger.Debugf("ID Walk names %v", names)
	id.Logger.Debugf("ID Walk myself: %s:%v", id.Path, id.Qid)

	if shouldClone(names) {
		id.Logger.Debugf("ID Walk cloned")
		return []p9.QID{id.Qid}, id, nil
	}

	qids := make([]p9.QID, 0, len(names))

	//TODO: [spec check] make sure we're not messing things up by doing this instead of mutating
	// ^ Does internal library expect fid to mutate on success or does newfid clobber some external state anyway
	walkedNode := &IPFS{} // operate on a copy
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

	id.Logger.Debugf("ID Walk ret %v, %v", qids, walkedNode)
	return qids, walkedNode, err
}

func (id *IPFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	id.Logger.Debugf("ID Open")
	if id.meta.Mode.IsDir() {
		c, err := id.core.Unixfs().Ls(id.Ctx, id.Path)
		if err != nil {
			return id.Qid, 0, err
		}
		id.directory = &directoryStream{
			entryChan: c,
		}
	}

	return id.Qid, 0, nil
}

func (id *IPFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	id.Logger.Debugf("ID Readdir")

	if id.directory == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", id.Path.String())
	}

	if count == 0 {
		return nil, nil
	}

	if offset < id.directory.cursor {
		return nil, fmt.Errorf("read offset %d is behind current entry %d, seeking backwards in directory streams is not supported", offset, id.directory.cursor)
	}

	ents := make([]p9.Dirent, 0)

out:
	for {
		select {
		case entry, open := <-id.directory.entryChan:
			if !open {
				break out
			}
			if entry.Err != nil {
				return nil, entry.Err
			}

			if offset <= id.directory.cursor {
				nineEnt, err := coreEntTo9Ent(entry)
				if err != nil {
					return nil, err
				}
				nineEnt.Offset = id.directory.cursor
				ents = append(ents, nineEnt)
				count--
			}

			id.directory.cursor++

			if count == 0 {
				break out
			}
		case <-id.Ctx.Done():
			return ents, id.Ctx.Err()
		}
	}

	if offset > uint64(len(ents)) {
		return nil, nil //cursor is at end of stream, nothing to return
	}

	id.Logger.Debugf("ID Readdir returning [%d]%v\n", len(ents), ents)
	return ents, nil
}

func (id *IPFS) ReadAt(p []byte, offset uint64) (int, error) {
	id.Logger.Debugf("ID ReadAt")
	const replaceMe = -1 //TODO: proper error codes

	apiNode, err := id.core.Unixfs().Get(id.Ctx, id.Path)
	if err != nil {
		return replaceMe, err
	}

	fIo, ok := apiNode.(files.File)
	if !ok {
		return replaceMe, fmt.Errorf("%q is not a file", id.Path.String())
	}

	if fileBound, err := fIo.Size(); err == nil {
		if int64(offset) >= fileBound {
			//NOTE [styx]: If the offset field is greater than or equal to the number of bytes in the file, a count of zero will be returned.
			return 0, nil
		}
	}

	if offset != 0 {
		_, err = fIo.Seek(int64(offset), io.SeekStart)
		if err != nil {
			return replaceMe, fmt.Errorf("Read - seek error: %s", err)
		}
	}

	readBytes, err := fIo.Read(p)
	if err != nil && err != io.EOF {
		return replaceMe, err
	}
	return readBytes, nil
}
