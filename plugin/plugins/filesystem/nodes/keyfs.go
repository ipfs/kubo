package fsnodes

import (
	"context"
	"fmt"
	gopath "path"
	"runtime"
	"sync"
	"time"

	"github.com/hugelgupf/p9/p9"
	"github.com/hugelgupf/p9/unimplfs"
	cid "github.com/ipfs/go-cid"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	fsutils "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/utils"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

var _ p9.File = (*KeyFS)(nil)
var _ fsutils.WalkRef = (*KeyFS)(nil)

type KeyFS struct {
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr

	IPFSBase
	KeyFSFileMeta

	// shared roots across all FS instances
	sharedLock *sync.Mutex                // should be held when accessing the root map
	mroots     map[string]fsutils.WalkRef // map["key"]*MFS{}
}

type KeyFSFileMeta struct {
	ents []p9.Dirent
	// TODO support key as a file too
	// keyFile KeyIO
}

func KeyFSAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	kd := &KeyFS{
		IPFSBase: newIPFSBase(ctx, "/keyfs", core, ops...),
		mroots:   make(map[string]fsutils.WalkRef),
	}
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

	// detach from our proxied system when we fall out of memory
	runtime.SetFinalizer(kd, func(keyRoot *KeyFS) {
		keyRoot.proxy.Close()
	})

	return kd
}

func (kd *KeyFS) Attach() (p9.File, error) {
	kd.Logger.Debugf("Attach")

	newFid := &KeyFS{
		IPFSBase: kd.IPFSBase.clone(),
		mroots:   kd.mroots,
	}

	// set new fs context
	if err := newFid.forkFilesystem(); err != nil {
		return nil, err
	}
	return newFid, nil
}

func (kd *KeyFS) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	kd.Logger.Debugf("Open")

	ctx, cancel := kd.callCtx()
	defer cancel()

	ents, err := getKeys(ctx, kd.core)
	if err != nil {
		kd.Logger.Errorf("Open hit: %s", err)
		cancel()
		return *kd.qid, 0, err
	}
	kd.ents = ents

	kd.Logger.Errorf("Open ret: %v", kd.ents)

	return *kd.qid, 0, nil

}
func (kd *KeyFS) Close() error {
	kd.ents = nil
	return kd.IPFSBase.close()
}

func (kd *KeyFS) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	if kd.ents == nil {
		return nil, fmt.Errorf("directory %q is not open for reading", kd.String())
	}

	return flatReaddir(kd.ents, offset, count)
}

/* WalkRef relevant */

func (kd *KeyFS) Fork() (fsutils.WalkRef, error) {
	base, err := kd.IPFSBase.fork()
	if err != nil {
		return nil, err
	}

	newFid := &KeyFS{
		IPFSBase: base,
		mroots:   kd.mroots,
	}
	return newFid, nil
}

// KeyFS forks the IPFS root that was set during construction
// and calls step on it rather than itself
func (kd *KeyFS) Step(keyName string) (fsutils.WalkRef, error) {
	callCtx, cancel := kd.callCtx()
	defer cancel()

	key, err := getKey(callCtx, keyName, kd.core)
	switch err {
	default:
		// unexpected failure
		return nil, err

	case errKeyNotInStore:
		// proxy non-keyed requests to an IPNS derivative
		proxyRef, err := kd.proxy.Fork()
		if err != nil {
			return nil, err
		}
		return proxyRef.Step(keyName)

	case nil:
		// appropriate roots that are names of keys we own
		mfsNode, ok := kd.mroots[keyName]
		if !ok {
			// init
			corePath, err := kd.core.ResolvePath(callCtx, key.Path())
			if err != nil {
				return nil, err
			}

			//TODO: check key target's type; MFS for dirs, UnixIO for files
			cid := corePath.Cid()
			opts := []nodeopts.AttachOption{
				nodeopts.Parent(kd),
				nodeopts.MFSPublish(ipnsPublisher(key.Name(), offlineAPI(kd.core).Name())),
				nodeopts.MFSRoot(cid),
				nodeopts.Logger(logging.Logger("IPNS-Key")),
			}

			mRoot, err := MFSAttacher(kd.filesystemCtx, kd.core, opts...).Attach()
			if err != nil {
				return nil, err
			}

			mfsNode = mRoot.(fsutils.WalkRef)
			kd.mroots[keyName] = mfsNode

			// TODO: validate this
			runtime.SetFinalizer(mfsNode, func(wr fsutils.WalkRef) {
				delete(kd.mroots, keyName)
			})
		}

		return mfsNode, nil
	}
}

func (kd *KeyFS) CheckWalk() error                    { return kd.Base.checkWalk() }
func (kd *KeyFS) QID() (p9.QID, error)                { return kd.Base.qID() }
func (kd *KeyFS) Backtrack() (fsutils.WalkRef, error) { return kd.IPFSBase.backtrack(kd) }

/* base class boilerplate */

func (kd *KeyFS) Walk(names []string) ([]p9.QID, p9.File, error) {
	kd.Logger.Debugf("Walk names %v", names)
	kd.Logger.Debugf("Walk myself: %v", kd.qid)

	return fsutils.Walker(kd, names)
}

func (kd *KeyFS) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	return kd.Base.getAttr(req)
}

func getKeys(ctx context.Context, core coreiface.CoreAPI) ([]p9.Dirent, error) {
	keys, err := core.Key().List(ctx)
	if err != nil {
		return nil, err
	}

	ents := make([]p9.Dirent, 0, len(keys))

	// temporary conversion storage
	attr := &p9.Attr{}
	requestType := p9.AttrMask{Mode: true}

	var offset uint64 = 1
	for _, key := range keys {
		//
		ipldNode, err := core.ResolveNode(ctx, key.Path())
		if err != nil {
			//FIXME: bug in either the CoreAPI, http client, or somewhere else
			//if err == coreiface.ErrResolveFailed {
			//HACK:
			if err.Error() == coreiface.ErrResolveFailed.Error() {
				continue // skip unresolvable keys (typical when a key exists but hasn't been published to
			}
			return nil, err
		}
		if _, err = ipldStat(ctx, attr, ipldNode, requestType); err != nil {
			return nil, err
		}

		ents = append(ents, p9.Dirent{
			//Name:   gopath.Base(key.Path().String()),
			Name:   gopath.Base(key.Name()),
			Offset: offset,
			QID: p9.QID{
				Type: attr.Mode.QIDType(),
				Path: cidToQPath(ipldNode.Cid()),
			},
		})
		offset++
	}
	return ents, nil
}

func ipnsPublisher(keyName string, nameAPI coreiface.NameAPI) func(context.Context, cid.Cid) error {
	return func(ctx context.Context, rootCid cid.Cid) error {
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		_, err := nameAPI.Publish(callCtx, corepath.IpfsPath(rootCid), coreoptions.Name.Key(keyName), coreoptions.Name.AllowOffline(true))
		return err
	}
}

func getKey(ctx context.Context, keyName string, core coreiface.CoreAPI) (coreiface.Key, error) {
	if keyName == "self" {
		return core.Key().Self(ctx)
	}

	keys, err := core.Key().List(ctx)
	if err != nil {
		return nil, err
	}

	var key coreiface.Key
	for _, curKey := range keys {
		if curKey.Name() == keyName {
			key = curKey
			break
		}
	}

	if key == nil {
		//return nil, fmt.Errorf(errFmtExternalWalk, keyName)
		return nil, errKeyNotInStore
	}

	return key, nil
}
