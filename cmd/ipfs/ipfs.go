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
	args := os.Args[1:]

	req, err := cli.Parse(args, Root)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(req.Path()) == 0 {
		req, err = cli.Parse(args, commands.Root)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	var local bool // TODO: option to force local
	var root *cmds.Command
	cmd, err := Root.Get(req.Path())
	if err == nil {
		local = true
		root = Root

	} else if local {
		fmt.Println(err)
		os.Exit(1)

	} else {
		cmd, err = commands.Root.Get(req.Path())
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		local = false
		root = commands.Root
	}

	// TODO: get converted options so we can use them here (e.g. --debug, --config)

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

	var res cmds.Response
	if local {
		// TODO: spin up node
		res = root.Call(req)
	} else {
		res, err = sendRequest(req)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

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
