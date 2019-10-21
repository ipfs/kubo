package fsnodes

import (
	"context"

	"github.com/hugelgupf/p9/p9"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	fsutils "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/utils"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

var _ p9.File = (*KeyFS)(nil)
var _ fsutils.WalkRef = (*KeyFS)(nil)

type KeyFS struct {
	IPFSBase
}

func KeyFSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	kd := &KeyFS{IPFSBase: newIPFSBase(ctx, "/keyfs", core, ops...)}
	kd.qid.Type = p9.TypeDir
	kd.meta.Mode, kd.metaMask.Mode = p9.ModeDirectory|IRXA|0220, true

	// non-keyed requests fall through to IPNS
	opts := []nodeopts.AttachOption{
		nodeopts.Parent(kd),
		nodeopts.Logger(logging.Logger("IPNS")),
	}

	subsystem, err := IPNSAttacher(ctx, core, opts...).Attach()
	if err != nil {
		panic(err)
	}

	kd.proxy = subsystem.(fsutils.WalkRef)

	return kd
}

func (kd *KeyFS) Fork() (fsutils.WalkRef, error) {
	newFid := &KeyFS{IPFSBase: kd.IPFSBase.clone()} // root has no paths to walk; don't set node up for change
	// set new operations context
	err := newFid.newOperations()
	return newFid, err
}

func (kd *KeyFS) Attach() (p9.File, error) {
	kd.Logger.Debugf("Attach")

	newFid := &KeyFS{IPFSBase: kd.IPFSBase.clone()} // root has no paths to walk; don't set node up for change
	// set new fs context
	err := newFid.newFilesystem()
	return newFid, err
}

// KeyFS forks the IPFS root that was set during construction
// and calls step on it rather than itself
func (kd *KeyFS) Step(name string) (fsutils.WalkRef, error) {
	newFid, err := kd.proxy.Fork()
	if err != nil {
		return nil, err
	}
	return newFid.Step(name)
}

func (kd *KeyFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	kd.Logger.Debugf("Walk names %v", names)
	kd.Logger.Debugf("Walk myself: %v", kd.qid)

	return fsutils.Walker(kd, names)
}

func (kd *KeyFS) Backtrack() (fsutils.WalkRef, error) {
	return kd.IPFSBase.backtrack(kd)
}

// temporary stub to allow forwarding requests on empty directory
func (kd *KeyFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	return nil, nil
}
