package daemon

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"time"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

//SendCommand connects to the address on the network with a timeout and encodes the connection into JSON
func SendCommand(command *Command, server *ma.Multiaddr) error {
	network, host, err := server.DialArgs()
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout(network, host, time.Millisecond*300)

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
