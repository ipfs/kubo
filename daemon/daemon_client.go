package daemon

import (
	"encoding/json"
	"io"
	"net"
	"os"
	"time"
)

func SendCommand(com *Command, server string) error {
	con, err := net.DialTimeout("tcp", server, time.Millisecond*300)
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
