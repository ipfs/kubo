package fsnodes

import (
	"context"
	"fmt"

	"github.com/djdv/p9/p9"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

type entPair struct {
	ent p9.Dirent
	key coreiface.Key
}

type KeyFS struct {
	IPFSBase
	keyEnts []entPair
}

func KeyFSAttacher(ctx context.Context, core coreiface.CoreAPI) *KeyFS {
	kd := &KeyFS{IPFSBase: newIPFSBase(ctx, rootPath("/ipns"), p9.TypeDir,
		core, logging.Logger("KeyFS"))}
	kd.meta, kd.metaMask = defaultRootAttr()
	kd.meta.Mode |= 0220
	return kd
}

func (kd *KeyFS) Attach() (p9.File, error) {
	kd.Logger.Debugf("Attach")

	var subSystem walkRef = IPFSAttacher(kd.Ctx, kd.core)
	attacher, ok := subSystem.(p9.Attacher)
	if !ok {
		return nil, fmt.Errorf("subsystem %T is not a valid file system", subSystem)
	}

	if _, err := attacher.Attach(); err != nil {
		return nil, err
	}
	kd.child = subSystem

	return kd, nil
}

func (kd *KeyFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	kd.Logger.Debugf("GetAttr")

	return kd.Qid, kd.metaMask, kd.meta, nil
}

func (kd *KeyFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	kd.Logger.Debugf("Walk names %v", names)
	kd.Logger.Debugf("Walk myself: %v", kd.Qid)

	if shouldClone(names) {
		kd.Logger.Debugf("Walk cloned")
		return []p9.QID{kd.Qid}, kd, nil
	}

	keys, err := kd.core.Key().List(kd.Ctx)
	if err != nil {
		return nil, nil, err
	}

	var coreKey coreiface.Key
	for _, key := range keys {
		if names[0] == key.Name() {
			coreKey = key
			break
		}
	}

	if coreKey != nil { // use IPNS FS to allow write support to the key
		ipns, err := IPNSAttacher(kd.Ctx, kd.core, coreKey).Attach()
		if err != nil {
			return nil, nil, err
		}
		return ipns.Walk(names)
	}

	// if we don't own the key, treat this as a typical IPFS core-request
	return walker(kd, names)
}

func (kd *KeyFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	kd.Logger.Debugf("Open")

	handleContext, cancel := context.WithCancel(kd.Ctx)
	kd.cancel = cancel

	var err error
	if kd.keyEnts, err = getKeys(handleContext, kd.core); err != nil {
		cancel()
		return kd.Qid, 0, err
	}

	return kd.Qid, ipfsBlockSize, nil
}

func (kd *KeyFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	kd.Logger.Debugf("Readdir")

	if kd.keyEnts == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", kd.Path.String())
	}

	shouldExit, err := boundCheck(offset, len(kd.keyEnts))
	if shouldExit {
		return nil, err
	}

	subSlice := kd.keyEnts[offset:]
	if len(subSlice) > int(count) {
		subSlice = subSlice[:count]
	}

	nineEnts := make([]p9.Dirent, 0, len(subSlice))
	for _, pair := range subSlice {
		nineEnts = append(nineEnts, pair.ent)
	}

	kd.Logger.Debugf("Readdir returning ents: %v", nineEnts)
	return nineEnts, nil
}
