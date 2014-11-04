package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

const (
	ApiUrlFormat = "http://%s%s/%s?%s"
	ApiPath      = "/api/v0" // TODO: make configurable
)

// Client is the commands HTTP client interface.
type Client interface {
	Send(req cmds.Request) (cmds.Response, error)
}

type client struct {
	serverAddress string
}

func NewClient(address string) Client {
	return &client{address}
}

func (c *client) Send(req cmds.Request) (cmds.Response, error) {
	var userEncoding string
	if enc, found := req.Option(cmds.EncShort); found {
		userEncoding = enc.(string)
		req.SetOption(cmds.EncShort, cmds.JSON)
	} else {
		enc, _ := req.Option(cmds.EncLong)
		userEncoding = enc.(string)
		req.SetOption(cmds.EncLong, cmds.JSON)
	}

	query, inputStream, err := getQuery(req)
	if err != nil {
		return nil, err
	}

	path := strings.Join(req.Path(), "/")
	url := fmt.Sprintf(ApiUrlFormat, c.serverAddress, ApiPath, path, query)

	httpRes, err := http.Post(url, "application/octet-stream", inputStream)
	if err != nil {
		return nil, err
	}

	res, err := getResponse(httpRes, req)
	if err != nil {
		return nil, err
	}

	if len(userEncoding) > 0 {
		req.SetOption(cmds.EncShort, userEncoding)
		req.SetOption(cmds.EncLong, userEncoding)
	}

	return res, nil
}

func getQuery(req cmds.Request) (string, io.Reader, error) {
	// TODO: handle multiple files with multipart
	var inputStream io.Reader

	query := url.Values{}
	for k, v := range req.Options() {
		query.Set(k, v.(string))
	}

	args := req.Arguments()
	argDefs := req.Command().Arguments
	var argDef cmds.Argument

	for i, arg := range args {
		if i < len(argDefs) {
			argDef = argDefs[i]
		}

		if argDef.Type == cmds.ArgString {
			query.Add("arg", arg.(string))

		} else {
			// TODO: multipart
			if inputStream != nil {
				return "", nil, fmt.Errorf("Currently, only one file stream is possible per request")
			}
			inputStream = arg.(io.Reader)
		}
	}

	return query.Encode(), inputStream, nil
}

// getResponse decodes a http.Response to create a cmds.Response
func getResponse(httpRes *http.Response, req cmds.Request) (cmds.Response, error) {
	var err error
	res := cmds.NewResponse(req)

	contentType := httpRes.Header["Content-Type"][0]
	contentType = strings.Split(contentType, ";")[0]

	if contentType == "application/octet-stream" {
		res.SetOutput(httpRes.Body)
		return res, nil
	}

	dec := json.NewDecoder(httpRes.Body)

	if httpRes.StatusCode >= http.StatusBadRequest {
		e := cmds.Error{}

		if httpRes.StatusCode == http.StatusNotFound {
			// handle 404s
			e.Message = "Command not found."
			e.Code = cmds.ErrClient

		} else if contentType == "text/plain" {
			// handle non-marshalled errors
			buf := bytes.NewBuffer(nil)
			io.Copy(buf, httpRes.Body)
			e.Message = string(buf.Bytes())
			e.Code = cmds.ErrNormal

		} else {
			// handle marshalled errors
			err = dec.Decode(&e)
			if err != nil {
				return nil, err
			}
		}

		res.SetError(e, e.Code)

	} else {
		v := req.Command().Type
		err = dec.Decode(&v)
		if err != nil {
			return nil, err
		}

		res.SetOutput(v)
	}

	return res, nil
}
