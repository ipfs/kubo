package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net"

	core "github.com/jbenet/go-ipfs/core"
	commands "github.com/jbenet/go-ipfs/core/commands"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

//DaemonListener listens to an initialized IPFS node and can send it commands instead of
//starting up a new set of connections
type DaemonListener struct {
	node   *core.IpfsNode
	list   net.Listener
	closed bool
}

//Command accepts user input and can be sent to the running IPFS node
type Command struct {
	Command string
	Args    []string
	Opts    map[string]interface{}
}

func NewDaemonListener(ipfsnode *core.IpfsNode, addr string) (*DaemonListener, error) {
	list, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	fmt.Println("New daemon listener initialized.")

	return &DaemonListener{
		node: ipfsnode,
		list: list,
	}, nil
}

func NewCommand() *Command {
	return &Command{
		Opts: make(map[string]interface{}),
	}
}

func (dl *DaemonListener) Listen() {
	fmt.Println("listen.")
	for {
		conn, err := dl.list.Accept()
		fmt.Println("Loop!")
		if err != nil {
			if !dl.closed {
				u.PErr("DaemonListener Accept: %v\n", err)
			}
			return
		}
		go dl.handleConnection(conn)
	}
}

func (dl *DaemonListener) handleConnection(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)

	var command Command
	err := dec.Decode(&command)
	if err != nil {
		fmt.Fprintln(conn, err)
		return
	}

	u.DOut("Got command: %v\n", command)
	err := ExecuteCommand(&command, dl.node, conn)
	if err != nil {
		fmt.Fprintln(conn, "%v\n", err)
	}
}

func ExecuteCommand(com *Command, ipfsnode *core.IpfsNode, out io.Writer) error {
	u.DOut("executing command: %s\n", com.Command)
	switch com.Command {
	case "add":
		depth := 1
		if r, ok := com.Opts["r"].(bool); r && ok {
			depth = -1
		}
		for _, path := range com.Args {
			nd, err := commands.AddPath(ipfsnode, path, depth)
			if err != nil {
				return fmt.Errorf("addFile error: %v", err)
			}

			k, err := nd.Key()
			if err != nil {
				return fmt.Errorf("addFile error: %v", err)
			}

			fmt.Fprintf(out, "Added node: %s = %s\n", path, k.Pretty())
		}
	case "cat":
		for _, fn := range com.Args {
			dagnode, err := ipfsnode.Resolver.ResolvePath(fn)
			if err != nil {
				return fmt.Errorf("catFile error: %v", err)
			}

			read, err := dag.NewDagReader(dagnode, ipfsnode.DAG)
			if err != nil {
				return fmt.Errorf("cat error: %v", err)
			}

			_, err = io.Copy(out, read)
			if err != nil {
				return fmt.Errorf("cat error: %v", err)
			}
		}
	case "ls":
		for _, fn := range com.Args {
			dagnode, err := ipfsnode.Resolver.ResolvePath(fn)
			if err != nil {
				return fmt.Errorf("ls error: %v", err)
			}

			for _, link := range dagnode.Links {
				fmt.Fprintf(out, "%s %d %s\n", link.Hash.B58String(), link.Size, link.Name)
			}
		}
	case "pin":
		for _, fn := range com.Args {
			dagnode, err := ipfsnode.Resolver.ResolvePath(fn)
			if err != nil {
				return fmt.Errorf("pin error: %v", err)
			}

			err = ipfsnode.PinDagNode(dagnode)
			if err != nil {
				return fmt.Errorf("pin: %v", err)
			}
		}
	default:
		return fmt.Errord("Invalid Command: '%s'", com.Command)
	}
}

func (dl *DaemonListener) Close() error {
	dl.closed = true
	return dl.list.Close()
}
