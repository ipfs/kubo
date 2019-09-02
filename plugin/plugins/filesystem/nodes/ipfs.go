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

type IPFS struct {
	IPFSBase
}

//TODO: [review] check fields; better wrappers around inheritance init, etc.
func initIPFS(ctx context.Context, core coreiface.CoreAPI, logger logging.EventLogger) p9.Attacher {
	id := &IPFS{IPFSBase: newIPFSBase(ctx, newRootPath("/ipfs"), p9.TypeDir, core, logger)}
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

	var attrMask p9.AttrMask
	if err := coreGetAttr(id.Ctx, &id.meta, &attrMask, id.core, id.Path); err != nil {
		return p9.QID{}, p9.AttrMask{}, p9.Attr{}, err
	}
	timeStamp(&id.meta, &attrMask)
	id.Qid.Type = id.meta.Mode.QIDType()

	return id.Qid, attrMask, id.meta, nil
}

func (id *IPFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	id.Logger.Debugf("ID Walk names %v", names)
	id.Logger.Debugf("ID Walk myself: %s:%v", id.Path, id.Qid)

	if doClone(names) {
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
	return id.Qid, 0, nil
}

func (id *IPFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	id.Logger.Debugf("ID Readdir")

	//FIXME: we read the entire directory for each readdir call; this is very wasteful with small requests (Unix `ls`)
	//TODO [quick hack]: append only entry list stored on id; likely going to be problematic for large directories (test: Wikipedia)
	entChan, err := coreLs(id.Ctx, id.Path, id.core)
	if err != nil {
		return nil, err
	}

	var ents []p9.Dirent

	var off uint64 = 1
	for ent := range entChan {
		if ent.Err != nil {
			return nil, err
		}
		off++

		nineEnt := coreEntTo9Ent(ent)
		nineEnt.Offset = off
		ents = append(ents, nineEnt)

		if uint32(len(ents)) == count {
			break
		}
	}

	//FIXME: I don't think order is gauranteed from Ls
	eLen := uint64(len(ents))
	if offset >= eLen {
		return nil, nil
	}

	offsetIndex := ents[offset:]
	if len(offsetIndex) > int(count) {
		id.Logger.Debugf("ID Readdir returning [%d]%v\n", count, offsetIndex[:count])
		return offsetIndex[:count], nil
	}

	id.Logger.Debugf("ID Readdir returning [%d]%v\n", len(offsetIndex), offsetIndex)
	return offsetIndex, nil
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
