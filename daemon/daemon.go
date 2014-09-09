package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	core "github.com/jbenet/go-ipfs/core"
	commands "github.com/jbenet/go-ipfs/core/commands"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

var ErrInvalidCommand = errors.New("invalid command")

type DaemonListener struct {
	node   *core.IpfsNode
	list   net.Listener
	closed bool
}

func NewDaemonListener(node *core.IpfsNode, addr string) (*DaemonListener, error) {
	list, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	fmt.Println("new daemon listener.")

	return &DaemonListener{
		node: node,
		list: list,
	}, nil
}

type Command struct {
	Command string
	Args    []string
	Opts    map[string]interface{}
}

func NewCommand() *Command {
	return &Command{
		Opts: make(map[string]interface{}),
	}
}

func (dl *DaemonListener) Listen() {
	fmt.Println("listen.")
	for {
		c, err := dl.list.Accept()
		fmt.Println("Loop!")
		if err != nil {
			if !dl.closed {
				u.PErr("DaemonListener Accept: %v\n", err)
			}
			return
		}
		go dl.handleConnection(c)
	}
}

func (dl *DaemonListener) handleConnection(c net.Conn) {
	defer c.Close()

	dec := json.NewDecoder(c)

	var com Command
	err := dec.Decode(&com)
	if err != nil {
		fmt.Fprintln(c, err)
		return
	}

	u.DOut("Got command: %v\n", com)
	ExecuteCommand(&com, dl.node, c)
}

func ExecuteCommand(com *Command, n *core.IpfsNode, out io.Writer) {
	u.DOut("executing command: %s\n", com.Command)
	switch com.Command {
	case "add":
		depth := 1
		if r, ok := com.Opts["r"].(bool); r && ok {
			depth = -1
		}
		for _, path := range com.Args {
			_, err := commands.AddPath(n, path, depth)
			if err != nil {
				fmt.Fprintf(out, "addFile error: %v\n", err)
				continue
			}
		}
	case "cat":
		for _, fn := range com.Args {
			nd, err := n.Resolver.ResolvePath(fn)
			if err != nil {
				fmt.Fprintf(out, "catFile error: %v\n", err)
				return
			}

			read, err := dag.NewDagReader(nd, n.DAG)
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}

			_, err = io.Copy(out, read)
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
		}
	case "ls":
		for _, fn := range com.Args {
			nd, err := n.Resolver.ResolvePath(fn)
			if err != nil {
				fmt.Fprintf(out, "ls: %v\n", err)
				return
			}

			for _, link := range nd.Links {
				fmt.Fprintf(out, "%s %d %s\n", link.Hash.B58String(), link.Size, link.Name)
			}
		}
	default:
		fmt.Fprintf(out, "Invalid Command: '%s'\n", com.Command)
	}
}

func (dl *DaemonListener) Close() error {
	dl.closed = true
	return dl.list.Close()
}
