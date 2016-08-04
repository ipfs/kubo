package filestore_util

import (
	"os"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	//b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	node "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
)

func VerifyBasic(fs *Datastore, level int, verbose int) (<-chan ListRes, error) {
	in, err := List(fs, func(r ListRes) bool { return r.NoBlockData() })
	if err != nil {
		return nil, err
	}
	verifyWhat := VerifyAlways
	out := make(chan ListRes, 16)
	if level <= 6 {
		verifyWhat = VerifyIfChanged
	}
	go func() {
		defer close(out)
		for res := range in {
			res.Status = verify(fs, res.Key, res.DataObj, verifyWhat)
			if verbose >= 3 || OfInterest(res.Status) {
				out <- res
			}
		}
	}()
	return out, nil
}

func VerifyKeys(keys []k.Key, node *core.IpfsNode, fs *Datastore, level int, verbose int) (<-chan ListRes, error) {
	out := make(chan ListRes, 16)
	verifyWhat := VerifyAlways
	if level <= 6 {
		verifyWhat = VerifyIfChanged
	}
	go func() {
		defer close(out)
		for _, key := range keys {
			if key == "" {
				continue
			}
			res := verifyKey(key, fs, node.Blockstore, verifyWhat)
			if verbose > 1 || OfInterest(res.Status) {
				out <- res
			}
		}
	}()
	return out, nil
}

func verifyKey(key k.Key, fs *Datastore, bs b.Blockstore, verifyWhat int) ListRes {
	dsKey := key.DsKey()
	dataObj, err := fs.GetDirect(dsKey)
	if err == nil && dataObj.NoBlockData() {
		res := ListRes{dsKey, dataObj, 0}
		res.Status = verify(fs, dsKey, dataObj, verifyWhat)
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
		Logger.Errorf("%s: %v", key, err)
		return ListRes{dsKey, nil, StatusError}
	}
}

func VerifyFull(node *core.IpfsNode, fs *Datastore, level int, verbose int, skipOrphans bool) (<-chan ListRes, error) {
	p := verifyParams{make(chan ListRes, 16), node, fs, level, verbose, skipOrphans, nil}
	ch, err := ListKeys(p.fs)
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(p.out)
		p.verify(ch)
	}()
	return p.out, nil
}

func VerifyKeysFull(keys []k.Key, node *core.IpfsNode, fs *Datastore, level int, verbose int) (<-chan ListRes, error) {
	p := verifyParams{make(chan ListRes, 16), node, fs, level, verbose, true, nil}
	go func() {
		defer close(p.out)
		p.verifyKeys(keys)
	}()
	return p.out, nil
}

type verifyParams struct {
	out  chan ListRes
	node *core.IpfsNode
	fs   *Datastore
	// level 0-1 means do not verify leaf nodes
	// level 2-6 means to verify based on time stamp
	// level 7-9 means to always verify
	// other levels may be added in the future, the larger the
	// number the more expensive the checks are
	verifyLevel int
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

func (p *verifyParams) verifyKeys(keys []k.Key) {
	p.skipOrphans = true
	for _, key := range keys {
		if key == "" {
			continue
		}
		dsKey := key.DsKey()
		dagNode, dataObj, r := p.get(dsKey)
		if dataObj == nil || AnError(r) {
			/* nothing to do */
		} else if dataObj.Internal() {
			r = p.verifyNode(dagNode)
		} else {
			r = p.verifyLeaf(dsKey, dataObj)
		}
		res := ListRes{dsKey, dataObj, r}
		res.Status = p.checkIfAppended(res)
		if p.verboseLevel > 1 || OfInterest(res.Status) {
			p.out <- res
			p.out <- EmptyListRes
		}
	}
}

func (p *verifyParams) verify(ch <-chan ListRes) {
	p.seen = make(map[string]int)
	unsafeToCont := false
	for res := range ch {
		dagNode, dataObj, r := p.get(res.Key)
		if dataObj == nil {
			r = StatusError
		}
		res.DataObj = dataObj
		if AnError(r) {
			/* nothing to do */
		} else if res.Internal() && res.WholeFile() {
			r = p.verifyNode(dagNode)
		} else if res.WholeFile() {
			r = p.verifyLeaf(res.Key, res.DataObj)
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
	// If we get an internal error we may incorrect mark nodes
	// some nodes orphans, so exit early
	if unsafeToCont {
		return
	}
	for key, status := range p.seen {
		if status != 0 {
			continue
		}
		res := ListRes{Key: ds.NewKey(key)}
		var err error
		res.DataObj, err = p.fs.GetDirect(res.Key)
		if err != nil {
			res.Status = StatusError
		} else if res.NoBlockData() {
			res.Status = p.verifyLeaf(res.Key, res.DataObj)
			if !AnError(res.Status) {
				res.Status = StatusOrphan
			}
		} else {
			res.Status = StatusOrphan
		}
		p.out <- res
	}
}

func (p *verifyParams) checkIfAppended(res ListRes) int {
	if res.Status != StatusOk || !res.WholeFile() {
		return res.Status
	}
	info, err := os.Stat(res.FilePath)
	if err != nil {
		return StatusError
	}
	if uint64(info.Size()) > res.Size {
		return StatusAppended
	}
	return res.Status
}

func (p *verifyParams) verifyNode(n *node.Node) int {
	if n == nil {
		return StatusError
	}
	complete := true
	for _, link := range n.Links {
		key := k.Key(link.Hash).DsKey()
		dagNode, dataObj, r := p.get(key)
		if AnError(r) || (dagNode != nil && len(dagNode.Links) == 0) {
			/* nothing to do */
		} else if dagNode != nil {
			r = p.verifyNode(dagNode)
		} else {
			r = p.verifyLeaf(key, dataObj)
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
	if complete && p.verifyLevel <= 1 {
		return StatusComplete
	} else if complete {
		return StatusOk
	} else {
		return StatusIncomplete
	}
}

func (p *verifyParams) verifyLeaf(key ds.Key, dataObj *DataObj) int {
	if p.verifyLevel <= 1 {
		return StatusUnchecked
	} else if p.verifyLevel <= 6 {
		return verify(p.fs, key, dataObj, VerifyIfChanged)
	} else {
		return verify(p.fs, key, dataObj, VerifyAlways)
	}
}

func (p *verifyParams) get(key ds.Key) (*node.Node, *DataObj, int) {
	dsKey, err := k.KeyFromDsKey(key)
	if err != nil {
		return nil, nil, StatusCorrupt
	}
	return getNode(key, dsKey, p.fs, p.node.Blockstore)
}
