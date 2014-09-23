package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	u "github.com/jbenet/go-ipfs/util"
)

// ErrDaemonNotRunning is returned when attempting to retrieve the daemon's
// address and the daemon is not actually running.
var ErrDaemonNotRunning = errors.New("daemon not running")

func getDaemonAddr(confdir string) (string, error) {
	var err error
	confdir, err = u.TildeExpansion(confdir)
	if err != nil {
		return "", err
	}
	fi, err := os.Open(confdir + "/rpcaddress")
	if err != nil {
		log.Debug("getDaemonAddr failed: %s", err)
		if err == os.ErrNotExist {
			return "", ErrDaemonNotRunning
		}
		return "", err
	}

	read := bufio.NewReader(fi)

	// TODO: operating system agostic line delim
	line, err := read.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(line), nil
}

// SendCommand issues a command (of type daemon.Command) to the daemon, if it
// is running (if not, errors out). This is done over network RPC API. The
// address of the daemon is retrieved from the configuration directory, where
// live daemons write their addresses to special files.
func SendCommand(command *Command, confdir string) error {
	server, err := getDaemonAddr(confdir)
	if err != nil {
		return err
	}

	maddr, err := ma.NewMultiaddr(server)
	if err != nil {
		return err
	}

	network, host, err := maddr.DialArgs()

	conn, err := net.Dial(network, host)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(conn)
	err = enc.Encode(command)
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, conn)

	return nil
}
