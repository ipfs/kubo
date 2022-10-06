package corehttp

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"time"

	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"
)

// serveCAR returns a CAR stream for specific DAG+selector
func (i *gatewayHandler) serveJSON(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, begin time.Time, ctype string, codec uint64) {
	// ctx, span := tracing.Span(ctx, "Gateway", "ServeCAR", trace.WithAttributes(attribute.String("path", resolvedPath.String())))
	// defer span.End()
	// ctx, cancel := context.WithCancel(ctx)
	// defer cancel()

	obj, err := i.api.Dag().Get(r.Context(), resolvedPath.Cid())
	if err != nil {
		webError(w, "ipfs dag get "+html.EscapeString(resolvedPath.String()), err, http.StatusInternalServerError)
		return
	}

	universal, ok := obj.(ipldlegacy.UniversalNode)
	if !ok {
		webError(w, "todo", fmt.Errorf("%T is not a valid IPLD node", obj), http.StatusInternalServerError)
		return
	}
	finalNode := universal.(ipld.Node)

	encoder, err := multicodec.LookupEncoder(codec)
	if err != nil {
		webError(w, "todo", err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", ctype)
	_ = encoder(finalNode, w)
}
