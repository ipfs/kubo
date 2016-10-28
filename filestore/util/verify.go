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
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
	//cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
)

type VerifyParams struct {
	Filter         ListFilter
	Level          int
	Verbose        int
	NoObjInfo      bool
	SkipOrphans    bool
	PostOrphan     bool
	IncompleteWhen []string
}

func CheckParamsBasic(fs *Basic, params *VerifyParams) (VerifyLevel, int, error) {
	level, err := VerifyLevelFromNum(fs, params.Level)
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
	iter := ListIterator{Iterator: fs.DB().NewIterator()}
	if params.Filter == nil {
		iter.Filter = func(r *DataObj) bool { return r.NoBlockData() }
	} else {
		iter.Filter = func(r *DataObj) bool { return r.NoBlockData() && params.Filter(r) }
	}
	verifyLevel, verbose, err := CheckParamsBasic(fs, params)
	if err != nil {
		return nil, err
	}
	out := reporter{make(chan ListRes, 16), params.NoObjInfo}
	go func() {
		defer out.close()
		for iter.Next() {
			key := iter.Key()
			dataObj, err := iter.Value()
			if err != nil {
				out.send(ListRes{key.Key, nil, StatusCorrupt})
			}
			status := verify(fs, key, dataObj, verifyLevel)
			if verbose >= ShowTopLevel || OfInterest(status) {
				out.send(ListRes{key.Key, dataObj, status})
			}
		}
	}()
	return out.ch, nil
}

func VerifyKeys(ks []*DbKey, node *core.IpfsNode, fs *Basic, params *VerifyParams) (<-chan ListRes, error) {
	out := reporter{make(chan ListRes, 16), params.NoObjInfo}
	verifyLevel, verbose, err := CheckParamsBasic(fs, params)
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

func verifyKey(dsKey *DbKey, fs *Basic, bs b.Blockstore, verifyLevel VerifyLevel) ListRes {
	_, dataObj, err := fs.GetDirect(dsKey)
	if err == nil && dataObj.NoBlockData() {
		res := ListRes{dsKey.Key, dataObj, 0}
		res.Status = verify(fs, dsKey, dataObj, verifyLevel)
		return res
	} else if err == nil {
		return ListRes{dsKey.Key, dataObj, StatusUnchecked}
	}
	c, _ := dsKey.Cid()
	found, _ := bs.Has(c)
	if found {
		return ListRes{dsKey.Key, nil, StatusFound}
	} else if err == ds.ErrNotFound && !found {
		return ListRes{dsKey.Key, nil, StatusKeyNotFound}
	} else {
		Logger.Errorf("%s: verifyKey: %v", dsKey.Format(), err)
		return ListRes{dsKey.Key, nil, StatusError}
	}
}

func VerifyFull(node *core.IpfsNode, fs Snapshot, params *VerifyParams) (<-chan ListRes, error) {
	verifyLevel, verbose, err := CheckParamsBasic(fs.Basic, params)
	if err != nil {
		return nil, err
	}
	skipOrphans := params.SkipOrphans
	postOrphan := params.PostOrphan
	if skipOrphans && postOrphan {
		return nil, fmt.Errorf("cannot specify both skip-orphans and post-orphan")
	}
	if params.Filter != nil {
		skipOrphans = true
		postOrphan = false
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
	iter := ListIterator{fs.DB().NewIterator(), params.Filter}
	go func() {
		defer p.out.close()
		switch {
		case skipOrphans:
			p.verifyRecursive(iter)
		case postOrphan:
			p.verifyPostOrphan(iter)
		default:
			p.verifyFull(iter)
		}
	}()
	return p.out.ch, nil
}

func VerifyKeysFull(ks []*DbKey, node *core.IpfsNode, fs *Basic, params *VerifyParams) (<-chan ListRes, error) {
	verifyLevel, verbose, err := CheckParamsBasic(fs, params)
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

// type VerifyType int

// const (
// 	Recursive VerifyType = iota
// 	Full
// 	PostOrphan
// )

type Hash string

type seen struct {
	status    Status
	reachable bool
}

type verifyParams struct {
	out            reporter
	node           *core.IpfsNode
	fs             *Basic
	verifyLevel    VerifyLevel
	verboseLevel   int // see help text for meaning
	seen           map[string]seen
	roots          []string
	incompleteWhen []bool
}

func (p *verifyParams) getStatus(key string) seen {
	if p.seen == nil {
		return seen{0, false}
	} else {
		return p.seen[key]
	}
}

func (p *verifyParams) setStatus(key *DbKey, val *DataObj, status Status, reachable bool) {
	if p.seen != nil {
		val, ok := p.seen[key.Hash]
		if status > 0 && !ok {
			p.seen[key.Hash] = seen{status, reachable}
		} else {
			if status > 0 {val.status = status}
			if reachable {val.reachable = true}
			p.seen[key.Hash] = val
		}
	}
	if p.roots != nil && val != nil && val.WholeFile() {
		p.roots = append(p.roots, key.Hash)
	}
}

func (p *verifyParams) verifyKeys(ks []*DbKey) {
	for _, dsKey := range ks {
		//if key == "" {
		//	continue
		//}
		res, children, r := p.get(dsKey)
		if res == nil || AnError(r) {
			/* nothing to do */
		} else if res[0].Val.Internal() {
			kv := res[0]
			r = p.verifyNode(children)
			p.verifyPostTopLevel(kv.Key, kv.Val, r)
			return
		}
		for _, kv := range res {
			r = p.verifyLeaf(kv.Key, kv.Val)
			p.verifyPostTopLevel(kv.Key, kv.Val, r)
		}
	}
}

func (p *verifyParams) verifyPostTopLevel(dsKey *DbKey, dataObj *DataObj, r Status) {
	res := ListRes{dsKey.Key, dataObj, r}
	res.Status = p.checkIfAppended(res)
	if p.verboseLevel >= ShowSpecified || OfInterest(res.Status) {
		p.out.send(res)
		p.out.ch <- EmptyListRes
	}
}

func (p *verifyParams) verifyRecursive(iter ListIterator) {
	p.verifyTopLevel(iter)
}

func (p *verifyParams) verifyFull(iter ListIterator) error {
	p.seen = make(map[string]seen)

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
	p.seen = make(map[string]seen)
	p.roots = make([]string, 0)
	
	reportErr := p.verifyTopLevel(iter)

	err := p.markReachable(p.roots)

	if reportErr != nil || err != nil {
		return InternalError
	}

	p.outputFutureOrphans()

	p.checkOrphans()

	return nil
}

var InternalError = errors.New("database corrupt or related")

func (p *verifyParams) verifyTopLevel(iter ListIterator) error {
	unsafeToCont := false
	for iter.Next() {
		key := iter.Key()
		r := StatusUnchecked
		val, err := iter.Value()
		if err != nil {
			r = StatusCorrupt
		}
		if AnError(r) {
			p.reportTopLevel(key, val, r)
		} else if val.Internal() && val.WholeFile() {
			children, err := GetLinks(val)
			if err != nil {
				r = StatusCorrupt
			} else {
				r = p.verifyNode(children)
			}
			p.reportTopLevel(key, val, r)
			p.setStatus(key, val, r, false)
		} else if val.WholeFile() {
			r = p.verifyLeaf(key, val)
			p.reportTopLevel(key, val, r)
			// mark the node as seen, but do not cache the status as
			// that status might be incomplete
			p.setStatus(key, val, StatusUnchecked, false)
		} else {
			// FIXME: Is this doing anything useful?
			p.setStatus(key, val, 0, false)
			continue
		}
		if AnInternalError(r) {
			unsafeToCont = true
		}
	}
	if unsafeToCont {
		return InternalError
	} else {
		return nil
	}
}

func (p *verifyParams) reportTopLevel(key *DbKey, val *DataObj, status Status) {
	res := ListRes{key.Key, val, status}
	res.Status = p.checkIfAppended(res)
	if p.verboseLevel >= ShowTopLevel || (p.verboseLevel >= 0 && OfInterest(res.Status)) {
		p.out.send(res)
		p.out.ch <- EmptyListRes
	}
}

func (p *verifyParams) checkOrphans() {
	for k, v := range p.seen {
		if v.reachable {
			continue
		}
		p.outputOrphans(k, v.status)
	}
}

func (p *verifyParams) checkIfAppended(res ListRes) Status {
	//println("checkIfAppened:", res.FormatDefault())
	if p.verifyLevel <= CheckExists || p.verboseLevel < 0 ||
		!IsOk(res.Status) || !res.WholeFile() || res.FilePath == "" {
		//println("checkIfAppened no go", res.FormatDefault())
		return res.Status
	}
	//println("checkIfAppened no checking", res.FormatDefault())
	info, err := os.Stat(res.FilePath)
	if err != nil {
		Logger.Warningf("%s: checkIfAppended: %v", res.MHash(), err)
		return res.Status
	}
	if uint64(info.Size()) > res.Size {
		return StatusAppended
	}
	return res.Status
}

var depth = 0

func (p *verifyParams) markReachable(keys []string) error {
	depth += 1
	for _, hash := range keys {
		v := p.seen[hash]
		r := v.status
		if r == StatusMarked {
			continue
		}
		if AnInternalError(r) { // not stricly necessary, but lets be extra safe
			return InternalError
		}
		//println("status", HashToKey(hash).Format(), r)
		if InternalNode(r) && r != StatusIncomplete {
			key := HashToKey(hash)
			_, val, err := p.fs.GetDirect(key)
			if err != nil {
				//println("um an error")
				return err
			}
			links, err := GetLinks(val)
			children := make([]string, 0, len(links))
			for _, link := range links {
				children = append(children, dshelp.CidToDsKey(link.Cid).String())
			}
			//println("recurse", depth, HashToKey(hash).Format(), "count", len(children))
			p.markReachable(children)
		}
		//println("seen", depth, HashToKey(hash).Format())
		v.status = StatusMarked
		p.seen[hash] = v
	}
	depth -= 1
	return nil
}

func (p *verifyParams) outputFutureOrphans() {
	for hash, v := range p.seen {
		if v.status == StatusMarked || v.status == StatusNone {
			continue
		}
		p.outputOrphans(hash, v.status)
	}
}

func (p *verifyParams) outputOrphans(hashStr string, status Status) {
	hash := HashToKey(hashStr)
	kvs, err := p.fs.GetAll(hash)
	if err != nil {
		Logger.Errorf("%s: verify: %v", MHash(hash), err)
		p.out.send(ListRes{hash.Key, nil, StatusError})
	}
	for _, kv := range kvs {
		if kv.Val.WholeFile() {
			continue
		}
		if status == StatusNone && kv.Val.NoBlockData() {
			r := p.verifyLeaf(kv.Key, kv.Val)
			if AnError(r) {
				p.out.send(ListRes{kv.Key.Key, kv.Val, r})
			}
		}
		p.out.send(ListRes{kv.Key.Key, kv.Val, StatusOrphan})
	}
}

func (p *verifyParams) verifyNode(links []*node.Link) Status {
	finalStatus := StatusComplete
	for _, link := range links {
		hash := CidToKey(link.Cid)
		v := p.getStatus(hash.Hash)
		if v.status == 0 {
			objs, children, r := p.get(hash)
			var dataObj *DataObj
			if objs != nil {
				dataObj = objs[0].Val
			}
			if AnError(r) {
				p.reportNodeStatus(hash, dataObj, r)
			} else if len(children) > 0 {
				r = p.verifyNode(children)
				p.reportNodeStatus(hash, dataObj, r)
				p.setStatus(hash, dataObj, r, true)
			} else if objs != nil {
				r = StatusNone
				for _, kv := range objs {
					r0 := p.verifyLeaf(kv.Key, kv.Val)
					p.reportNodeStatus(kv.Key, kv.Val, r0)
					if p.rank(r0) < p.rank(r) {
						r = r0
					}
				}
				p.setStatus(hash, dataObj, r, true)
			}
			v.status = r
		}
		if AnInternalError(v.status) {
			return StatusError
		} else if p.incompleteWhen[v.status] {
			finalStatus = StatusIncomplete
		} else if !IsOk(v.status) && !Unchecked(v.status) {
			finalStatus = StatusProblem
		}
	}
	if finalStatus == StatusComplete && p.verifyLevel > CheckExists {
		finalStatus = StatusAllPartsOk
	}
	return finalStatus
}

func (p *verifyParams) reportNodeStatus(key *DbKey, val *DataObj, status Status) {
	if p.verboseLevel >= ShowChildren || (p.verboseLevel >= ShowProblemChildren && OfInterest(status)) {
		p.out.send(ListRes{key.Key, val, status})
	}
}

// determine the rank of the status indicator if multiple entries have
// the same hash and differnt status, the one with the lowest rank
// will be used
func (p *verifyParams) rank(r Status) int {
	category := r - r%10
	switch {
	case r == 0:
		return 999
	case category == CategoryOk:
		return int(r)
	case category == CategoryUnchecked:
		return 100 + int(r)
	case category == CategoryBlockErr && !p.incompleteWhen[r]:
		return 200 + int(r)
	case category == CategoryBlockErr && p.incompleteWhen[r]:
		return 400 + int(r)
	case category == CategoryOtherErr:
		return 500 + int(r)
	default:
		// should not really happen
		return 600 + int(r)
	}
}

func (p *verifyParams) verifyLeaf(key *DbKey, dataObj *DataObj) Status {
	return verify(p.fs, key, dataObj, p.verifyLevel)
}

func (p *verifyParams) get(k *DbKey) ([]KeyVal, []*node.Link, Status) {
	return getNodes(k, p.fs, p.node.Blockstore)
}
