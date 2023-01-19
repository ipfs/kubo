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
	"go.uber.org/zap"
)

// convertibleCodecs maps supported input codecs into supported output codecs.
var convertibleCodecs = map[mc.Code][]mc.Code{
	mc.Raw:     {mc.DagCbor, mc.DagJson},
	mc.DagPb:   {mc.DagCbor, mc.DagJson},
	mc.DagJson: {mc.DagCbor, mc.DagJson},
	mc.DagCbor: {mc.DagCbor, mc.DagJson},
	mc.Json:    {mc.Cbor, mc.Json, mc.DagCbor, mc.DagJson},
	mc.Cbor:    {mc.Cbor, mc.Json, mc.DagCbor, mc.DagJson},
}

// contentTypeToExtension maps the HTTP Content Type to the respective file
// extension, used in Content-Disposition header when downloading the file.
var contentTypeToExtension = map[string]string{
	"application/json":              ".json",
	"application/vnd.ipld.dag-json": ".json",
	"application/cbor":              ".cbor",
	"application/vnd.ipld.dag-cbor": ".cbor",
}

// getResponseContentTypeAndCodec returns the response content type and codec based
// on the requested content type and CID codec. The requested content type has
// priority over the CID codec.
func getResponseContentTypeAndCodec(requestedContentType string, codec mc.Code) (string, mc.Code) {
	switch requestedContentType {
	case "application/json":
		return "application/json", mc.Json
	case "application/cbor":
		return "application/cbor", mc.Cbor
	case "application/vnd.ipld.dag-json":
		return "application/vnd.ipld.dag-json", mc.DagJson
	case "application/vnd.ipld.dag-cbor":
		return "application/vnd.ipld.dag-cbor", mc.DagCbor
	}

	switch codec {
	case mc.Json:
		return "application/json", mc.Json
	case mc.Cbor:
		return "application/cbor", mc.Cbor
	case mc.DagJson:
		return "application/vnd.ipld.dag-json", mc.DagJson
	case mc.DagCbor:
		return "application/vnd.ipld.dag-cbor", mc.DagCbor
	}

	return "", 0
}

func (i *gatewayHandler) serveCodec(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, requestedContentType string, logger *zap.SugaredLogger) {
	ctx, span := tracing.Span(ctx, "Gateway", "ServeCodec", trace.WithAttributes(attribute.String("path", resolvedPath.String()), attribute.String("requestedContentType", requestedContentType)))
	defer span.End()

	// If the resolved path still has some remainder, return error for now.
	// TODO: handle this when we have IPLD Patch (https://ipld.io/specs/patch/) via HTTP PUT
	// TODO: (depends on https://github.com/ipfs/kubo/issues/4801 and https://github.com/ipfs/kubo/issues/4782)
	if resolvedPath.Remainder() != "" {
		path := strings.TrimSuffix(resolvedPath.String(), resolvedPath.Remainder())
		err := fmt.Errorf("%q of %q could not be returned: reading IPLD Kinds other than Links (CBOR Tag 42) is not implemented: try reading %q instead", resolvedPath.Remainder(), resolvedPath.String(), path)
		webError(w, "unsupported pathing", err, http.StatusNotImplemented)
		return
	}

	cidCodec := mc.Code(resolvedPath.Cid().Prefix().Codec)
	responseContentType, responseCodec := getResponseContentTypeAndCodec(requestedContentType, cidCodec)

	// This should never happen unless function is called with wrong parameters.
	if responseContentType == "" {
		err := fmt.Errorf("content type not found for codec: %v", cidCodec)
		webError(w, "internal error", err, http.StatusInternalServerError)
		return
	}

	// Set HTTP headers (for caching, etc).
	modtime := addCacheControlHeaders(w, r, contentPath, resolvedPath.Cid())
	name := setCodecContentDisposition(w, r, resolvedPath, responseContentType)
	w.Header().Set("Content-Type", responseContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// No content type is specified by the user (via Accept, or format=). However,
	// we support this format. Let's handle it.
	if requestedContentType == "" {
		isDAG := responseCodec == mc.DagJson || responseCodec == mc.DagCbor
		acceptsHTML := strings.Contains(r.Header.Get("Accept"), "text/html")
		download := r.URL.Query().Get("download") == "true"

		if isDAG && acceptsHTML && !download {
			i.serveCodecHTML(ctx, w, r, resolvedPath, contentPath)
		} else {
			// Here we cannot use serveRawBlock because we want to use the right
			// content type as we know the content type we are serving.
			i.serveCodecRaw(ctx, w, r, resolvedPath, contentPath, name, modtime)
		}

		return
	}

	// This should never happen unless the function is called with wrong parameters.
	if _, ok := convertibleCodecs[cidCodec]; !ok {
		err := fmt.Errorf("codec cannot be handled: %v", cidCodec)
		webError(w, "internal error", err, http.StatusInternalServerError)
		return
	}

	// If the user has requested a CID in some content type that can be converted
	// to the target content type, we serve it converted with the correct headers.
	for _, targetCodec := range convertibleCodecs[cidCodec] {
		if targetCodec == responseCodec {
			i.serveCodecConverted(ctx, w, r, resolvedPath, contentPath, responseCodec, modtime)
			return
		}
	}

	// If the user has requested for a conversion that is not possible (such as
	// requesting a UnixFS file as a JSON), we defer to the regular serve UnixFS
	// function that will serve the data behind it accordingly.
	i.serveUnixFS(ctx, w, r, resolvedPath, contentPath, begin, logger)
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

func (i *gatewayHandler) serveCodecConverted(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, toCodec mc.Code, modtime time.Time) {
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

	encoder, err := multicodec.LookupEncoder(uint64(toCodec))
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
