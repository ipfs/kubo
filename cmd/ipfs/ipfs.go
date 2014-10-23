package main

import (
	"fmt"
	"io"
	"os"
	"runtime/pprof"

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

	cmd, err := commands.Root.Get(req.Path())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// TODO: send request to daemon via HTTP API

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

	res := commands.Root.Call(req)

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
