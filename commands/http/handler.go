package http

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	cors "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/rs/cors"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	cmds "github.com/ipfs/go-ipfs/commands"
	u "github.com/ipfs/go-ipfs/util"
)

var log = u.Logger("commands/http")

// the internal handler for the API
type internalHandler struct {
	ctx  cmds.Context
	root *cmds.Command
	cfg  *ServerConfig
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
	originHeader           = "origin"
)

const (
	ACAOrigin      = "Access-Control-Allow-Origin"
	ACAMethods     = "Access-Control-Allow-Methods"
	ACACredentials = "Access-Control-Allow-Credentials"
)

var mimeTypes = map[string]string{
	cmds.JSON: "application/json",
	cmds.XML:  "application/xml",
	cmds.Text: "text/plain",
}

type ServerConfig struct {
	// Headers is an optional map of headers that is written out.
	Headers map[string][]string

	// CORSOpts is a set of options for CORS headers.
	CORSOpts *cors.Options
}

func skipAPIHeader(h string) bool {
	switch h {
	case "Access-Control-Allow-Origin":
		return true
	case "Access-Control-Allow-Methods":
		return true
	case "Access-Control-Allow-Credentials":
		return true
	default:
		return false
	}
}

func NewHandler(ctx cmds.Context, root *cmds.Command, cfg *ServerConfig) *Handler {
	if cfg == nil {
		panic("must provide a valid ServerConfig")
	}

	// Wrap the internal handler with CORS handling-middleware.
	// Create a handler for the API.
	internal := internalHandler{ctx, root, cfg}
	c := cors.New(*cfg.CORSOpts)
	return &Handler{internal, c.Handler(internal)}
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Call the CORS handler which wraps the internal handler.
	i.corsHandler.ServeHTTP(w, r)
}

func (i internalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Incoming API request: ", r.URL)

	if !allowOrigin(r, i.cfg) || !allowReferer(r, i.cfg) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 - Forbidden"))
		log.Warningf("API blocked request to %s. (possible CSRF)", r.URL)
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

	ctx, cancel := context.WithCancel(node.Context())
	defer cancel()

	err = req.SetRootContext(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// call the command
	res := i.root.Call(req)

	// set user's headers first.
	for k, v := range i.cfg.Headers {
		if !skipAPIHeader(k) {
			w.Header()[k] = v
		}
	}

	// now handle responding to the client properly
	sendResponse(w, r, res, req)
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

func sendResponse(w http.ResponseWriter, r *http.Request, res cmds.Response, req cmds.Request) {
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
		// we don't set the Content-Type for streams, so that browsers can MIME-sniff the type themselves
		// we set this header so clients have a way to know this is an output stream
		// (not marshalled command output)
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

	if r.Method == "HEAD" { // after all the headers.
		return
	}

	if err := writeResponse(status, w, out); err != nil {
		if strings.Contains(err.Error(), "broken pipe") {
			log.Info("client disconnect while writing stream ", err)
			return
		}

		log.Error("error while writing stream ", err)
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

// allowOrigin just stops the request if the origin is not allowed.
// the CORS middleware apparently does not do this for us...
func allowOrigin(r *http.Request, cfg *ServerConfig) bool {
	origin := r.Header.Get("Origin")

	// curl, or ipfs shell, typing it in manually, or clicking link
	// NOT in a browser. this opens up a hole. we should close it,
	// but right now it would break things. TODO
	if origin == "" {
		return true
	}

	for _, o := range cfg.CORSOpts.AllowedOrigins {
		if o == "*" { // ok! you asked for it!
			return true
		}

		if o == origin { // allowed explicitly
			return true
		}
	}

	return false
}

// allowReferer this is here to prevent some CSRF attacks that
// the API would be vulnerable to. We check that the Referer
// is allowed by CORS Origin (origins and referrers here will
// work similarly in the normla uses of the API).
// See discussion at https://github.com/ipfs/go-ipfs/issues/1532
func allowReferer(r *http.Request, cfg *ServerConfig) bool {
	referer := r.Referer()

	// curl, or ipfs shell, typing it in manually, or clicking link
	// NOT in a browser. this opens up a hole. we should close it,
	// but right now it would break things. TODO
	if referer == "" {
		return true
	}

	u, err := url.Parse(referer)
	if err != nil {
		// bad referer. but there _is_ something, so bail.
		log.Debug("failed to parse referer: ", referer)
		// debug because referer comes straight from the client. dont want to
		// let people DOS by putting a huge referer that gets stored in log files.
		return false
	}
	origin := u.Scheme + "://" + u.Host

	// check CORS ACAOs and pretend Referer works like an origin.
	// this is valid for many (most?) sane uses of the API in
	// other applications, and will have the desired effect.
	for _, o := range cfg.CORSOpts.AllowedOrigins {
		if o == "*" { // ok! you asked for it!
			return true
		}

		// referer is allowed explicitly
		if o == origin {
			return true
		}
	}

	return false
}
