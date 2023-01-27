package corehttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	ipns_pb "github.com/ipfs/go-ipns/pb"
	path "github.com/ipfs/go-path"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"go.uber.org/zap"
)

func (i *gatewayHandler) serveIpnsRecord(ctx context.Context, w http.ResponseWriter, r *http.Request, resolvedPath ipath.Resolved, contentPath ipath.Path, begin time.Time, logger *zap.SugaredLogger) {
	if contentPath.Namespace() != "ipns" {
		err := fmt.Errorf("%s is not an IPNS link", contentPath.String())
		webError(w, err.Error(), err, http.StatusBadRequest)
		return
	}

	key := contentPath.String()
	key = strings.TrimSuffix(key, "/")
	if strings.Count(key, "/") > 2 {
		err := errors.New("cannot find ipns key for subpath")
		webError(w, err.Error(), err, http.StatusBadRequest)
		return
	}

	rawRecord, err := i.api.Routing().Get(ctx, key)
	if err != nil {
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	var record ipns_pb.IpnsEntry
	err = proto.Unmarshal(rawRecord, &record)
	if err != nil {
		webError(w, err.Error(), err, http.StatusInternalServerError)
		return
	}

	// Set cache control headers based on the TTL set in the IPNS record. If the
	// TTL is not present, we use the Last-Modified tag. We are tracking IPNS
	// caching on: https://github.com/ipfs/kubo/issues/1818.
	// TODO: use addCacheControlHeaders once #1818 is fixed.
	w.Header().Set("Etag", getEtag(r, resolvedPath.Cid()))
	if record.Ttl != nil {
		seconds := int(time.Duration(*record.Ttl).Seconds())
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", seconds))
	} else {
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	}

	// Set Content-Disposition
	var name string
	if urlFilename := r.URL.Query().Get("filename"); urlFilename != "" {
		name = urlFilename
	} else {
		name = path.SplitList(key)[2] + ".ipns-record"
	}
	setContentDispositionHeader(w, name, "attachment")

	w.Header().Set("Content-Type", "application/vnd.ipfs.ipns-record")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	_, _ = w.Write(rawRecord)
}
