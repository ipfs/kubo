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

func PinFSAttacher(ctx context.Context, core coreiface.CoreAPI, parent walkRef) *PinFS {
	pd := &PinFS{IPFSBase: newIPFSBase(ctx, rootPath("/ipfs"), p9.TypeDir,
		core, logging.Logger("PinFS"))}
	pd.meta, pd.metaMask = defaultRootAttr()
	if parent != nil {
		pd.parent = parent
	} else {
		pd.parent = pd
	}
	return pd
}

func (pd *PinFS) Attach() (p9.File, error) {
	pd.Logger.Debugf("Attach")
	_, err := pd.Base.Attach()
	if err != nil {
		return nil, err
	}

	subSystem, err := IPFSAttacher(pd.filesystemCtx, pd.core, pd).Attach()
	if err != nil {
		return nil, fmt.Errorf("could not attach to subsystem: %s", err)
	}

	walkRef, ok := subSystem.(walkRef)
	if !ok {
		return nil, fmt.Errorf("%q does not provide traverals methods", "ipfs")
	}

	pd.child = walkRef

	return pd, nil
}

func (pd *PinFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	pd.Logger.Debugf("GetAttr")

	return pd.Qid, pd.metaMask, pd.meta, nil
}

func (pd *PinFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	pd.Logger.Debugf("Walk names %v", names)
	pd.Logger.Debugf("Walk myself: %v", pd.Qid)

	return walker(pd, names)
}

func (pd *PinFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	pd.Logger.Debugf("Open")

	var handleContext context.Context
	handleContext, pd.operationsCancel = context.WithCancel(pd.filesystemCtx)

	// IPFS core representation
	pins, err := pd.core.Pin().Ls(handleContext, coreoptions.Pin.Type.Recursive())
	if err != nil {
		pd.operationsCancel()
		return pd.Qid, 0, err
	}

	// 9P representation
	pd.ents = make([]p9.Dirent, 0, len(pins))

	// temporary conversion storage
	attr := &p9.Attr{}
	requestType := p9.AttrMask{Mode: true}

	// actual conversion
	for i, pin := range pins {
		ipldNode, err := pd.core.ResolveNode(handleContext, pin.Path())
		if err != nil {
			pd.operationsCancel()
			return pd.Qid, 0, err
		}
		if err, _ = ipldStat(handleContext, attr, ipldNode, requestType); err != nil {
			pd.operationsCancel()
			return pd.Qid, 0, err
		}

		pd.ents = append(pd.ents, p9.Dirent{
			Name:   gopath.Base(pin.Path().String()),
			Offset: uint64(i + 1),
			QID: p9.QID{
				Type: attr.Mode.QIDType(),
				Path: cidToQPath(ipldNode.Cid()),
			},
		})
	}

	return pd.Qid, ipfsBlockSize, nil
}

func (pd *PinFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	pd.Logger.Debugf("Readdir")

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

	pd.Logger.Debugf("Readdir returning ents: %v", subSlice)
	return subSlice, nil
}
