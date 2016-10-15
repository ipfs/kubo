package filestore_util

import (
	"errors"
	"fmt"
	"os"
	//"sync"
	//"strings"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	. "github.com/ipfs/go-ipfs/filestore/support"
	k "gx/ipfs/QmYEoKZXHoAToWfhGF3vryhMn3WWhE1o2MasQ8uzY5iDi9/go-key"
	//b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	node "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
)

type VerifyParams struct {
	Filter         ListFilter
	Level          int
	Verbose        int
	NoObjInfo      bool
	SkipOrphans    bool
	IncompleteWhen []string
}

func CheckParamsBasic(params *VerifyParams) (VerifyLevel, int, error) {
	level, err := VerifyLevelFromNum(params.Level)
	if err != nil {
		return 0, 0, err
	}
	verbose := params.Verbose
	if verbose < 0 || verbose > 9 {
		return 0, 0, errors.New("verbose must be between 0-9")
	}
	return level, verbose, nil
}

func ParseIncompleteWhen(args []string) ([]bool, error) {
	ret := make([]bool, 100)
	ret[StatusKeyNotFound] = true
	ret[StatusIncomplete] = true
	for _, arg := range args {
		switch arg {
		case "changed":
			ret[StatusFileChanged] = true
		case "no-file":
			ret[StatusFileMissing] = true
		case "error":
			ret[StatusFileError] = true
		default:
			return nil, fmt.Errorf("IncompleteWhen: Expect one of: changed, no-file, error.  Got: %s", arg)
		}
	}
	return ret, nil
}

type reporter struct {
	ch        chan ListRes
	noObjInfo bool
}

func (out *reporter) send(res ListRes) {
	if out.noObjInfo {
		res.DataObj = nil
	}
	out.ch <- res
}

func (out *reporter) close() {
	close(out.ch)
}

func VerifyBasic(fs *Basic, params *VerifyParams) (<-chan ListRes, error) {
	iter := ListIterator{Iterator: fs.NewIterator()}
	if params.Filter == nil {
		iter.Filter = func(r *DataObj) bool { return r.NoBlockData() }
	} else {
		iter.Filter = func(r *DataObj) bool { return r.NoBlockData() && params.Filter(r) }
	}
	verifyLevel, verbose, err := CheckParamsBasic(params)
	if err != nil {
		return nil, err
	}
	out := reporter{make(chan ListRes, 16), params.NoObjInfo}
	go func() {
		defer out.close()
		for iter.Next() {
			key := iter.Key()
			bytes, dataObj, err := iter.Value()
			if err != nil {
				out.send(ListRes{key, nil, StatusCorrupt})
			}
			status := verify(fs, key, bytes, dataObj, verifyLevel)
			if verbose >= ShowTopLevel || OfInterest(status) {
				out.send(ListRes{key, dataObj, status})
			}
		}
	}()
	return out.ch, nil
}

func VerifyKeys(ks []*cid.Cid, node *core.IpfsNode, fs *Basic, params *VerifyParams) (<-chan ListRes, error) {
	out := reporter{make(chan ListRes, 16), params.NoObjInfo}
	verifyLevel, verbose, err := CheckParamsBasic(params)
	if err != nil {
		return nil, err
	}
	go func() {
		defer out.close()
		for _, k := range ks {
			//if key == "" {
			//	continue
			//}
			res := verifyKey(k, fs, node.Blockstore, verifyLevel)
			if verbose >= ShowSpecified || OfInterest(res.Status) {
				out.send(res)
			}
		}
	}()
	return out.ch, nil
}

func verifyKey(k *cid.Cid, fs *Basic, bs b.Blockstore, verifyLevel VerifyLevel) ListRes {
	dsKey := b.CidToDsKey(k)
	origData, dataObj, err := fs.GetDirect(dsKey)
	if err == nil && dataObj.NoBlockData() {
		res := ListRes{dsKey, dataObj, 0}
		res.Status = verify(fs, dsKey, origData, dataObj, verifyLevel)
		return res
	} else if err == nil {
		return ListRes{dsKey, dataObj, StatusUnchecked}
	}
	found, _ := bs.Has(k)
	if found {
		return ListRes{dsKey, nil, StatusFound}
	} else if err == ds.ErrNotFound && !found {
		return ListRes{dsKey, nil, StatusKeyNotFound}
	} else {
		Logger.Errorf("%s: verifyKey: %v", k, err)
		return ListRes{dsKey, nil, StatusError}
	}
}

func VerifyFull(node *core.IpfsNode, fs Snapshot, params *VerifyParams) (<-chan ListRes, error) {
	verifyLevel, verbose, err := CheckParamsBasic(params)
	if err != nil {
		return nil, err
	}
	skipOrphans := params.SkipOrphans
	if params.Filter != nil {
		skipOrphans = true
	}
	p := verifyParams{
		out:          reporter{make(chan ListRes, 16), params.NoObjInfo},
		node:         node,
		fs:           fs.Basic,
		verifyLevel:  verifyLevel,
		verboseLevel: verbose,
	}
	p.incompleteWhen, err = ParseIncompleteWhen(params.IncompleteWhen)
	if err != nil {
		return nil, err
	}
	iter := ListIterator{fs.NewIterator(), params.Filter}
	go func() {
		defer p.out.close()
		if skipOrphans {
			p.verifyRecursive(iter)
		} else {
			p.verifyFull(iter)
		}
	}()
	return p.out.ch, nil
}

func VerifyKeysFull(ks []*cid.Cid, node *core.IpfsNode, fs *Basic, params *VerifyParams) (<-chan ListRes, error) {
	verifyLevel, verbose, err := CheckParamsBasic(params)
	if err != nil {
		return nil, err
	}
	p := verifyParams{
		out:          reporter{make(chan ListRes, 16), params.NoObjInfo},
		node:         node,
		fs:           fs,
		verifyLevel:  verifyLevel,
		verboseLevel: verbose,
	}
	p.incompleteWhen, err = ParseIncompleteWhen(params.IncompleteWhen)
	if err != nil {
		return nil, err
	}
	go func() {
		defer p.out.close()
		p.verifyKeys(ks)
	}()
	return p.out.ch, nil
}

func VerifyPostOrphan(node *core.IpfsNode, fs Snapshot, level int, incompleteWhen []string) (<-chan ListRes, error) {
	verifyLevel, err := VerifyLevelFromNum(level)
	if err != nil {
		return nil, err
	}
	p := verifyParams{
		out:         reporter{make(chan ListRes, 16), true},
		node:        node,
		fs:          fs.Basic,
		verifyLevel: verifyLevel,
	}
	p.incompleteWhen, err = ParseIncompleteWhen(incompleteWhen)
	if err != nil {
		return nil, err
	}
	iter := ListIterator{fs.NewIterator(), nil}
	go func() {
		defer p.out.close()
		p.verifyPostOrphan(iter)
	}()
	return p.out.ch, nil
}

// type VerifyType int

// const (
// 	Recursive VerifyType = iota
// 	Full
// 	PostOrphan
// )

type verifyParams struct {
	out            reporter
	node           *core.IpfsNode
	fs             *Basic
	verifyLevel    VerifyLevel
	verboseLevel   int // see help text for meaning
	seen           map[ds.Key]int
	roots          []ds.Key
	incompleteWhen []bool
}

func (p *verifyParams) getStatus(key ds.Key) int {
	if p.seen == nil {
		return 0
	} else {
		return p.seen[key]
	}
}

func (p *verifyParams) setStatus(key ds.Key, val *DataObj, status int) ListRes {
	if p.seen != nil {
		_, ok := p.seen[key]
		if !ok || status > 0 {
			p.seen[key] = status
		}
	}
	if p.roots != nil && val != nil && val.WholeFile() {
		p.roots = append(p.roots, key)
	}
	return ListRes{key, val, status}
}

func (p *verifyParams) verifyKeys(ks []*cid.Cid) {
	for _, k := range ks {
		//if key == "" {
		//	continue
		//}
		dsKey := b.CidToDsKey(k)
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
		if p.verboseLevel >= ShowSpecified || OfInterest(res.Status) {
			p.out.send(res)
			p.out.ch <- EmptyListRes
		}
	}
}

func (p *verifyParams) verifyRecursive(iter ListIterator) {
	p.verifyTopLevel(iter)
}

func (p *verifyParams) verifyFull(iter ListIterator) error {
	p.seen = make(map[ds.Key]int)

	err := p.verifyTopLevel(iter)
	// An error indicates an internal error that might mark some nodes
	// incorrectly as orphans, so exit early
	if err != nil {
		return InternalError
	}

	p.checkOrphans()

	return nil
}

func (p *verifyParams) verifyPostOrphan(iter ListIterator) error {
	p.seen = make(map[ds.Key]int)
	p.roots = make([]ds.Key, 0)

	p.verboseLevel = -1
	reportErr := p.verifyTopLevel(iter)

	err := p.markReachable(p.roots)

	if reportErr != nil || err != nil {
		return InternalError
	}

	p.markFutureOrphans()

	p.checkOrphans()

	return nil
}

var InternalError = errors.New("database corrupt or related")

func (p *verifyParams) verifyTopLevel(iter ListIterator) error {
	unsafeToCont := false
	for iter.Next() {
		key := iter.Key()
		r := StatusUnchecked
		origData, val, err := iter.Value()
		if err != nil {
			r = StatusCorrupt
		}
		if AnError(r) {
			/* nothing to do */
		} else if val.Internal() && val.WholeFile() {
			children, err := GetLinks(val)
			if err != nil {
				r = StatusCorrupt
			} else {
				r = p.verifyNode(children)
			}
		} else if val.WholeFile() {
			r = p.verifyLeaf(key, origData, val)
		} else {
			p.setStatus(key, val, 0)
			continue
		}
		if AnInternalError(r) {
			unsafeToCont = true
		}
		res := p.setStatus(key, val, r)
		res.Status = p.checkIfAppended(res)
		if p.verboseLevel >= ShowTopLevel || (p.verboseLevel >= 0 && OfInterest(res.Status)) {
			p.out.send(res)
			p.out.ch <- EmptyListRes
		}
	}
	if unsafeToCont {
		return InternalError
	} else {
		return nil
	}
}

func (p *verifyParams) checkOrphans() {
	for key, status := range p.seen {
		if status != 0 {
			continue
		}
		bytes, val, err := p.fs.GetDirect(key)
		if err != nil {
			Logger.Errorf("%s: verify: %v", MHash(key), err)
			p.out.send(ListRes{key, val, StatusError})
		} else if val.NoBlockData() {
			status = p.verifyLeaf(key, bytes, val)
			if AnError(status) {
				p.out.send(ListRes{key, val, status})
			}
		}
		p.out.send(ListRes{key, val, StatusOrphan})
	}
}

func (p *verifyParams) checkIfAppended(res ListRes) int {
	if p.verifyLevel <= CheckExists || p.verboseLevel < 0 ||
		!IsOk(res.Status) || !res.WholeFile() || res.FilePath == "" {
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

func (p *verifyParams) markReachable(keys []ds.Key) error {
	for _, key := range keys {
		r := p.seen[key]
		if r == StatusMarked {
			continue
		}
		if AnInternalError(r) { // not stricly necessary, but lets me extra safe
			return InternalError
		}
		if InternalNode(r) && r != StatusIncomplete {
			_, val, err := p.fs.GetDirect(key)
			if err != nil {
				return err
			}
			links, err := GetLinks(val)
			children := make([]ds.Key, 0, len(links))
			for _, link := range links {
				children = append(children, k.Key(link.Hash).DsKey())
			}
			p.markReachable(children)
		}
		if OfInterest(r) {
			p.out.send(ListRes{key, nil, r})
		}
		p.seen[key] = StatusMarked
	}
	return nil
}

func (p *verifyParams) markFutureOrphans() {
	for key, status := range p.seen {
		if status == StatusMarked || status == 0 {
			continue
		}
		if AnError(status) {
			p.out.send(ListRes{key, nil, status})
		}
		p.out.send(ListRes{key, nil, StatusOrphan})
	}
}

func (p *verifyParams) verifyNode(links []*node.Link) int {
	finalStatus := StatusComplete
	for _, link := range links {
		key := k.Key(link.Hash).DsKey()
		res := ListRes{Key: key}
		res.Status = p.getStatus(key)
		if res.Status == 0 {
			origData, dataObj, children, r := p.get(key)
			if AnError(r) {
				/* nothing to do */
			} else if len(children) > 0 {
				r = p.verifyNode(children)
			} else if dataObj != nil {
				r = p.verifyLeaf(key, origData, dataObj)
			}
			res = p.setStatus(key, dataObj, r)
		}
		if p.verboseLevel >= ShowChildren || (p.verboseLevel >= ShowProblemChildren && OfInterest(res.Status)) {
			p.out.send(res)
		}
		if AnInternalError(res.Status) {
			return StatusError
		} else if p.incompleteWhen[res.Status] {
			finalStatus = StatusIncomplete
		} else if !IsOk(res.Status) && !Unchecked(res.Status) {
			finalStatus = StatusProblem
		}
	}
	if finalStatus == StatusComplete && p.verifyLevel > CheckExists {
		finalStatus = StatusAllPartsOk
	}
	return finalStatus
}

func (p *verifyParams) verifyLeaf(key ds.Key, origData []byte, dataObj *DataObj) int {
	return verify(p.fs, key, origData, dataObj, p.verifyLevel)
}

func (p *verifyParams) get(dsKey ds.Key) ([]byte, *DataObj, []*node.Link, int) {
	return getNode(dsKey, p.fs, p.node.Blockstore)
}
