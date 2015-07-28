package http

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/rs/cors"

	cmds "github.com/ipfs/go-ipfs/commands"
	u "github.com/ipfs/go-ipfs/util"
)

var log = u.Logger("commands/http")

// the internal handler for the API
type internalHandler struct {
	ctx  cmds.Context
	root *cmds.Command
}

// The Handler struct is funny because we want to wrap our internal handler
// with CORS while keeping our fields.
type Handler struct {
	internalHandler
	corsHandler http.Handler
}

var ErrNotFound = errors.New("404 page not found")

const (
	StreamErrHeader        = "X-Stream-Error"
	streamHeader           = "X-Stream-Output"
	channelHeader          = "X-Chunked-Output"
	uaHeader               = "User-Agent"
	contentTypeHeader      = "Content-Type"
	contentLengthHeader    = "Content-Length"
	contentDispHeader      = "Content-Disposition"
	transferEncodingHeader = "Transfer-Encoding"
	applicationJson        = "application/json"
	applicationOctetStream = "application/octet-stream"
	plainText              = "text/plain"
)

var mimeTypes = map[string]string{
	cmds.JSON: "application/json",
	cmds.XML:  "application/xml",
	cmds.Text: "text/plain",
}

func NewHandler(ctx cmds.Context, root *cmds.Command, allowedOrigin string) *Handler {
	// allow whitelisted origins (so we can make API requests from the browser)
	if len(allowedOrigin) > 0 {
		log.Info("Allowing API requests from origin: " + allowedOrigin)
	}

	// Create a handler for the API.
	internal := internalHandler{ctx, root}

	// Create a CORS object for wrapping the internal handler.
	c := cors.New(cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT"},

		// use AllowOriginFunc instead of AllowedOrigins because we want to be
		// restrictive by default.
		AllowOriginFunc: func(origin string) bool {
			return (allowedOrigin == "*") || (origin == allowedOrigin)
		},
	})

	// Wrap the internal handler with CORS handling-middleware.
	return &Handler{internal, c.Handler(internal)}
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Call the CORS handler which wraps the internal handler.
	i.corsHandler.ServeHTTP(w, r)
}

func (i internalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Incoming API request: ", r.URL)

	// error on external referers (to prevent CSRF attacks)
	referer := r.Referer()
	scheme := r.URL.Scheme
	if len(scheme) == 0 {
		scheme = "http"
	}
	host := fmt.Sprintf("%s://%s/", scheme, r.Host)
	// empty string means the user isn't following a link (they are directly typing in the url)
	if referer != "" && !strings.HasPrefix(referer, host) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 - Forbidden"))
		return
	}

	req, err := Parse(r, i.root)
	if err != nil {
		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(err.Error()))
		return
	}

	// get the node's context to pass into the commands.
	node, err := i.ctx.GetNode()
	if err != nil {
		s := fmt.Sprintf("cmds/http: couldn't GetNode(): %s", err)
		http.Error(w, s, http.StatusInternalServerError)
		return
	}

	//ps: take note of the name clash - commands.Context != context.Context
	req.SetInvocContext(i.ctx)
	err = req.SetRootContext(node.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// call the command
	res := i.root.Call(req)

	// now handle responding to the client properly
	sendResponse(w, req, res)
}

func guessMimeType(res cmds.Response) (string, error) {
	// Try to guess mimeType from the encoding option
	enc, found, err := res.Request().Option(cmds.EncShort).String()
	if err != nil {
		return "", err
	}
	if !found {
		return "", errors.New("no encoding option set")
	}

	return mimeTypes[enc], nil
}

func sendResponse(w http.ResponseWriter, req cmds.Request, res cmds.Response) {
	mime, err := guessMimeType(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	status := http.StatusOK
	// if response contains an error, write an HTTP error status code
	if e := res.Error(); e != nil {
		if e.Code == cmds.ErrClient {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}
		// NOTE: The error will actually be written out by the reader below
	}

	out, err := res.Reader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h := w.Header()
	if res.Length() > 0 {
		h.Set(contentLengthHeader, strconv.FormatUint(res.Length(), 10))
	}

	if _, ok := res.Output().(io.Reader); ok {
		mime = ""
		h.Set(streamHeader, "1")
	}

	// if output is a channel and user requested streaming channels,
	// use chunk copier for the output
	_, isChan := res.Output().(chan interface{})
	if !isChan {
		_, isChan = res.Output().(<-chan interface{})
	}

	streamChans, _, _ := req.Option("stream-channels").Bool()
	if isChan {
		h.Set(channelHeader, "1")
		if streamChans {
			// streaming output from a channel will always be json objects
			mime = applicationJson
		}
	}

	if mime != "" {
		h.Set(contentTypeHeader, mime)
	}
	h.Set(transferEncodingHeader, "chunked")

	if err := writeResponse(status, w, out); err != nil {
		log.Error("error while writing stream", err)
	}
}

// Copies from an io.Reader to a http.ResponseWriter.
// Flushes chunks over HTTP stream as they are read (if supported by transport).
func writeResponse(status int, w http.ResponseWriter, out io.Reader) error {
	// hijack the connection so we can write our own chunked output and trailers
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Error("Failed to create hijacker! cannot continue!")
		return errors.New("Could not create hijacker")
	}
	conn, writer, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	defer conn.Close()

	// write status
	writer.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, http.StatusText(status)))

	// Write out headers
	w.Header().Write(writer)

	// end of headers
	writer.WriteString("\r\n")

	// write body
	streamErr := writeChunks(out, writer)

	// close body
	writer.WriteString("0\r\n")

	// if there was a stream error, write out an error trailer. hopefully
	// the client will pick it up!
	if streamErr != nil {
		writer.WriteString(StreamErrHeader + ": " + sanitizedErrStr(streamErr) + "\r\n")
	}
	writer.WriteString("\r\n") // close response
	writer.Flush()
	return streamErr
}

func writeChunks(r io.Reader, w *bufio.ReadWriter) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)

		if n > 0 {
			length := fmt.Sprintf("%x\r\n", n)
			w.WriteString(length)

			_, err := w.Write(buf[0:n])
			if err != nil {
				return err
			}

			w.WriteString("\r\n")
			w.Flush()
		}

		if err != nil && err != io.EOF {
			return err
		}
		if err == io.EOF {
			break
		}
	}
	return nil
}

func sanitizedErrStr(err error) string {
	s := err.Error()
	s = strings.Split(s, "\n")[0]
	s = strings.Split(s, "\r")[0]
	return s
}
