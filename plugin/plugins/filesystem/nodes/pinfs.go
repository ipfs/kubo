package fsnodes

import (
	"context"
	"fmt"
	gopath "path"

	"github.com/hugelgupf/p9/p9"
	"github.com/hugelgupf/p9/unimplfs"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	fsutils "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/utils"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
)

var _ p9.File = (*PinFS)(nil)
var _ fsutils.WalkRef = (*PinFS)(nil)

type PinFS struct {
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr

	IPFSBase
	ents []p9.Dirent
}

func PinFSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	pd := &PinFS{IPFSBase: newIPFSBase(ctx, "/pinfs", core, ops...)}
	pd.qid.Type = p9.TypeDir
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

	pd.proxy = subsystem.(fsutils.WalkRef)

	return pd
}

// this root has no paths to walk; forking anythign besides the fs doesn't make sense for us
func (pd *PinFS) clone() (fsutils.WalkRef, error) {
	newFid := &PinFS{IPFSBase: pd.IPFSBase.clone()}
	if err := newFid.forkFilesystem(); err != nil {
		return nil, err
	}
	return newFid, nil
}

func (pd *PinFS) Attach() (p9.File, error) {
	pd.Logger.Debugf("Attach")
	return pd.clone()
}

func (pd *PinFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	pd.Logger.Debugf("Open")

	qid := *pd.qid

	// IPFS core representation
	pins, err := pd.core.Pin().Ls(pd.operationsCtx, coreoptions.Pin.Type.Recursive())
	if err != nil {
		return qid, 0, err
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
			return qid, 0, err
		}
		if _, err = ipldStat(callCtx, attr, ipldNode, requestType); err != nil {
			cancel()
			return qid, 0, err
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

	return qid, ipfsBlockSize, nil
}

func (pd *PinFS) Close() error {
	pd.ents = nil
	return pd.IPFSBase.close()
}

func (pd *PinFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	pd.Logger.Debugf("Readdir")

	if pd.ents == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", pd.String())
	}

	return flatReaddir(pd.ents, offset, count)
}

/* WalkRef relevant */

func (pd *PinFS) Fork() (fsutils.WalkRef, error) {
	// root has no paths to walk; don't set node up for change
	return pd.clone()
}

// PinFS forks the IPFS root that was set during construction
// and calls step on it rather than itself
func (pd *PinFS) Step(name string) (fsutils.WalkRef, error) {
	newFid, err := pd.proxy.Fork()
	if err != nil {
		return nil, err
	}
	return newFid.Step(name)
}

func (pd *PinFS) CheckWalk() error                    { return pd.Base.checkWalk() }
func (pd *PinFS) QID() (p9.QID, error)                { return pd.Base.qID() }
func (pd *PinFS) Backtrack() (fsutils.WalkRef, error) { return pd.IPFSBase.backtrack(pd) }

/* base class boilerplate */

func (pd *PinFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	pd.Logger.Debugf("Walk names %v", names)
	pd.Logger.Debugf("Walk myself: %v", pd.qid)

	return fsutils.Walker(pd, names)
}

func (pd *PinFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	return pd.Base.getAttr(req)
}
