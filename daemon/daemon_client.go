package daemon

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"time"	
)


//connects to the address on the network with a timeout and encodes the connection into JSON
func SendCommand(command *Command, server string) error {
	
	conn, err := net.DialTimeout("tcp", server, time.Millisecond*300)
	
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
