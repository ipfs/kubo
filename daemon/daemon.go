package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	core "github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/core/commands"
	u "github.com/jbenet/go-ipfs/util"

	lock "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/camlistore/lock"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"
)

var log = u.Logger("daemon")

// LockFile is the filename of the daemon lock, relative to config dir
const LockFile = "daemon.lock"

// DaemonListener listens to an initialized IPFS node and can send it commands instead of
// starting up a new set of connections
type DaemonListener struct {
	node   *core.IpfsNode
	list   manet.Listener
	closed bool
	wg     sync.WaitGroup
	lk     io.Closer
}

//Command accepts user input and can be sent to the running IPFS node
type Command struct {
	Command string
	Args    []string
	Opts    map[string]interface{}
}

func NewDaemonListener(ipfsnode *core.IpfsNode, addr ma.Multiaddr, confdir string) (*DaemonListener, error) {
	var err error
	confdir, err = u.TildeExpansion(confdir)
	if err != nil {
		return nil, err
	}

	lk, err := daemonLock(confdir)
	if err != nil {
		return nil, err
	}

	ofi, err := os.Create(confdir + "/rpcaddress")
	if err != nil {
		log.Warning("Could not create rpcaddress file: %s", err)
		return nil, err
	}

	_, err = ofi.Write([]byte(addr.String()))
	if err != nil {
		log.Warning("Could not write to rpcaddress file: %s", err)
		return nil, err
	}
	ofi.Close()

	list, err := manet.Listen(addr)
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
	if dl.closed {
		panic("attempting to listen on a closed daemon Listener")
	}

	// add ourselves to workgroup. and remove ourselves when done.
	dl.wg.Add(1)
	defer dl.wg.Done()

	log.Info("daemon listening")
	for {
		conn, err := dl.list.Accept()
		if err != nil {
			if !dl.closed {
				log.Warning("DaemonListener Accept: %v", err)
			}
			return
		}
		go dl.handleConnection(conn)
	}
}

func (dl *DaemonListener) handleConnection(conn manet.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)

	var command Command
	err := dec.Decode(&command)
	if err != nil {
		fmt.Fprintln(conn, err)
		return
	}

	log.Debug("Got command: %v", command)
	switch command.Command {
	case "add":
		err = commands.Add(dl.node, command.Args, command.Opts, conn)
	case "cat":
		err = commands.Cat(dl.node, command.Args, command.Opts, conn)
	case "ls":
		err = commands.Ls(dl.node, command.Args, command.Opts, conn)
	case "pin":
		err = commands.Pin(dl.node, command.Args, command.Opts, conn)
	case "publish":
		err = commands.Publish(dl.node, command.Args, command.Opts, conn)
	case "resolve":
		err = commands.Resolve(dl.node, command.Args, command.Opts, conn)
	case "diag":
		err = commands.Diag(dl.node, command.Args, command.Opts, conn)
	case "blockGet":
		err = commands.BlockGet(dl.node, command.Args, command.Opts, conn)
	case "blockPut":
		err = commands.BlockPut(dl.node, command.Args, command.Opts, conn)
	case "log":
		err = commands.Log(dl.node, command.Args, command.Opts, conn)
	default:
		err = fmt.Errorf("Invalid Command: '%s'", command.Command)
	}
	if err != nil {
		log.Error("%s: %s", command.Command, err)
		fmt.Fprintln(conn, err)
	}
}

func (dl *DaemonListener) Close() error {
	dl.closed = true
	err := dl.list.Close()
	dl.wg.Wait() // wait till done before releasing lock.
	dl.lk.Close()
	return err
}

func daemonLock(confdir string) (io.Closer, error) {
	return lock.Lock(path.Join(confdir, LockFile))
}
