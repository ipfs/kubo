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
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Set Content-Disposition
	name := rootCid.String() + ".car"
	setContentDispositionHeader(w, name, "attachment")

	// Weak Etag W/ because we can't guarantee byte-for-byte identical  responses
	// (CAR is streamed, and in theory, blocks may arrive from datastore in non-deterministic order)
	etag := `W/` + getEtag(r, rootCid)
	w.Header().Set("Etag", etag)

	// Finish early if Etag match
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Make it clear we don't support range-requests over a car stream
	// Partial downloads and resumes should be handled using
	// IPLD selectors: https://github.com/ipfs/go-ipfs/issues/8769
	w.Header().Set("Accept-Ranges", "none")

	// Explicit Cache-Control to ensure fresh stream on retry.
	// CAR stream could be interrupted, and client should be able to resume and get full response, not the truncated one
	w.Header().Set("Cache-Control", "no-cache, no-transform")

	w.Header().Set("Content-Type", "application/vnd.ipld.car; version=1")
	w.Header().Set("X-Content-Type-Options", "nosniff") // no funny business in the browsers :^)

	// Same go-car settings as dag.export command
	store := dagStore{dag: i.api.Dag(), ctx: ctx}

	// TODO: support selectors passed as request param: https://github.com/ipfs/go-ipfs/issues/8769
	dag := gocar.Dag{Root: rootCid, Selector: selectorparse.CommonSelector_ExploreAllRecursively}
	car := gocar.NewSelectiveCar(ctx, store, []gocar.Dag{dag}, gocar.TraverseLinksOnlyOnce())

	if err := car.Write(w); err != nil {
		// We return error as a trailer, however it is not something browsers can access
		// (https://github.com/mdn/browser-compat-data/issues/14703)
		// Due to this, we suggest client always verify that
		// the received CAR stream response is matching requested DAG selector
		w.Header().Set("X-Stream-Error", err.Error())
		return
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
