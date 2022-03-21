package corehttp

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	cid "github.com/ipfs/go-cid"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
)

// serveRawBlock returns bytes behind a raw block
func (i *gatewayHandler) serveRawBlock(w http.ResponseWriter, r *http.Request, blockCid cid.Cid, contentPath ipath.Path, begin time.Time) {
	blockReader, err := i.api.Block().Get(r.Context(), contentPath)
	if err != nil {
		webError(w, "ipfs block get "+blockCid.String(), err, http.StatusInternalServerError)
		return
	}
	block, err := ioutil.ReadAll(blockReader)
	if err != nil {
		webError(w, "ipfs block get "+blockCid.String(), err, http.StatusInternalServerError)
		return
	}
	content := bytes.NewReader(block)

	// Set Content-Disposition
	name := blockCid.String() + ".bin"
	setContentDispositionHeader(w, name, "attachment")

	// Set remaining headers
	modtime := addCacheControlHeaders(w, r, contentPath, blockCid)
	w.Header().Set("Content-Type", "application/vnd.ipld.raw")
	w.Header().Set("X-Content-Type-Options", "nosniff") // no funny business in the browsers :^)

	// Done: http.ServeContent will take care of
	// If-None-Match+Etag, Content-Length and range requests
	http.ServeContent(w, r, name, modtime, content)

	// Update metrics
	i.rawBlockGetMetric.WithLabelValues(contentPath.Namespace()).Observe(time.Since(begin).Seconds())
}
