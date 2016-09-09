package filestore_util

import (
	"os"
	//"sync"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	. "github.com/ipfs/go-ipfs/filestore/support"
	//b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	node "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
)

func VerifyBasic(fs Snapshot, level int, verbose int) (<-chan ListRes, error) {
	iter := ListIterator{
		Iterator: fs.NewIterator(),
		Filter: func(r *DataObj) bool { return r.NoBlockData() },
	}
	verifyLevel, err := VerifyLevelFromNum(level)
	if err != nil {
		return nil, err
	}
	out := make(chan ListRes, 16)
	go func() {
		defer close(out)
		for iter.Next() {
			key := iter.Key()
			bytes, dataObj, err := iter.Value()
			if err != nil {
				out <- ListRes{key, nil, StatusCorrupt}
			}
			status := verify(fs.Basic, key, bytes, dataObj, verifyLevel)
			if verbose >= 3 || OfInterest(status) {
				out <- ListRes{key, dataObj, status}
			}
		}
	}()
	return out, nil
}

func VerifyKeys(keys []k.Key, node *core.IpfsNode, fs *Basic, level int, verbose int) (<-chan ListRes, error) {
	out := make(chan ListRes, 16)
	verifyLevel, err := VerifyLevelFromNum(level)
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(out)
		for _, key := range keys {
			if key == "" {
				continue
			}
			res := verifyKey(key, fs, node.Blockstore, verifyLevel)
			if verbose > 1 || OfInterest(res.Status) {
				out <- res
			}
		}
	}()
	return out, nil
}

func verifyKey(key k.Key, fs *Basic, bs b.Blockstore, verifyLevel VerifyLevel) ListRes {
	dsKey := key.DsKey()
	origData, dataObj, err := fs.GetDirect(dsKey)
	if err == nil && dataObj.NoBlockData() {
		res := ListRes{dsKey, dataObj, 0}
		res.Status = verify(fs, dsKey, origData, dataObj, verifyLevel)
		return res
	} else if err == nil {
		return ListRes{dsKey, dataObj, StatusUnchecked}
	}
	found, _ := bs.Has(key)
	if found {
		return ListRes{dsKey, nil, StatusFound}
	} else if err == ds.ErrNotFound && !found {
		return ListRes{dsKey, nil, StatusKeyNotFound}
	} else {
		Logger.Errorf("%s: verifyKey: %v", key, err)
		return ListRes{dsKey, nil, StatusError}
	}
}

func VerifyFull(node *core.IpfsNode, fs Snapshot, level int, verbose int, skipOrphans bool) (<-chan ListRes, error) {
	verifyLevel, err := VerifyLevelFromNum(level)
	if err != nil {
		return nil, err
	}
	p := verifyParams{make(chan ListRes, 16), node, fs.Basic, verifyLevel, verbose, skipOrphans, nil}
	iter := ListIterator{fs.NewIterator(), ListFilterAll}
	go func() {
		defer close(p.out)
		p.verify(iter)
	}()
	return p.out, nil
}

func VerifyKeysFull(keys []k.Key, node *core.IpfsNode, fs *Basic, level int, verbose int) (<-chan ListRes, error) {
	verifyLevel, err := VerifyLevelFromNum(level)
	if err != nil {
		return nil, err
	}
	p := verifyParams{make(chan ListRes, 16), node, fs, verifyLevel, verbose, true, nil}
	go func() {
		defer close(p.out)
		p.verifyKeys(keys)
	}()
	return p.out, nil
}

type verifyParams struct {
	out         chan ListRes
	node        *core.IpfsNode
	fs          *Basic
	verifyLevel VerifyLevel
	// level 7-9 show everything
	//       5-6 don't show child nodes with a status of StatusOk, StatusUnchecked, or StatusComplete
	//       3-4 don't show child nodes
	//       0-2 don't show child nodes and don't show root nodes with of StatusOk, or StatusComplete
	verboseLevel int
	skipOrphans  bool // don't check for orphans
	seen         map[string]int
}

func (p *verifyParams) setStatus(dsKey ds.Key, status int) {
	if p.skipOrphans {
		return
	}
	key := string(dsKey.Bytes()[1:])
	_, ok := p.seen[key]
	if !ok || status > 0 {
		p.seen[key] = status
	}
}

// FIXME: Unify this with verifyNode?
func (p *verifyParams) verifyKeys(keys []k.Key) {
	p.skipOrphans = true
	for _, key := range keys {
		if key == "" {
			continue
		}
		dsKey := key.DsKey()
		origData, dataObj, children, r := p.get(dsKey)
		if dataObj == nil || AnError(r) {
			/* nothing to do */
		} else if dataObj.Internal() {
			r = p.verifyNode(children)
		} else {
			r = p.verifyLeaf(dsKey, origData, dataObj)
		}
		res := ListRes{dsKey, dataObj, r}
		res.Status = p.checkIfAppended(res)
		if p.verboseLevel > 1 || OfInterest(res.Status) {
			p.out <- res
			p.out <- EmptyListRes
		}
	}
}

func (p *verifyParams) verify(iter ListIterator) {
	p.seen = make(map[string]int)
	unsafeToCont := false
	for iter.Next() {
		res := ListRes{iter.Key(), nil, 0}
		r := StatusUnchecked
		origData, dataObj, err := iter.Value()
		if err != nil {
			r = StatusCorrupt
		}
		res.DataObj = dataObj
		if AnError(r) {
			/* nothing to do */
		} else if res.Internal() && res.WholeFile() {
			children, err := GetLinks(dataObj)
			if err != nil {
				r = StatusCorrupt
			} else {
				r = p.verifyNode(children)
			}
		} else if res.WholeFile() {
			r = p.verifyLeaf(res.Key, origData, res.DataObj)
		} else {
			p.setStatus(res.Key, 0)
			continue
		}
		if AnInternalError(r) {
			unsafeToCont = true
		}
		res.Status = r
		res.Status = p.checkIfAppended(res)
		p.setStatus(res.Key, r)
		if p.verboseLevel >= 2 || OfInterest(res.Status) {
			p.out <- res
			p.out <- EmptyListRes
		}
	}
	// If we get an internal error we may incorrectly mark nodes
	// some nodes as orphans, so exit early
	if unsafeToCont {
		return
	}
	// Now check the orphans
	for key, status := range p.seen {
		if status != 0 {
			continue
		}
		dsKey := ds.NewKey(key)
		bytes, val, err := p.fs.GetDirect(dsKey)
		status := StatusUnchecked
		if err != nil {
			Logger.Errorf("%s: verify: %v", MHash(dsKey), err)
			status = StatusError
		} else if val.NoBlockData() {
			status = p.verifyLeaf(dsKey, bytes, val)
			if !AnError(status) {
				status = StatusOrphan
			}
		} else {
			status = StatusOrphan
		}
		p.out <- ListRes{dsKey, val, status}
	}
}

func (p *verifyParams) checkIfAppended(res ListRes) int {
	if res.Status != StatusOk || !res.WholeFile() || res.FilePath == "" {
		return res.Status
	}
	info, err := os.Stat(res.FilePath)
	if err != nil {
		Logger.Errorf("%s: checkIfAppended: %v", res.MHash(), err)
		return StatusError
	}
	if uint64(info.Size()) > res.Size {
		return StatusAppended
	}
	return res.Status
}

func (p *verifyParams) verifyNode(links []*node.Link) int {
	complete := true
	for _, link := range links {
		key := k.Key(link.Hash).DsKey()
		origData, dataObj, children, r := p.get(key)
		if AnError(r) {
			/* nothing to do */
		} else if len(children) > 0 {
			r = p.verifyNode(children)
		} else if dataObj != nil {
			r = p.verifyLeaf(key, origData, dataObj)
		}
		p.setStatus(key, r)
		res := ListRes{key, dataObj, r}
		if p.verboseLevel >= 7 || (p.verboseLevel >= 4 && OfInterest(r)) {
			p.out <- res
		}
		if AnInternalError(r) {
			return StatusError
		} else if AnError(r) {
			complete = false
		}
	}
	if complete && p.verifyLevel <= CheckExists {
		return StatusComplete
	} else if complete {
		return StatusOk
	} else {
		return StatusIncomplete
	}
}

func (p *verifyParams) verifyLeaf(key ds.Key, origData []byte, dataObj *DataObj) int {
	return verify(p.fs, key, origData, dataObj, p.verifyLevel)
}

func (p *verifyParams) get(dsKey ds.Key) ([]byte, *DataObj, []*node.Link, int) {
	return getNode(dsKey, p.fs, p.node.Blockstore)
}
