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
	ents []p9.Dirent
}

func PinFSAttacher(ctx context.Context, core coreiface.CoreAPI) *PinFS {
	pd := &PinFS{IPFSBase: newIPFSBase(ctx, rootPath("/ipfs"), p9.TypeDir,
		core, logging.Logger("PinFS"))}
	pd.meta, pd.metaMask = defaultRootAttr()

	return pd
}

func (pd *PinFS) Attach() (p9.File, error) {
	pd.Logger.Debugf("PD Attach")

	var subSystem walkRef = IPFSAttacher(pd.Ctx, pd.core)
	attacher, ok := subSystem.(p9.Attacher)
	if !ok {
		return nil, fmt.Errorf("subsystem %T is not a valid file system", subSystem)
	}

	if _, err := attacher.Attach(); err != nil {
		return nil, err
	}
	pd.child = subSystem

	return pd, nil
}

func (pd *PinFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	pd.Logger.Debugf("PD GetAttr")

	return pd.Qid, pd.metaMask, pd.meta, nil
}

func (pd *PinFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	pd.Logger.Debugf("PD Walk names %v", names)
	pd.Logger.Debugf("PD Walk myself: %v", pd.Qid)

	return walker(pd, names)
}

func (pd *PinFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	pd.Logger.Debugf("PD Open")

	handleContext, cancel := context.WithCancel(pd.Ctx)
	pd.cancel = cancel

	// IPFS core representation
	pins, err := pd.core.Pin().Ls(handleContext, coreoptions.Pin.Type.Recursive())
	if err != nil {
		cancel()
		return pd.Qid, 0, err
	}

	// 9P representation
	pd.ents = make([]p9.Dirent, 0, len(pins))

	// actual conversion
	for i, pin := range pins {
		dirEnt := &p9.Dirent{
			Name:   gopath.Base(pin.Path().String()),
			Offset: uint64(i + 1),
		}

		if err = coreStat(handleContext, dirEnt, pd.core, pin.Path()); err != nil {
			cancel()
			return pd.Qid, 0, err
		}
		pd.ents = append(pd.ents, *dirEnt)
	}

	return pd.Qid, ipfsBlockSize, nil
}

func (pd *PinFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	pd.Logger.Debugf("PD Readdir")

	if pd.ents == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", pd.Path.String())
	}

	shouldExit, err := boundCheck(offset, len(pd.ents))
	if shouldExit {
		return nil, err
	}

	subSlice := pd.ents[offset:]
	if len(subSlice) > int(count) {
		subSlice = subSlice[:count]
	}

	pd.Logger.Debugf("PD Readdir returning ents: %v", subSlice)
	return subSlice, nil
}
