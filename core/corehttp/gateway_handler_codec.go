package corehttp

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/tracing"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"
	mc "github.com/multiformats/go-multicodec"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// codecToContentType maps the supported IPLD codecs to the HTTP Content
// Type they should have.
var codecToContentType = map[uint64]string{
	uint64(mc.Json):    "application/json",
	uint64(mc.Cbor):    "application/cbor",
	uint64(mc.DagJson): "application/vnd.ipld.dag-json",
	uint64(mc.DagCbor): "application/vnd.ipld.dag-cbor",
}

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

func (i *gatewayHandler) serveCodec(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, requestedContentType string) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeCodec", trace.WithAttributes(attribute.String("path", resolvedPath.String()), attribute.String("requestedContentType", requestedContentType)))
	defer span.End()

	// If the resolved path still has some remainder, return bad request.
	if resolvedPath.Remainder() != "" {
		path := strings.TrimSuffix(resolvedPath.String(), resolvedPath.Remainder())
		err := fmt.Errorf("%s could not be fully resolved, try %s instead", resolvedPath.String(), path)
		webError(w, "path has remainder", err, http.StatusBadRequest)
		return
	}

	// No content type is specified by the user (via Accept, or format=). However,
	// we support this format. Let's handle it.
	if requestedContentType == "" {
		cidCodec := resolvedPath.Cid().Prefix().Codec
		isDAG := cidCodec == uint64(mc.DagJson) || cidCodec == uint64(mc.DagCbor)
		acceptsHTML := strings.Contains(r.Header.Get("Accept"), "text/html")

		if isDAG && acceptsHTML {
			i.serverCodecHTML(ctx, w, r, resolvedPath, contentPath)
		} else {
			cidContentType, ok := codecToContentType[cidCodec]
			if !ok {
				// Should not happen unless function is called with wrong parameters.
				err := fmt.Errorf("content type not found for codec: %v", cidCodec)
				webError(w, "internal error", err, http.StatusInternalServerError)
				return
			}

			i.serveCodecRaw(ctx, w, r, resolvedPath, contentPath, cidContentType)
		}

		return
	}

	// Otherwise, the user has requested a specific content type. Let's first get
	// the codecs that can be used with this content type.
	codecs, ok := contentTypeToCodecs[requestedContentType]
	if !ok {
		// This is never supposed to happen unless function is called with wrong parameters.
		err := fmt.Errorf("unsupported content type: %s", requestedContentType)
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	// If the requested content type has "dag-", ALWAYS go through the encoding
	// process in order to validate the content.
	if strings.Contains(requestedContentType, "dag-") {
		i.serveCodecConverted(ctx, w, r, resolvedPath, contentPath, requestedContentType, codecs[len(codecs)-1])
		return
	}

	// Otherwise, check if the data is encoded with the requested content type.
	// If so, we can directly stream the raw data. serveRawBlock cannot be directly
	// used here as it sets different headers.
	for _, codec := range codecs {
		if resolvedPath.Cid().Prefix().Codec == codec {
			i.serveCodecRaw(ctx, w, r, resolvedPath, contentPath, requestedContentType)
			return
		}
	}

	// Finally, if nothing of the above is true, we have to actually convert the codec.
	i.serveCodecConverted(ctx, w, r, resolvedPath, contentPath, requestedContentType, codecs[len(codecs)-1])
}

func (i *gatewayHandler) serverCodecHTML(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path) {
	w.Write([]byte("TODO"))
}

func (i *gatewayHandler) serveCodecRaw(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, contentType string) {
	modtime := addCacheControlHeaders(w, r, contentPath, resolvedPath.Cid())
	name := addContentDispositionHeader(w, r, contentPath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

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
}

func (i *gatewayHandler) serveCodecConverted(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, contentType string, codec uint64) {
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

	encoder, err := multicodec.LookupEncoder(codec)
	if err != nil {
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	// Keep it in memory so we can detect encoding errors in order to conform
	// to the specification.
	var buf bytes.Buffer
	err = encoder(finalNode, &buf)
	if err != nil {
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	// Set Cache-Control and read optional Last-Modified time
	modtime := addCacheControlHeaders(w, r, contentPath, resolvedPath.Cid())
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Sets correct Last-Modified header. This code is borrowed from the standard
	// library (net/http/server.go) as we cannot use serveFile.
	if !(modtime.IsZero() || modtime.Equal(unixEpochTime)) {
		w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	}

	w.Write(buf.Bytes())
}
