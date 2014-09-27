package daemon

import (
	"encoding/json"
	"io"
	"net"
	"os"

	ma "github.com/jbenet/go-multiaddr"
)

//SendCommand connects to the address on the network with a timeout and encodes the connection into JSON
func SendCommand(command *Command, server string) error {

	maddr, err := ma.NewMultiaddr(server)
	if err != nil {
		return err
	}

	network, host, err := maddr.DialArgs()
	if err != nil {
		return err
	}

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
