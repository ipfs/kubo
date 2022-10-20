package corehttp

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/tracing"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"
	"github.com/ipld/go-ipld-prime/traversal"
	mc "github.com/multiformats/go-multicodec"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var unixEpochTime = time.Unix(0, 0)

// contentTypeToCodecs maps the HTTP Content Type to the respective
// possible codecs. If the original data is in one of those codecs,
// we stream the raw bytes. Otherwise, we encode in the last codec
// of the list.
var contentTypeToCodecs = map[string][]uint64{
	"application/json":              {uint64(mc.Json), uint64(mc.DagJson)},
	"application/vnd.ipld.dag-json": {uint64(mc.DagJson)},
	"application/cbor":              {uint64(mc.Cbor), uint64(mc.DagCbor)},
	"application/vnd.ipld.dag-cbor": {uint64(mc.DagCbor)},
}

func (i *gatewayHandler) serveCodec(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, contentType string) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeCodec", trace.WithAttributes(attribute.String("path", resolvedPath.String()), attribute.String("contentType", contentType)))
	defer span.End()

	codecs, ok := contentTypeToCodecs[contentType]
	if !ok {
		// This is never supposed to happen unless function is called with wrong parameters.
		err := fmt.Errorf("unsupported content type: %s", contentType)
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	// Set Cache-Control and read optional Last-Modified time
	modtime := addCacheControlHeaders(w, r, contentPath, resolvedPath.Cid())
	name := addContentDispositionHeader(w, r, contentPath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// If the data is already encoded with the possible codecs, we can just stream the raw
	// data. serveRawBlock cannot be directly used here as it sets different headers.
	for _, codec := range codecs {
		if resolvedPath.Cid().Prefix().Codec == codec {

			blockCid := resolvedPath.Cid()
			blockReader, err := i.api.Block().Get(ctx, resolvedPath)
			if err != nil {
				webError(w, "ipfs block get "+blockCid.String(), err, http.StatusInternalServerError)
				return
			}
			block, err := io.ReadAll(blockReader)
			if err != nil {
				webError(w, "ipfs block get "+blockCid.String(), err, http.StatusInternalServerError)
				return
			}
			content := bytes.NewReader(block)

			// ServeContent will take care of
			// If-None-Match+Etag, Content-Length and range requests
			_, _, _ = ServeContent(w, r, name, modtime, content)
			return
		}
	}

	// Sets correct Last-Modified header. This code is borrowed from the standard
	// library (net/http/server.go) as we cannot use serveFile.
	if !(modtime.IsZero() || modtime.Equal(unixEpochTime)) {
		w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	}

	obj, err := i.api.Dag().Get(ctx, resolvedPath.Cid())
	if err != nil {
		webError(w, "ipfs dag get "+html.EscapeString(resolvedPath.String()), err, http.StatusInternalServerError)
		return
	}

	universal, ok := obj.(ipldlegacy.UniversalNode)
	if !ok {
		err = fmt.Errorf("%T is not a valid IPLD node", obj)
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}
	finalNode := universal.(ipld.Node)

	if len(resolvedPath.Remainder()) > 0 {
		remainderPath := ipld.ParsePath(resolvedPath.Remainder())

		finalNode, err = traversal.Get(finalNode, remainderPath)
		if err != nil {
			webError(w, err.Error(), err, http.StatusInternalServerError)
			return
		}
	}

	// Otherwise convert it using the last codec of the list.
	encoder, err := multicodec.LookupEncoder(codecs[len(codecs)-1])
	if err != nil {
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	_ = encoder(finalNode, w)
}
