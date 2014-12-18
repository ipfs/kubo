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
	config "github.com/jbenet/go-ipfs/config"
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

	// save user-provided encoding
	previousUserProvidedEncoding, found, err := req.Option(cmds.EncShort).String()
	if err != nil {
		return nil, err
	}

	// override with json to send to server
	req.SetOption(cmds.EncShort, cmds.JSON)

	query, err := getQuery(req)
	if err != nil {
		return nil, err
	}

	var fileReader *MultiFileReader
	var reader io.Reader

	if req.Files() != nil {
		fileReader = NewMultiFileReader(req.Files(), true)
		reader = fileReader
	} else {
		// if we have no file data, use an empty Reader
		// (http.NewRequest panics when a nil Reader is used)
		reader = strings.NewReader("")
	}

	path := strings.Join(req.Path(), "/")
	url := fmt.Sprintf(ApiUrlFormat, c.serverAddress, ApiPath, path, query)

	httpReq, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return nil, err
	}

	// TODO extract string consts?
	if fileReader != nil {
		httpReq.Header.Set("Content-Type", "multipart/form-data; boundary="+fileReader.Boundary())
		httpReq.Header.Set("Content-Disposition", "form-data: name=\"files\"")
	} else {
		httpReq.Header.Set("Content-Type", "application/octet-stream")
	}
	version := config.CurrentVersionNumber
	httpReq.Header.Set("User-Agent", fmt.Sprintf("/go-ipfs/%s/", version))

	httpRes, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// using the overridden JSON encoding in request
	res, err := getResponse(httpRes, req)
	if err != nil {
		return nil, err
	}

	if found && len(previousUserProvidedEncoding) > 0 {
		// reset to user provided encoding after sending request
		// NB: if user has provided an encoding but it is the empty string,
		// still leave it as JSON.
		req.SetOption(cmds.EncShort, previousUserProvidedEncoding)
	}

	return res, nil
}

func getQuery(req cmds.Request) (string, error) {
	query := url.Values{}
	for k, v := range req.Options() {
		str := fmt.Sprintf("%v", v)
		query.Set(k, str)
	}

	args := req.Arguments()
	argDefs := req.Command().Arguments

	argDefIndex := 0

	for _, arg := range args {
		argDef := argDefs[argDefIndex]
		// skip ArgFiles
		for argDef.Type == cmds.ArgFile {
			argDefIndex++
			argDef = argDefs[argDefIndex]
		}

		query.Add("arg", arg)

		if len(argDefs) > argDefIndex+1 {
			argDefIndex++
		}
	}

	return query.Encode(), nil
}

// getResponse decodes a http.Response to create a cmds.Response
func getResponse(httpRes *http.Response, req cmds.Request) (cmds.Response, error) {
	var err error
	res := cmds.NewResponse(req)

	contentType := httpRes.Header["Content-Type"][0]
	contentType = strings.Split(contentType, ";")[0]

	if len(httpRes.Header.Get(streamHeader)) > 0 {
		// if output is a stream, we can just use the body reader
		res.SetOutput(httpRes.Body)
		return res, nil

	} else if len(httpRes.Header.Get(channelHeader)) > 0 {
		// if output is coming from a channel, decode each chunk
		outChan := make(chan interface{})
		go func() {
			dec := json.NewDecoder(httpRes.Body)
			v := req.Command().Type

			for {
				err := dec.Decode(&v)
				if err != nil && err != io.EOF {
					fmt.Println(err.Error())
					return
				}
				if err == io.EOF {
					close(outChan)
					return
				}
				outChan <- v
			}
		}()

		res.SetOutput(outChan)
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
		if err != nil && err != io.EOF {
			return nil, err
		}

		res.SetOutput(v)
	}

	return res, nil
}
