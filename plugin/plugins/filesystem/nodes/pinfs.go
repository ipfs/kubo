package fsnodes

import (
	"context"
	gopath "path"

	"github.com/djdv/p9/p9"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
)

type PinFS struct {
	IPFSBase
}

//TODO: [review] check fields
func initPinFS(ctx context.Context, core coreiface.CoreAPI, logger logging.EventLogger) p9.Attacher {
	pd := &PinFS{
		IPFSBase: IPFSBase{
			Path: newRootPath("/ipfs"),
			core: core,
			Base: Base{
				Logger: logger,
				Ctx:    ctx,
				Qid:    p9.QID{Type: p9.TypeDir}}}}

	pd.Qid.Path = cidToQPath(pd.Path.Cid())
	pd.meta, pd.metaMask = defaultRootAttr()
	return pd
}

func (pd *PinFS) Attach() (p9.File, error) {
	pd.Logger.Debugf("PD Attach")
	//TODO: check core connection here
	return pd, nil
}

func (pd *PinFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	pd.Logger.Debugf("PD GetAttr")

	return pd.Qid, pd.metaMask, pd.meta, nil
}

func (pd *PinFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	pd.Logger.Debugf("PD Walk names %v", names)
	pd.Logger.Debugf("PD Walk myself: %v", pd.Qid)

	if doClone(names) {
		pd.Logger.Debugf("PD Walk cloned")
		return []p9.QID{pd.Qid}, pd, nil
	}

	ipfsDir, err := initIPFS(pd.Ctx, pd.core, pd.Logger).Attach()
	if err != nil {
		return nil, nil, err
	}

	return ipfsDir.Walk(names)
}

func (pd *PinFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	pd.Logger.Debugf("PD Open")
	return pd.Qid, 0, nil
}

func (pd *PinFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	pd.Logger.Debugf("PD Readdir")

	pins, err := pd.core.Pin().Ls(pd.Ctx, coreoptions.Pin.Type.Recursive())
	if err != nil {
		return nil, err
	}

	pLen := uint64(len(pins))
	if offset >= pLen {
		return nil, nil
	}

	//TODO: we should initialize and store entries during Open() to assure order is maintained through read calls
	offsetIndex := pins[offset:]
	if len(offsetIndex) > int(count) {
		offsetIndex = offsetIndex[:count]
	}

	ents := make([]p9.Dirent, 0, len(offsetIndex))
	for i, pin := range offsetIndex {
		dirEnt := &p9.Dirent{
			Name:   gopath.Base(pin.Path().String()),
			Offset: uint64(i + 1),
		}

		if err = coreStat(pd.Ctx, dirEnt, pd.core, pin.Path()); err != nil {
			return nil, err
		}
		ents = append(ents, *dirEnt)
	}

	pd.Logger.Debugf("PD Readdir returning ents:%v", ents)
	return ents, err
}
