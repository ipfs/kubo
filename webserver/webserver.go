package webserver

import (
	"io"
	"net/http"
	"net/url"

	core "github.com/jbenet/go-ipfs/core"
	mdag "github.com/jbenet/go-ipfs/merkledag"
)

// WebServer implementation
type WebServer struct {
	Ipfs *core.IpfsNode
}

var _ http.Handler = (*WebServer)(nil);

func NewWebServer(ipfs *core.IpfsNode) *WebServer {
	ws := new(WebServer)
	ws.Ipfs = ipfs;
	return ws
}

func SendError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	w.Write([]byte(err.Error()))
}

func (ws WebServer) ServeHTTP (w http.ResponseWriter, req *http.Request) {
	err := ws.SendFile(w, req)
	if err != nil {
		SendError(w, 500, err)
	}
}

func (ws WebServer) SendFile (w http.ResponseWriter, req *http.Request) error {
	requestURL, err := url.Parse(req.RequestURI)
	if err != nil {
		return err
	}

	dagnode, err := ws.Ipfs.Resolver.ResolvePath(requestURL.Path)
	if err != nil {
		return err
	}

	r, err := mdag.NewDagReader(dagnode, ws.Ipfs.DAG)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, r)
	if err != nil {
		return err;
	}

	//io.WriteString(w, "hello, world " + requestURL.Path + "\n")

	return nil
}

