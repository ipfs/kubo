package corehttp

import (
	"context"
	"net/http"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	gocar "github.com/ipld/go-car"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
)

// serveCar returns a CAR stream for specific DAG+selector
func (i *gatewayHandler) serveCar(w http.ResponseWriter, r *http.Request, rootCid cid.Cid, contentPath ipath.Path) {
	ctx := r.Context()

	// Set Content-Disposition
	name := rootCid.String() + ".car"
	setContentDispositionHeader(w, name, "attachment")

	// Set remaining headers
	/* TODO modtime := addCacheControlHeaders(w, r, contentPath, rootCid)
	- how does cache-control look like, given car can fail mid-stream?
	  - we don't want clients to cache partial/interrupted CAR
	  - we may document that client should verify that all blocks were dowloaded,
	    or we may leverage content-length to hint something went wrong
	*/

	/* TODO: content-length (so user agents show % of remaining download)
	- introduce max-car-size  limit in go-ipfs-config and pre-compute CAR first, and then get size and use lazySeeker?
	- are we able to provide length for Unixfs DAGs? (CumulativeSize+CARv0 header+envelopes)
	*/

	w.Header().Set("Content-Type", "application/vnd.ipld.car; version=1")
	w.Header().Set("X-Content-Type-Options", "nosniff") // no funny business in the browsers :^)

	// Same go-car settings as dag.export command
	store := dagStore{dag: i.api.Dag(), ctx: ctx}
	dag := gocar.Dag{Root: rootCid, Selector: selectorparse.CommonSelector_ExploreAllRecursively}
	car := gocar.NewSelectiveCar(ctx, store, []gocar.Dag{dag}, gocar.TraverseLinksOnlyOnce())

	w.WriteHeader(http.StatusOK)

	if err := car.Write(w); err != nil {
		// TODO: can we do any error handling here?
	}
}

type dagStore struct {
	dag coreiface.APIDagService
	ctx context.Context
}

func (ds dagStore) Get(c cid.Cid) (blocks.Block, error) {
	obj, err := ds.dag.Get(ds.ctx, c)
	return obj, err
}
