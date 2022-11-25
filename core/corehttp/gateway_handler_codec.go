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

	cid "github.com/ipfs/go-cid"
	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/assets"
	dih "github.com/ipfs/kubo/assets/dag-index-html"
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

// contentTypeToExtension maps the HTTP Content Type to the respective file
// extension, used in Content-Disposition header when downloading the file.
var contentTypeToExtension = map[string]string{
	"application/json":              ".json",
	"application/vnd.ipld.dag-json": ".json",
	"application/cbor":              ".cbor",
	"application/vnd.ipld.dag-cbor": ".cbor",
}

func (i *gatewayHandler) serveCodec(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, requestedContentType string) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeCodec", trace.WithAttributes(attribute.String("path", resolvedPath.String()), attribute.String("requestedContentType", requestedContentType)))
	defer span.End()

	cidCodec := resolvedPath.Cid().Prefix().Codec
	responseContentType := requestedContentType

	// If the resolved path still has some remainder, return error for now.
	// TODO: handle this when we have IPLD Patch (https://ipld.io/specs/patch/) via HTTP PUT
	// TODO: (depends on https://github.com/ipfs/kubo/issues/4801 and https://github.com/ipfs/kubo/issues/4782)
	if resolvedPath.Remainder() != "" {
		path := strings.TrimSuffix(resolvedPath.String(), resolvedPath.Remainder())
		err := fmt.Errorf("%q of %q could not be returned: reading IPLD Kinds other than Links (CBOR Tag 42) is not implemented: try reading %q instead", resolvedPath.Remainder(), resolvedPath.String(), path)
		webError(w, "unsupported pathing", err, http.StatusNotImplemented)
		return
	}

	// If no explicit content type was requested, the response will have one based on the codec from the CID
	if requestedContentType == "" {
		cidContentType, ok := codecToContentType[cidCodec]
		if !ok {
			// Should not happen unless function is called with wrong parameters.
			err := fmt.Errorf("content type not found for codec: %v", cidCodec)
			webError(w, "internal error", err, http.StatusInternalServerError)
			return
		}
		responseContentType = cidContentType
	}

	// Set HTTP headers (for caching etc)
	modtime := addCacheControlHeaders(w, r, contentPath, resolvedPath.Cid())
	name := setCodecContentDisposition(w, r, resolvedPath, responseContentType)
	w.Header().Set("Content-Type", responseContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// No content type is specified by the user (via Accept, or format=). However,
	// we support this format. Let's handle it.
	if requestedContentType == "" {
		isDAG := cidCodec == uint64(mc.DagJson) || cidCodec == uint64(mc.DagCbor)
		acceptsHTML := strings.Contains(r.Header.Get("Accept"), "text/html")
		download := r.URL.Query().Get("download") == "true"

		if isDAG && acceptsHTML && !download {
			i.serveCodecHTML(ctx, w, r, resolvedPath, contentPath)
		} else {
			i.serveCodecRaw(ctx, w, r, resolvedPath, contentPath, name, modtime)
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

	// If we need to convert, use the last codec (strict dag- variant)
	toCodec := codecs[len(codecs)-1]

	// If the requested content type has "dag-", ALWAYS go through the encoding
	// process in order to validate the content.
	if strings.Contains(requestedContentType, "dag-") {
		i.serveCodecConverted(ctx, w, r, resolvedPath, contentPath, toCodec, modtime)
		return
	}

	// Otherwise, check if the data is encoded with the requested content type.
	// If so, we can directly stream the raw data. serveRawBlock cannot be directly
	// used here as it sets different headers.
	for _, codec := range codecs {
		if resolvedPath.Cid().Prefix().Codec == codec {
			i.serveCodecRaw(ctx, w, r, resolvedPath, contentPath, name, modtime)
			return
		}
	}

	// Finally, if nothing of the above is true, we have to actually convert the codec.
	i.serveCodecConverted(ctx, w, r, resolvedPath, contentPath, toCodec, modtime)
}

func (i *gatewayHandler) serveCodecHTML(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path) {
	// A HTML directory index will be presented, be sure to set the correct
	// type instead of relying on autodetection (which may fail).
	w.Header().Set("Content-Type", "text/html")

	// Clear Content-Disposition -- we want HTML to be rendered inline
	w.Header().Del("Content-Disposition")

	// Generated index requires custom Etag (output may change between Kubo versions)
	dagEtag := getDagIndexEtag(resolvedPath.Cid())
	w.Header().Set("Etag", dagEtag)

	// Remove Cache-Control for now to match UnixFS dir-index-html responses
	// (we don't want browser to cache HTML forever)
	// TODO: if we ever change behavior for UnixFS dir listings, same changes should be applied here
	w.Header().Del("Cache-Control")

	cidCodec := mc.Code(resolvedPath.Cid().Prefix().Codec)
	if err := dih.DagIndexTemplate.Execute(w, dih.DagIndexTemplateData{
		Path:      contentPath.String(),
		CID:       resolvedPath.Cid().String(),
		CodecName: cidCodec.String(),
		CodecHex:  fmt.Sprintf("0x%x", uint64(cidCodec)),
	}); err != nil {
		webError(w, "failed to generate HTML listing for this DAG: try fetching raw block with ?format=raw", err, http.StatusInternalServerError)
	}
}

func (i *gatewayHandler) serveCodecRaw(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, name string, modtime time.Time) {
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

func (i *gatewayHandler) serveCodecConverted(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, toCodec uint64, modtime time.Time) {
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

	encoder, err := multicodec.LookupEncoder(toCodec)
	if err != nil {
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	// Ensure IPLD node conforms to the codec specification.
	var buf bytes.Buffer
	err = encoder(finalNode, &buf)
	if err != nil {
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	// Sets correct Last-Modified header. This code is borrowed from the standard
	// library (net/http/server.go) as we cannot use serveFile.
	if !(modtime.IsZero() || modtime.Equal(unixEpochTime)) {
		w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	}

	_, _ = w.Write(buf.Bytes())
}

func setCodecContentDisposition(w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentType string) string {
	var dispType, name string

	ext, ok := contentTypeToExtension[contentType]
	if !ok {
		// Should never happen.
		ext = ".bin"
	}

	if urlFilename := r.URL.Query().Get("filename"); urlFilename != "" {
		name = urlFilename
	} else {
		name = resolvedPath.Cid().String() + ext
	}

	// JSON should be inlined, but ?download=true should still override
	if r.URL.Query().Get("download") == "true" {
		dispType = "attachment"
	} else {
		switch ext {
		case ".json": // codecs that serialize to JSON can be rendered by browsers
			dispType = "inline"
		default: // everything else is assumed binary / opaque bytes
			dispType = "attachment"
		}
	}

	setContentDispositionHeader(w, name, dispType)
	return name
}

func getDagIndexEtag(dagCid cid.Cid) string {
	return `"DagIndex-` + assets.AssetHash + `_CID-` + dagCid.String() + `"`
}
