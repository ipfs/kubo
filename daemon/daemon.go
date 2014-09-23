package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/core/commands"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/op/go-logging"

	"github.com/camlistore/lock"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

var log = logging.MustGetLogger("daemon")

// DaemonListener listens to an initialized IPFS node and can send it commands instead of
// starting up a new set of connections
type DaemonListener struct {
	node   *core.IpfsNode
	list   net.Listener
	closed bool
	lk     io.Closer
}

//Command accepts user input and can be sent to the running IPFS node
type Command struct {
	Command string
	Args    []string
	Opts    map[string]interface{}
}

func NewDaemonListener(ipfsnode *core.IpfsNode, addr *ma.Multiaddr, confdir string) (*DaemonListener, error) {
	var err error
	confdir, err = u.TildeExpansion(confdir)
	if err != nil {
		return nil, err
	}

	lk, err := lock.Lock(confdir + "/daemon.lock")
	if err != nil {
		return nil, err
	}

	network, host, err := addr.DialArgs()
	if err != nil {
		return nil, err
	}

	ofi, err := os.Create(confdir + "/rpcaddress")
	if err != nil {
		log.Warning("Could not create rpcaddress file: %s", err)
		return nil, err
	}

	_, err = ofi.Write([]byte(host))
	if err != nil {
		log.Warning("Could not write to rpcaddress file: %s", err)
		return nil, err
	}
	ofi.Close()

	list, err := net.Listen(network, host)
	if err != nil {
		return nil, err
	}
	log.Info("New daemon listener initialized.")

	return &DaemonListener{
		node: ipfsnode,
		list: list,
		lk:   lk,
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
			dl.lk.Close()
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
