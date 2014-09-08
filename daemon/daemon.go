package daemon

import (
	"encoding/json"
	"errors"
	"net"
	"strings"

	u "github.com/jbenet/go-ipfs/util"
)

var ErrInvalidCommand = errors.New("invalid command")

type DaemonListener struct {
	list     net.Listener
	CommChan chan *Command
	closed   bool
}

func NewDaemonListener(addr string) (*DaemonListener, error) {
	list, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &DaemonListener{
		list:     list,
		CommChan: make(chan *Command),
	}, nil
}

type Command struct {
	command string
	args    []string
	resp    chan string
}

func (dl *DaemonListener) Listen() {
	for {
		c, err := dl.list.Accept()
		if err != nil {
			if !dl.closed {
				u.PErr("DaemonListener Accept: %v\n", err)
			}
			return
		}
		go dl.handleConnection(c)
	}
}

func (dl *DaemonListener) handleConnection(c net.Conn) {
	dec := json.NewDecoder(c)
	enc := json.NewEncoder(c)
	var com string
	err := dec.Decode(&com)
	if err != nil {
		err := enc.Encode(err.Error())
		if err != nil {
			u.PErr("DaemonListener decode: %v\n", err)
		}
		return
	}
	u.DOut("Got command: %v\n", com)

	cmd, err := parseCommand(com)
	if err != nil {
		err := enc.Encode(err.Error())
		if err != nil {
			u.PErr("DaemonListener parse: %v\n", err)
		}
		return
	}

	select {
	case dl.CommChan <- cmd:
	default:
		u.PErr("Recieved command after closing...")
		return
	}

	resp := <-cmd.resp
	err = enc.Encode(resp)
	if err != nil {
		u.PErr("handleConnection: %v\n", err)
	}
}

func parseCommand(cmdi string) (*Command, error) {
	params := strings.Split(cmdi, " ")
	if len(params) == 0 {
		return nil, ErrInvalidCommand
	}

	//TODO: some sort of validation here

	return &Command{
		command: params[0],
		args:    params[1:],
		resp:    make(chan string),
	}, nil
}

func (dl *DaemonListener) Close() error {
	dl.closed = true
	close(dl.CommChan)
	return dl.list.Close()
}
