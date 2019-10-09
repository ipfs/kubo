package fsnodes

import (
	"context"
	"fmt"
	gopath "path"

	"github.com/djdv/p9/p9"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
)

var _ p9.File = (*PinFS)(nil)
var _ walkRef = (*PinFS)(nil)

type PinFS struct {
	IPFSBase
	ents []p9.Dirent
}

func PinFSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	pd := &PinFS{IPFSBase: newIPFSBase(ctx, "/pinfs", core, ops...)}
	pd.Qid.Type = p9.TypeDir
	pd.meta.Mode, pd.metaMask.Mode = p9.ModeDirectory|IRXA, true

	// set up our subsystem, used to relay walk names to IPFS
	subOpts := []nodeopts.AttachOption{
		nodeopts.Parent(pd),
		nodeopts.Logger(logging.Logger("IPFS")),
	}

	subsystem, err := IPFSAttacher(ctx, core, subOpts...).Attach()
	if err != nil {
		panic(err)
	}

	pd.proxy = subsystem.(walkRef)

	return pd
}

func (pd *PinFS) Derive() walkRef {
	newFid := &PinFS{
		IPFSBase: pd.IPFSBase.Derive(),
	}
	return newFid
}

func (pd *PinFS) Attach() (p9.File, error) {
	pd.Logger.Debugf("Attach")
	return pd, nil
}

// PinFS proxies steps to the IPFS root that was set during construction
func (pd *PinFS) Step(name string) (walkRef, error) {
	// derive copy of IPFS root
	p := pd.proxy.Derive()
	// proxy the request for "name"
	return p.Step(name)
}

func (pd *PinFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	pd.Logger.Debugf("Walk names %v", names)
	pd.Logger.Debugf("Walk myself: %v", pd.Qid)

	return walker(pd, names)
}

func (pd *PinFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	pd.Logger.Debugf("Open")

	// IPFS core representation
	pins, err := pd.core.Pin().Ls(pd.filesystemCtx, coreoptions.Pin.Type.Recursive())
	if err != nil {
		return pd.Qid, 0, err
	}

	// 9P representation
	pd.ents = make([]p9.Dirent, 0, len(pins))

	// temporary conversion storage
	attr := &p9.Attr{}
	requestType := p9.AttrMask{Mode: true}

	// actual conversion
	for i, pin := range pins {
		callCtx, cancel := pd.callCtx()
		ipldNode, err := pd.core.ResolveNode(callCtx, pin.Path())
		if err != nil {
			cancel()
			return pd.Qid, 0, err
		}
		if _, err = ipldStat(callCtx, attr, ipldNode, requestType); err != nil {
			cancel()
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
		cancel()
	}

	return pd.Qid, ipfsBlockSize, nil
}

func (pd *PinFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	pd.Logger.Debugf("Readdir")

	if pd.ents == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", pd.StringPath())
	}

	return flatReaddir(pd.ents, offset, count)
}

func (pd *PinFS) Backtrack() (walkRef, error) {
	// return the parent if we are the root
	if len(pd.Trail) == 0 {
		return pd.parent, nil
	}

	// otherwise step back
	pd.Trail = pd.Trail[1:]

	// reset meta
	return pd, nil
}

func (pd *PinFS) Flush() error {
	pd.Logger.Errorf("flushing:%q:%d", pd.StringPath(), pd.NinePath())
	return nil
}

func (pd *PinFS) Close() error {
	pd.ents = nil
	return nil
}
