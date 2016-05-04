package filestore_util

import (
	"os"

	bs "github.com/ipfs/go-ipfs/blocks/blockstore"
	k "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	//b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	//mh "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/ipfs/go-datastore"
	node "github.com/ipfs/go-ipfs/merkledag"
)

func VerifyBasic(fs *Datastore, level int, verbose int) (<-chan ListRes, error) {
	in, err := List(fs, false)
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
			if !res.NoBlockData() {
				continue
			}
			res.Status = verify(fs, res.Key, res.DataObj, verifyWhat)
			if verbose >= 3 || OfInterest(res.Status) {
				out <- res
			}
		}
	}()
	return out, nil
}

func VerifyFull(node *core.IpfsNode, fs *Datastore, level int, verbose int, skipOrphans bool) (<-chan ListRes, error) {
	p := verifyParams{make(chan ListRes, 16), node, fs, level, verbose, skipOrphans, nil}
	ch, err := List(p.fs, true)
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(p.out)
		p.verify(ch)
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

func (p *verifyParams) verify(ch <-chan ListRes) {
	p.seen = make(map[string]int)
	unsafeToCont := false
	for res := range ch {
		dagNode, dataObj, r := p.get(res.Key)
		res.DataObj = dataObj
		if dataObj == nil {
			r = StatusError
		}
		if AnError(r) {
			/* nothing to do */
		} else if res.FileRoot() {
			if dagNode == nil {
				// we expect a node, so even if the status is
				// okay we should set it to an Error
				if !AnError(r) {
					r = StatusError
				}
			} else {
				r = p.verifyNode(dagNode)
			}
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
	dataObj, err := p.fs.GetDirect(key)
	if err == nil {
		//println("in filestore ", b58.Encode(key.Bytes()[1:]))
		if dataObj.NoBlockData() {
			return nil, dataObj, StatusUnchecked
		} else {
			node, err := node.DecodeProtobuf(dataObj.Data)
			if err != nil {
				return nil, nil, StatusCorrupt
			}
			return node, dataObj, StatusOk
		}
	}
	//println("not in filestore ", b58.Encode(key.Bytes()[1:]))
	block, err2 := p.node.Blockstore.Get(k.KeyFromDsKey(key))
	if err == ds.ErrNotFound && err2 == bs.ErrNotFound {
		return nil, nil, StatusKeyNotFound
	} else if err2 != nil {
		return nil, nil, StatusError
	}
	node, err := node.DecodeProtobuf(block.Data())
	if err != nil {
		return nil, nil, StatusCorrupt
	}
	return node, nil, StatusFound
}
