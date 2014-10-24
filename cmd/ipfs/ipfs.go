package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/commands/cli"
	"github.com/jbenet/go-ipfs/core/commands"
	u "github.com/jbenet/go-ipfs/util"
)

// log is the command logger
var log = u.Logger("cmd/ipfs")

const API_PATH = "/api/v0"

func main() {
	req, err := cli.Parse(os.Args[1:], commands.Root)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// TODO: call command locally if option tells us to, or if command is CLI-only (e.g. ipfs init)

	cmd, err := commands.Root.Get(req.Path())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	res, err := sendRequest(req)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if debug, ok := req.Option("debug"); ok && debug.(bool) {
		u.Debug = true

		// if debugging, setup profiling.
		if u.Debug {
			ofi, err := os.Create("cpu.prof")
			if err != nil {
				fmt.Println(err)
				return
			}
			pprof.StartCPUProfile(ofi)
			defer ofi.Close()
			defer pprof.StopCPUProfile()
		}
	}

	//res := commands.Root.Call(req)

	if res.Error() != nil {
		fmt.Println(res.Error().Error())

		if cmd.Help != "" && res.Error().Code == cmds.ErrClient {
			// TODO: convert from markdown to ANSI terminal format?
			fmt.Println(cmd.Help)
		}

		os.Exit(1)
	}

	_, err = io.Copy(os.Stdout, res)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func sendRequest(req cmds.Request) (cmds.Response, error) {
	// TODO: load RPC host from config
	url := "http://localhost:8080" + API_PATH
	url += "/" + strings.Join(req.Path(), "/")

	// TODO: support other encodings once we have multicodec to decode response
	//       (we shouldn't have to set this here)
	encoding := cmds.JSON
	req.SetOption(cmds.EncShort, encoding)

	query := "?"
	options := req.Options()
	for k, v := range options {
		query += "&" + k + "=" + v.(string)
	}

	httpRes, err := http.Post(url+query, "application/octet-stream", req.Stream())
	if err != nil {
		return nil, err
	}

	res := cmds.NewResponse(req)

	contentType := httpRes.Header["Content-Type"][0]
	contentType = strings.Split(contentType, ";")[0]

	if contentType == "application/octet-stream" {
		res.SetValue(httpRes.Body)
		return res, nil
	}

	// TODO: decode based on `encoding`, using multicodec
	dec := json.NewDecoder(httpRes.Body)

	if httpRes.StatusCode >= http.StatusBadRequest {
		e := cmds.Error{}
		err = dec.Decode(&e)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		res.SetError(e, e.Code)

	} else {
		var v interface{}
		err = dec.Decode(&v)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		res.SetValue(v)
	}

	return res, nil
}
