package daemon

import (
	"encoding/json"
	"net"
)

func SendCommand(command, server string) (string, error) {
	con, err := net.Dial("tcp", server)
	if err != nil {
		return "", err
	}

	enc := json.NewEncoder(con)
	err = enc.Encode(command)
	if err != nil {
		return "", err
	}

	dec := json.NewDecoder(con)

	var resp string
	err = dec.Decode(&resp)
	if err != nil {
		return "", err
	}

	return resp, nil
}
