package fsnodes

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/hugelgupf/p9/p9"
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
	id := &IPFS{
		IPFSBase: IPFSBase{
			core: core,
			Base: Base{
				Logger: logger,
				Ctx:    ctx,
				Qid: p9.QID{
					Type:    p9.TypeDir,
					Version: 1,
					Path:    uint64(pIPFSRoot)}}}}
	return id
}

func (id *IPFS) Attach() (p9.File, error) {
	id.Logger.Debugf("ID Attach")
	//TODO: check core connection here
	return id, nil
}

func (id *IPFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	id.Logger.Debugf("ID GetAttr")
	qid := p9.QID{
		Type:    p9.TypeDir,
		Version: 1}

	var attr p9.Attr
	var attrMask p9.AttrMask

	//TODO: make this impossible; initalize a valid CID for roots
	if id.Path != nil {
		qid.Path = cidToQPath(id.Path.Cid())
		if err := coreGetAttr(id.Ctx, &attr, &attrMask, id.core, id.Path); err != nil {
			return p9.QID{}, p9.AttrMask{}, p9.Attr{}, err
		}
	} else {
		qid.Path = uint64(pIPFSRoot)
		attr.Mode = p9.ModeDirectory | p9.Read | p9.Exec
		attr.RDev, attrMask.RDev = dIPFS, true
	}

	return qid, attrMask, attr, nil
}

func (id *IPFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	id.Logger.Debugf("ID Walk names %v", names)
	id.Logger.Debugf("ID Walk myself: %v", id.Qid)

	if doClone(names) {
		id.Logger.Debugf("ID Walk cloned")
		return []p9.QID{id.Qid}, id, nil
	}

	var ipfsPath corepath.Path
	var resolvedPath corepath.Resolved
	var err error
	qids := make([]p9.QID, 0, len(names))

	for _, name := range names {
		//TODO: convert this into however Go deals with do;while
		if ipfsPath == nil {
			ipfsPath = corepath.New("/ipfs/" + name)
		} else {
			ipfsPath = corepath.Join(ipfsPath, name)
		}
		//TODO: timerctx; don't want to hang forever
		resolvedPath, err = id.core.ResolvePath(id.Ctx, ipfsPath)
		if err != nil {
			return nil, nil, err
		}

		//XXX: generate QID more directly
		dirEnt := &p9.Dirent{}
		if err = coreStat(id.Ctx, dirEnt, id.core, ipfsPath); err != nil {
			return nil, nil, err
		}

		qids = append(qids, dirEnt.QID)
	}

	id.Path = resolvedPath

	id.Logger.Debugf("ID Walk reg ret %v, %v", qids, id)
	return qids, id, nil
}

func (id *IPFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	id.Logger.Debugf("ID Open")
	return id.Qid, 0, nil
}

func (id *IPFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	id.Logger.Debugf("ID Readdir")

	entChan, err := coreLs(id.Ctx, id.Path, id.core)
	if err != nil {
		return nil, err
	}

	var ents []p9.Dirent

	var off uint64 = 0
	for ent := range entChan {
		id.Logger.Debugf("got ent: %v\n", ent)
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

	id.Logger.Debugf("ID Readdir returning ents:%v", ents)
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
			return replaceMe, errors.New("read offset extends past end of file")
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
