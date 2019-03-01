package fusemount

import (
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	lru "gx/ipfs/QmQjMHF8ptRgx4E57UFMiT4YM6kqaJeYxZ1MCDX23aw4rK/golang-lru"
)

/* OLD
type cidCache interface {
	Add(*cid.Cid, fusePath)
	Request(*cid.Cid) fusePath
	Release(*cid.Cid)
}
*/

type cidCache struct {
	actual *lru.ARCCache
	cb     cid.Builder
}

func (cc *cidCache) Add(nCid cid.Cid, fp fusePath) {
	if cidCacheEnabled {
		cc.actual.Add(nCid, fp)
	}
}

func (cc *cidCache) Request(nCid cid.Cid) fusePath {
	if !cidCacheEnabled {
		return nil
	}
	if v, ok := cc.actual.Get(nCid); ok {
		if _, ok = v.(fusePath); !ok {
			log.Errorf("Cache entry for %q is not valid: {%T}%#v", nCid, v, v)
			return nil
		}
		return v.(fusePath)
	}
	return nil
}

func (cc *cidCache) Release(nCid cid.Cid) {
	if !cidCacheEnabled {
		return
	}
	cc.actual.Remove(nCid)
}

//TODO: size from conf
func (cc *cidCache) Init() error {
	if !cidCacheEnabled {
		return nil
	}

	arc, err := lru.NewARC(100) //NOTE: arbitrary debug size
	if err != nil {
		return err
	}
	cc.actual = arc
	//TODO: pkg/cid should expose a recommendation hint, i.e. cid.RecommendedDefault
	cc.cb = cid.NewPrefixV1(cid.Raw, mh.SHA2_256)
	return nil
}

func (cc *cidCache) ReleasePath(path string) {
	if !cidCacheEnabled {
		return
	}

	pathCid, err := cc.cb.Sum([]byte(path))
	if err != nil {
		log.Errorf("Cache - hash error [report this]: %s", err)
		return
	}
	cc.actual.Remove(pathCid)
}

func (cc *cidCache) RequestPath(path string) fusePath {
	if !cidCacheEnabled {
		return nil
	}

	pathCid, err := cc.cb.Sum([]byte(path))
	if err != nil {
		log.Errorf("Cache - hash error [report this]: %s", err)
		return nil
	}
	if node, ok := cc.actual.Get(pathCid); ok {
		return node.(fusePath)
	}
	return nil
}

func (cc *cidCache) Hash(path string) (cid.Cid, error) {
	return cc.cb.Sum([]byte(path))
}
