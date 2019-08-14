package fsnodes

import (
	"context"
	gopath "path"

	"github.com/hugelgupf/p9/p9"
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
			core: core,
			Base: Base{
				Logger: logger,
				Ctx:    ctx,
				Qid: p9.QID{
					Type:    p9.TypeDir,
					Version: 1,
					Path:    uint64(pPinRoot)}}}}
	return pd
}

func (pd *PinFS) Attach() (p9.File, error) {
	pd.Logger.Debugf("PD Attach")
	//TODO: check core connection here
	return pd, nil
}

func (pd *PinFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	pd.Logger.Debugf("PD GetAttr")
	qid := p9.QID{
		Type:    p9.TypeDir,
		Version: 1,
		Path:    uint64(pPinRoot),
	}

	//TODO: [metadata] quick hack; revise
	attr := p9.Attr{
		Mode: p9.ModeDirectory,
		RDev: dMemory,
	}

	attrMask := p9.AttrMask{
		Mode: true,
		RDev: true,
	}

	return qid, attrMask, attr, nil
}

func (pd *PinFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	pd.Logger.Debugf("PD Walk names %v", names)
	pd.Logger.Debugf("PD Walk myself: %v", pd.Qid)

	if doClone(names) {
		pd.Logger.Debugf("PD Walk cloned")
		return []p9.QID{pd.Qid}, pd, nil
	}

//NOTE: if doClone is false, it implies len(names) > 0
	var tailFile p9.File = pd
	var subQids []p9.QID
	qids := make([]p9.QID, 0, len(names))

		ipfsDir, err := initIPFS(pd.Ctx, pd.core, pd.Logger).Attach()
		if err != nil {
			return nil,nil,err
		}
		if subQids, tailFile, err = ipfsDir.Walk(names); err != nil {
			return nil, nil, err
		}
		qids = append(qids, subQids...)
	pd.Logger.Debugf("PD Walk reg ret %v, %v", qids, tailFile)
	return qids, tailFile, nil
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

	if count < uint32(len(pins)) {
		pins = pins[:count]
	}
	ents := make([]p9.Dirent, 0, len(pins))

	for i, pin := range pins {
		dirEnt := &p9.Dirent{
			Name:   gopath.Base(pin.Path().String()),
			Offset: uint64(i),
		}

		if err = coreStat(pd.Ctx, dirEnt, pd.core, pin.Path()); err != nil {
			return nil, err
		}
		ents = append(ents, *dirEnt)
	}

	pd.Logger.Debugf("PD Readdir returning ents:%v", ents)
	return ents, err
}
