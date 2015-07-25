package http

import (
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
	contentTypeHeader      = "Content-Type"
	contentLengthHeader    = "Content-Length"
	transferEncodingHeader = "Transfer-Encoding"
	applicationJson        = "application/json"
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
		err = fmt.Errorf("cmds/http: couldn't GetNode(): %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

func sendResponse(w http.ResponseWriter, req cmds.Request, res cmds.Response) {

	var mime string
	if _, ok := res.Output().(io.Reader); ok {
		mime = ""
		// we don't set the Content-Type for streams, so that browsers can MIME-sniff the type themselves
		// we set this header so clients have a way to know this is an output stream
		// (not marshalled command output)
		// TODO: set a specific Content-Type if the command response needs it to be a certain type
	} else {
		// Try to guess mimeType from the encoding option
		enc, found, err := res.Request().Option(cmds.EncShort).String()
		if err != nil || !found {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mime = mimeTypes[enc]
	}

	status := 200
	// if response contains an error, write an HTTP error status code
	if e := res.Error(); e != nil {
		if e.Code == cmds.ErrClient {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}
		// TODO: do we just ignore this error? or what?
	}

	out, err := res.Reader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// if output is a channel and user requested streaming channels,
	// use chunk copier for the output
	_, isChan := res.Output().(chan interface{})
	if !isChan {
		_, isChan = res.Output().(<-chan interface{})
	}

	streamChans, _, _ := req.Option("stream-channels").Bool()
	if isChan && streamChans {
		// streaming output from a channel will always be json objects
		mime = applicationJson
	}

	if err := copyChunks(mime, status, isChan, res.Length(), w, out); err != nil {
		log.Error("error while writing stream", err)
	}
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Call the CORS handler which wraps the internal handler.
	i.corsHandler.ServeHTTP(w, r)
}

// Copies from an io.Reader to a http.ResponseWriter.
// Flushes chunks over HTTP stream as they are read (if supported by transport).
func copyChunks(contentType string, status int, channel bool, length uint64, w http.ResponseWriter, out io.Reader) error {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return errors.New("Could not create hijacker")
	}
	conn, writer, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	defer conn.Close()

	writer.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, http.StatusText(status)))
	writer.WriteString(streamHeader + ": 1\r\n")
	if contentType != "" {
		writer.WriteString(contentTypeHeader + ": " + contentType + "\r\n")
	}
	if channel {
		writer.WriteString(channelHeader + ": 1\r\n")
	}
	if length > 0 {
		w.Header().Set(contentLengthHeader, strconv.FormatUint(length, 10))
	}
	writer.WriteString(transferEncodingHeader + ": chunked\r\n")

	writer.WriteString("\r\n")

	writeChunks := func() error {
		buf := make([]byte, 32*1024)
		for {
			n, err := out.Read(buf)

			if n > 0 {
				length := fmt.Sprintf("%x\r\n", n)
				writer.WriteString(length)

				_, err := writer.Write(buf[0:n])
				if err != nil {
					return err
				}

				writer.WriteString("\r\n")
				writer.Flush()
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

	streamErr := writeChunks()
	writer.WriteString("0\r\n") // close body

	// if there was a stream error, write out an error trailer. hopefully
	// the client will pick it up!
	if streamErr != nil {
		writer.WriteString(StreamErrHeader + ": " + sanitizedErrStr(streamErr) + "\r\n")
	}
	writer.WriteString("\r\n") // close response
	writer.Flush()
	return streamErr
}

func sanitizedErrStr(err error) string {
	s := err.Error()
	s = strings.Split(s, "\n")[0]
	s = strings.Split(s, "\r")[0]
	return s
}

type flushResponse struct {
	W http.ResponseWriter
}

func (fr *flushResponse) Write(buf []byte) (int, error) {
	n, err := fr.W.Write(buf)
	if err != nil {
		return n, err
	}

	if flusher, ok := fr.W.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}
