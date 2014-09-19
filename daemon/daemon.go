package daemon

import (
	"encoding/json"
	"fmt"
	"net"

	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/core/commands"
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
	switch command.Command {
	case "add":
		err = commands.Add(dl.node, command.Args, command.Opts, conn)
	case "cat":
		err = commands.Cat(dl.node, command.Args, command.Opts, conn)
	case "ls":
		err = commands.Ls(dl.node, command.Args, command.Opts, conn)
	case "pin":
		err = commands.Pin(dl.node, command.Args, command.Opts, conn)
	case "peers":
		err = commands.Peers(dl.node, command.Args, command.Opts, conn)
	default:
		err = fmt.Errorf("Invalid Command: '%s'", command.Command)
	}
	if err != nil {
		fmt.Fprintln(conn, err)
	}
}

func (dl *DaemonListener) Close() error {
	dl.closed = true
	return dl.list.Close()
}
