package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

const USAGE = "ma-pipe-unidir [-l|--listen] [--pidFile=path] [-h|--help] <send|recv> <multiaddr>\n"

type Opts struct {
	Listen  bool
	PidFile string
}

func app() int {
	opts := Opts{}
	flag.BoolVar(&opts.Listen, "l", false, "")
	flag.BoolVar(&opts.Listen, "listen", false, "")
	flag.StringVar(&opts.PidFile, "pidFile", "", "")
	flag.Usage = func() {
		fmt.Print(USAGE)
	}
	flag.Parse()
	args := flag.Args()

	if len(args) < 2 { // <mode> <addr>
		fmt.Print(USAGE)
		return 1
	}

	mode := args[0]
	addr := args[1]

	if mode != "send" && mode != "recv" {
		fmt.Print(USAGE)
		return 1
	}

	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return 1
	}

	var conn manet.Conn

	if opts.Listen {
		listener, err := manet.Listen(maddr)
		if err != nil {
			return 1
		}

		if len(opts.PidFile) > 0 {
			data := []byte(strconv.Itoa(os.Getpid()))
			err := ioutil.WriteFile(opts.PidFile, data, 0644)
			if err != nil {
				return 1
			}

			defer os.Remove(opts.PidFile)
		}

		conn, err = listener.Accept()
		if err != nil {
			return 1
		}
	} else {
		var err error
		conn, err = manet.Dial(maddr)
		if err != nil {
			return 1
		}

		if len(opts.PidFile) > 0 {
			data := []byte(strconv.Itoa(os.Getpid()))
			err := ioutil.WriteFile(opts.PidFile, data, 0644)
			if err != nil {
				return 1
			}

			defer os.Remove(opts.PidFile)
		}

	}

	defer conn.Close()
	switch mode {
	case "recv":
		_, err = io.Copy(os.Stdout, conn)
	case "send":
		_, err = io.Copy(conn, os.Stdin)
	default:
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

func main() {
	os.Exit(app())
}
