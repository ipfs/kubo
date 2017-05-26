package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	manet "gx/ipfs/Qmf1Gq7N45Rpuw7ev47uWgH6dLPtdnvcMRNPkVBwqjLJg2/go-multiaddr-net"
)

const USAGE = "ma-pipe-unidir [-l|--listen] [-h|--help] <send|recv> <multiaddr>\n"

type Opts struct {
	Listen bool
}

func main() {
	opts := Opts{}
	flag.BoolVar(&opts.Listen, "l", false, "")
	flag.BoolVar(&opts.Listen, "listen", false, "")
	flag.Usage = func() {
		fmt.Print(USAGE)
	}
	flag.Parse()
	args := flag.Args()

	if len(args) < 2 { // <mode> <addr>
		fmt.Print(USAGE)
		return
	}

	mode := args[0]
	addr := args[1]

	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return
	}

	var conn manet.Conn

	if opts.Listen {
		listener, err := manet.Listen(maddr)
		if err != nil {
			return
		}

		conn, err = listener.Accept()
		if err != nil {
			return
		}
	} else {
		var err error
		conn, err = manet.Dial(maddr)
		if err != nil {
			return
		}
	}

	defer conn.Close()
	switch mode {
	case "recv":
		io.Copy(os.Stdout, conn)
	case "send":
		io.Copy(conn, os.Stdin)
	default:
		//TODO: a bit late
		fmt.Print(USAGE)
		return
	}
}
