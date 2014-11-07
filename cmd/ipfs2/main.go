package main

import (
	"fmt"
	"io"
	"os"
	"runtime/pprof"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	cmds "github.com/jbenet/go-ipfs/commands"
	cmdsCli "github.com/jbenet/go-ipfs/commands/cli"
	cmdsHttp "github.com/jbenet/go-ipfs/commands/http"
	"github.com/jbenet/go-ipfs/config"
	"github.com/jbenet/go-ipfs/core"
	commands "github.com/jbenet/go-ipfs/core/commands2"
	daemon "github.com/jbenet/go-ipfs/daemon2"
	u "github.com/jbenet/go-ipfs/util"
)

// log is the command logger
var log = u.Logger("cmd/ipfs")

const heapProfile = "ipfs.mprof"

func main() {
	args := os.Args[1:]
	req, root := createRequest(args)
	handleOptions(req, root)
	res := callCommand(req, root)
	outputResponse(res)

	if u.Debug {
		err := writeHeapProfileToFile()
		if err != nil {
			log.Critical(err)
		}
	}
}

func createRequest(args []string) (cmds.Request, *cmds.Command) {
	req, root, cmd, err := cmdsCli.Parse(args, Root, commands.Root)
	if err != nil {
		fmt.Println(err)
		if cmd != nil {
			if cmd.Help != "" {
				fmt.Println(cmd.Help)
			}
		} else {
			fmt.Println(Root.Help)
		}
		os.Exit(1)
	}

	options, err := getOptions(req, root)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	configPath, err := getConfigRoot(options)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	conf, err := getConfig(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx := req.Context()
	ctx.ConfigRoot = configPath
	ctx.Config = conf

	if _, found := options.Option("encoding"); !found {
		if req.Command().Marshallers != nil && req.Command().Marshallers[cmds.Text] != nil {
			req.SetOption("encoding", cmds.Text)
		} else {
			req.SetOption("encoding", cmds.JSON)
		}
	}

	return req, root
}

func handleOptions(req cmds.Request, root *cmds.Command) {
	options, err := getOptions(req, root)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if help, found := options.Option("help"); found {
		if helpBool, ok := help.(bool); helpBool && ok {
			fmt.Println(req.Command().Help)
			os.Exit(0)
		} else if !ok {
			fmt.Println("error: expected 'help' option to be a bool")
			os.Exit(1)
		}
	}

	if debug, found := options.Option("debug"); found {
		if debugBool, ok := debug.(bool); debugBool && ok {
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
		} else if !ok {
			fmt.Println("error: expected 'debug' option to be a bool")
			os.Exit(1)
		}
	}
}

func callCommand(req cmds.Request, root *cmds.Command) cmds.Response {
	var res cmds.Response

	if root == Root {
		res = root.Call(req)

	} else {
		options, err := getOptions(req, root)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var found bool
		var local interface{}
		localBool := false
		if local, found = options.Option("local"); found {
			var ok bool
			localBool, ok = local.(bool)
			if !ok {
				fmt.Println("error: expected 'local' option to be a bool")
				os.Exit(1)
			}
		}

		if (!found || !localBool) && daemon.Locked(req.Context().ConfigRoot) {
			addr, err := ma.NewMultiaddr(req.Context().Config.Addresses.API)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			_, host, err := manet.DialArgs(addr)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			client := cmdsHttp.NewClient(host)

			res, err = client.Send(req)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

		} else {
			node, err := core.NewIpfsNode(req.Context().Config, false)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			defer node.Close()
			req.Context().Node = node

			res = root.Call(req)
		}
	}

	return res
}

func outputResponse(res cmds.Response) {
	if res.Error() != nil {
		fmt.Println(res.Error().Error())

		if res.Request().Command().Help != "" && res.Error().Code == cmds.ErrClient {
			// TODO: convert from markdown to ANSI terminal format?
			fmt.Println(res.Request().Command().Help)
		}

		os.Exit(1)
	}

	out, err := res.Reader()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	io.Copy(os.Stdout, out)
}

func getOptions(req cmds.Request, root *cmds.Command) (cmds.Request, error) {
	tempReq := cmds.NewRequest(req.Path(), req.Options(), nil, nil)

	options, err := root.GetOptions(tempReq.Path())
	if err != nil {
		return nil, err
	}

	err = tempReq.ConvertOptions(options)
	if err != nil {
		return nil, err
	}

	return tempReq, nil
}

func getConfigRoot(req cmds.Request) (string, error) {
	if opt, found := req.Option("config"); found {
		if optStr, ok := opt.(string); ok {
			return optStr, nil
		} else {
			return "", fmt.Errorf("Expected 'config' option to be a string")
		}
	}

	configPath, err := config.PathRoot()
	if err != nil {
		return "", err
	}
	return configPath, nil
}

func getConfig(path string) (*config.Config, error) {
	configFile, err := config.Filename(path)
	if err != nil {
		return nil, err
	}

	return config.Load(configFile)
}

func writeHeapProfileToFile() error {
	mprof, err := os.Create(heapProfile)
	if err != nil {
		log.Fatal(err)
	}
	defer mprof.Close()
	return pprof.WriteHeapProfile(mprof)
}
