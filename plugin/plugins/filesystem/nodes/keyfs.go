package fsnodes

import (
	"context"
	"fmt"

	"github.com/djdv/p9/p9"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

type KeyFS struct {
	IPFSBase
	ents []p9.Dirent
}

func KeyFSAttacher(ctx context.Context, core coreiface.CoreAPI) *KeyFS {
	kd := &KeyFS{IPFSBase: newIPFSBase(ctx, rootPath("/ipns"), p9.TypeDir,
		core, logging.Logger("KeyFS"))}
	kd.meta, kd.metaMask = defaultRootAttr()
	kd.meta.Mode |= 0220
	return kd
}

func (kd *KeyFS) Attach() (p9.File, error) {
	kd.Logger.Debugf("KD Attach")
	//TODO: check core connection here
	return kd, nil
}

func (kd *KeyFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	kd.Logger.Debugf("KD GetAttr")

	return kd.Qid, kd.metaMask, kd.meta, nil
}

func (kd *KeyFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	kd.Logger.Debugf("KD Walk names %v", names)
	kd.Logger.Debugf("KD Walk myself: %v", kd.Qid)

	if shouldClone(names) {
		kd.Logger.Debugf("KD Walk cloned")
		return []p9.QID{kd.Qid}, kd, nil
	}

	return walker(kd, names)

	if kd.ents == nil {
		var err error
		if kd.ents, err = getKeys(kd.Ctx, kd.core); err != nil {
			return nil, nil, err
		}
	}

	ipfsDir, err := IPFSAttacher(kd.Ctx, kd.core).Attach()
	if err != nil {
		return nil, nil, err
	}

	return ipfsDir.Walk(names)
}

func (kd *KeyFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	kd.Logger.Debugf("KD Open")

	/*
		handleContext, cancel := context.WithCancel(kd.Ctx)
		kd.cancel = cancel
	*/

	// IPFS core representation

	kd.Logger.Errorf("key ents:%#v", kd.ents)

	return kd.Qid, ipfsBlockSize, nil
}

func (kd *KeyFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	kd.Logger.Debugf("KD Readdir")

	if kd.ents == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", kd.Path.String())
	}

	shouldExit, err := boundCheck(offset, len(kd.ents))
	if shouldExit {
		return nil, err
	}

	subSlice := kd.ents[offset:]
	if len(subSlice) > int(count) {
		subSlice = subSlice[:count]
	}

	kd.Logger.Debugf("KD Readdir returning ents: %v", subSlice)
	return subSlice, nil
}
