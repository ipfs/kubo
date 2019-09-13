package fsnodes

import (
	"context"
	"fmt"
	gopath "path"

	"github.com/djdv/p9/p9"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
)

type PinFS struct {
	IPFSBase
	directory *directoryStorage
}

type directoryStorage struct {
	ents   []p9.Dirent
	cursor uint64
}

//TODO: [review] check fields
func PinFSAttacher(ctx context.Context, core coreiface.CoreAPI) *PinFS {
	pd := &PinFS{IPFSBase: newIPFSBase(ctx, newRootPath("/ipfs"), p9.TypeDir,
		core, logging.Logger("PinFS"))}
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

	if shouldClone(names) {
		pd.Logger.Debugf("PD Walk cloned")
		return []p9.QID{pd.Qid}, pd, nil
	}

	ipfsDir, err := IPFSAttacher(pd.Ctx, pd.core).Attach()
	if err != nil {
		return nil, nil, err
	}

	return ipfsDir.Walk(names)
}

func (pd *PinFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	pd.Logger.Debugf("PD Open")

	// IPFS core representation
	pins, err := pd.core.Pin().Ls(pd.Ctx, coreoptions.Pin.Type.Recursive())
	if err != nil {
		return pd.Qid, ipfsBlockSize, err
	}

	// 9P representation
	pd.directory = &directoryStorage{
		ents: make([]p9.Dirent, 0, len(pins)),
	}

	// actual conversion
	for i, pin := range pins {
		dirEnt := &p9.Dirent{
			Name:   gopath.Base(pin.Path().String()),
			Offset: uint64(i + 1),
		}

		if err = coreStat(pd.Ctx, dirEnt, pd.core, pin.Path()); err != nil {
			return pd.Qid, 0, err
		}
		pd.directory.ents = append(pd.directory.ents, *dirEnt)
	}

	return pd.Qid, ipfsBlockSize, nil
}

func (pd *PinFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	pd.Logger.Debugf("PD Readdir")

	if pd.directory == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", pd.Path.String())
	}

	if offset < 0 {
		return nil, fmt.Errorf("offset %d can't be negative", offset)
	}

	if entCount := uint64(len(pd.directory.ents)); offset > entCount {
		return nil, fmt.Errorf("offset %d extends beyond directory bound %d", offset, entCount)
	}

	subSlice := pd.directory.ents[offset:]
	if len(subSlice) > int(count) {
		subSlice = subSlice[:count]
	}

	pd.Logger.Debugf("PD Readdir returning ents: %v", subSlice)
	return subSlice, nil
}
