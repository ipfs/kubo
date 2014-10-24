package main

import (
	"fmt"
	"io"
	"os"
	"runtime/pprof"

	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsCli "github.com/jbenet/go-ipfs/commands/cli"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	"github.com/jbenet/go-ipfs/core/commands"
	u "github.com/jbenet/go-ipfs/util"
)

// log is the command logger
var log = u.Logger("cmd/ipfs")

func main() {
	args := os.Args[1:]

	req, err := cmdsCli.Parse(args, Root)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(req.Path()) == 0 {
		req, err = cmdsCli.Parse(args, commands.Root)
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
		res, err = cmdsHttp.Send(req)
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
