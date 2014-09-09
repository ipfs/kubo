package daemon

import (
	"encoding/json"
	"io"
	"net"
	"os"
)

func SendCommand(com *Command, server string) error {
	con, err := net.Dial("tcp", server)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(con)
	err = enc.Encode(com)
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, con)

	return nil
}
