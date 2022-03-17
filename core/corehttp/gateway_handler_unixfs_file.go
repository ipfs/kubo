package corehttp

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	gopath "path"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
)

// serveFile returns data behind a file along with HTTP headers based on
// the file itself, its CID and the contentPath used for accessing it.
func (i *gatewayHandler) serveFile(w http.ResponseWriter, r *http.Request, contentPath ipath.Path, fileCid cid.Cid, file files.File) {

	// Set Cache-Control and read optional Last-Modified time
	modtime := addCacheControlHeaders(w, r, contentPath, fileCid)

	// Set Content-Disposition
	name := addContentDispositionHeader(w, r, contentPath)

	// Prepare size value for Content-Length HTTP header (set inside of http.ServeContent)
	size, err := file.Size()
	if err != nil {
		http.Error(w, "cannot serve files with unknown sizes", http.StatusBadGateway)
		return
	}

	// Lazy seeker enables efficient range-requests and HTTP HEAD responses
	content := &lazySeeker{
		size:   size,
		reader: file,
	}

	// Calculate deterministic value for Content-Type HTTP header
	// (we prefer to do it here, rather than using implicit sniffing in http.ServeContent)
	var ctype string
	if _, isSymlink := file.(*files.Symlink); isSymlink {
		// We should be smarter about resolving symlinks but this is the
		// "most correct" we can be without doing that.
		ctype = "inode/symlink"
	} else {
		ctype = mime.TypeByExtension(gopath.Ext(name))
		if ctype == "" {
			// uses https://github.com/gabriel-vasile/mimetype library to determine the content type.
			// Fixes https://github.com/ipfs/go-ipfs/issues/7252
			mimeType, err := mimetype.DetectReader(content)
			if err != nil {
				http.Error(w, fmt.Sprintf("cannot detect content-type: %s", err.Error()), http.StatusInternalServerError)
				return
			}

			ctype = mimeType.String()
			_, err = content.Seek(0, io.SeekStart)
			if err != nil {
				http.Error(w, "seeker can't seek", http.StatusInternalServerError)
				return
			}
		}
		// Strip the encoding from the HTML Content-Type header and let the
		// browser figure it out.
		//
		// Fixes https://github.com/ipfs/go-ipfs/issues/2203
		if strings.HasPrefix(ctype, "text/html;") {
			ctype = "text/html"
		}
	}
	// Setting explicit Content-Type to avoid mime-type sniffing on the client
	// (unifies behavior across gateways and web browsers)
	w.Header().Set("Content-Type", ctype)

	// special fixup around redirects
	w = &statusResponseWriter{w}

	// Done: http.ServeContent will take care of
	// If-None-Match+Etag, Content-Length and range requests
	http.ServeContent(w, r, name, modtime, content)
}
