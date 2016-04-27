package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	cli "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/codegangsta/cli"
	util "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/whyrusleeping/iptb/util"
)

func parseRange(s string) ([]int, error) {
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		ranges := strings.Split(s[1:len(s)-1], ",")
		var out []int
		for _, r := range ranges {
			rng, err := expandDashRange(r)
			if err != nil {
				return nil, err
			}

			out = append(out, rng...)
		}
		return out, nil
	} else {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}

		return []int{i}, nil
	}
}

func expandDashRange(s string) ([]int, error) {
	parts := strings.Split(s, "-")
	if len(parts) == 0 {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		return []int{i}, nil
	}
	low, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, err
	}

	hi, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}

	var out []int
	for i := low; i <= hi; i++ {
		out = append(out, i)
	}
	return out, nil
}

func handleErr(s string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, s, err)
		os.Exit(1)
	}
}

func main() {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		initCmd,
		startCmd,
		killCmd,
		restartCmd,
		shellCmd,
		getCmd,
		connectCmd,
		dumpStacksCmd,
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var initCmd = cli.Command{
	Name:  "init",
	Usage: "create and initialize testbed nodes",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "count, n",
			Usage: "number of ipfs nodes to initialize",
		},
		cli.IntFlag{
			Name:  "port, p",
			Usage: "port to start allocations from",
		},
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force initialization (overwrite existing configs)",
		},
		cli.BoolFlag{
			Name:  "mdns",
			Usage: "turn on mdns for nodes",
		},
		cli.StringFlag{
			Name:  "bootstrap",
			Usage: "select bootstrapping style for cluster",
			Value: "star",
		},
		cli.BoolFlag{
			Name:  "utp",
			Usage: "use utp for addresses",
		},
		cli.StringFlag{
			Name:  "cfg",
			Usage: "override default config with values from the given file",
		},
	},
	Action: func(c *cli.Context) {
		if c.Int("count") == 0 {
			fmt.Printf("please specify number of nodes: '%s init -n 10'\n", os.Args[0])
			os.Exit(1)
		}
		cfg := &util.InitCfg{
			Bootstrap: c.String("bootstrap"),
			Force:     c.Bool("f"),
			Count:     c.Int("count"),
			Mdns:      c.Bool("mdns"),
			Utp:       c.Bool("utp"),
			PortStart: c.Int("port"),
			Override:  c.String("cfg"),
		}

		err := util.IpfsInit(cfg)
		handleErr("ipfs init err: ", err)
	},
}

var startCmd = cli.Command{
	Name:  "start",
	Usage: "starts up all testbed nodes",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "wait",
			Usage: "wait for nodes to fully come online before returning",
		},
	},
	Action: func(c *cli.Context) {
		err := util.IpfsStart(c.Bool("wait"))
		handleErr("ipfs start err: ", err)
	},
}

var killCmd = cli.Command{
	Name:    "kill",
	Usage:   "kill a given node (or all nodes if none specified)",
	Aliases: []string{"stop"},
	Action: func(c *cli.Context) {
		if c.Args().Present() {
			i, err := strconv.Atoi(c.Args()[0])
			if err != nil {
				fmt.Println("failed to parse node number: ", err)
				os.Exit(1)
			}
			err = util.KillNode(i)
			if err != nil {
				fmt.Println("failed to kill node: ", err)
			}
			return
		}
		err := util.IpfsKillAll()
		handleErr("ipfs kill err: ", err)
	},
}

var restartCmd = cli.Command{
	Name:  "restart",
	Usage: "kill all nodes, then restart",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "wait",
			Usage: "wait for nodes to come online before returning",
		},
	},
	Action: func(c *cli.Context) {
		err := util.IpfsKillAll()
		handleErr("ipfs kill err: ", err)

		err = util.IpfsStart(c.Bool("wait"))
		handleErr("ipfs start err: ", err)
	},
}

var shellCmd = cli.Command{
	Name:  "shell",
	Usage: "execs your shell with certain environment variables set",
	Description: `Starts a new shell and sets some environment variables for you:

IPFS_PATH - set to testbed node 'n's IPFS_PATH
NODE[x] - set to the peer ID of node x
`,
	Action: func(c *cli.Context) {
		if !c.Args().Present() {
			fmt.Println("please specify which node you want a shell for")
			os.Exit(1)
		}
		n, err := strconv.Atoi(c.Args()[0])
		handleErr("parse err: ", err)

		err = util.IpfsShell(n)
		handleErr("ipfs shell err: ", err)
	},
}

var connectCmd = cli.Command{
	Name:  "connect",
	Usage: "connect two nodes together",
	Action: func(c *cli.Context) {
		if len(c.Args()) < 2 {
			fmt.Println("iptb connect [node] [node]")
			os.Exit(1)
		}

		from, err := parseRange(c.Args()[0])
		if err != nil {
			fmt.Printf("failed to parse: %s\n", err)
			return
		}

		to, err := parseRange(c.Args()[1])
		if err != nil {
			fmt.Printf("failed to parse: %s\n", err)
			return
		}

		for _, f := range from {
			for _, t := range to {
				err = util.ConnectNodes(f, t)
				if err != nil {
					fmt.Printf("failed to connect: %s\n", err)
					return
				}
			}
		}
	},
}

var getCmd = cli.Command{
	Name:  "get",
	Usage: "get an attribute of the given node",
	Action: func(c *cli.Context) {
		if len(c.Args()) < 2 {
			fmt.Println("iptb get [attr] [node]")
			os.Exit(1)
		}
		attr := c.Args().First()
		num, err := strconv.Atoi(c.Args()[1])
		handleErr("error parsing node number: ", err)

		val, err := util.GetAttr(attr, num)
		handleErr("error getting attribute: ", err)
		fmt.Println(val)
	},
}

var dumpStacksCmd = cli.Command{
	Name:  "dump-stack",
	Usage: "get a stack dump from the given daemon",
	Action: func(c *cli.Context) {
		if len(c.Args()) < 1 {
			fmt.Println("iptb dump-stack [node]")
			os.Exit(1)
		}

		num, err := strconv.Atoi(c.Args()[0])
		handleErr("error parsing node number: ", err)

		addr, err := util.GetNodesAPIAddr(num)
		handleErr("failed to get api addr: ", err)

		resp, err := http.Get("http://" + addr + "/debug/pprof/goroutine?debug=2")
		handleErr("GET stack dump failed: ", err)
		defer resp.Body.Close()

		io.Copy(os.Stdout, resp.Body)
	},
}
