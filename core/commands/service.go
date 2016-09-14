package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"os"
	exec "os/exec"

	cmds "github.com/ipfs/go-ipfs/commands"
	corenet "github.com/ipfs/go-ipfs/core/corenet"
	peer "gx/ipfs/QmQGwpJy9P4yXZySmqkZEXCmbBpJUb8xntCv8Ca4taZwDC/go-libp2p-peer"
)

var ServiceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remote services over IPFS",
	},
	Subcommands: map[string]*cmds.Command{
		"listen": ListenCmd,
		"dial":   DialCmd,
	},
}

var ListenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Listen for connections",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("Name", true, false, "Name of service to listen on"),
		cmds.StringArg("Command", false, true, "Command to expose"),
	},
	Options: []cmds.Option{
		cmds.StringOption("auth", "a", "reserved for future authentication or authorization"),
	},
	PreRun: func(req cmds.Request) error {
		if req.Option("auth").Found() {
			return fmt.Errorf("--auth not implemented")
		}

		return nil
	},
	Interact: interactWithRemote,
	Run: func(req cmds.Request, res cmds.Response) {
		/*
		 *                                /-> channel 0 (close, etc)
		 *  req.Stdin -> unpack channel --
		 *                                \-> non-0 channel -> connSinks[channel]
		 *
		 *                 /- < connSinks[channel]
		 *  corenet conn --
		 *                 \- > ChanneledWriter{res.Stdout(), nextChannel}
		 *
		 *  res.Stdout() <- Status updates on channel 0(new connection, closed, etc.)
		 */

		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// Must be online!
		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}
		list, err := corenet.Listen(n, "/app/"+req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var nextChannel uint64 = 1
		connSinks := map[uint64]io.Writer{}

		go func() {
			msgIn := req.Stdin().(cmds.MessageIO)
			for {
				rawMsg, err := msgIn.ReadMessage()

				if err != nil {
					return
				}

				var msg ChannelMessage
				if err := json.Unmarshal(rawMsg, &msg); err != nil {
					return
				}

				switch msg.ID {
				case 0:
					var action ChannelCtlMessage
					if err := json.Unmarshal(msg.Data, &action); err != nil {
						return
					}
				default:
					if out := connSinks[msg.ID]; out != nil {
						out.Write(msg.Data)
					} else {
						log.Debugf("Service write to non-existent channel %d on service %s", msg.ID, req.Arguments()[0])
					}
				}
			}
		}()

		for {
			conn, err := list.Accept()

			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			_, err = ChannelCtlMessage{"accept", []string{strconv.FormatUint(nextChannel, 10)}}.Write(res.Stdout())

			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			stdin, w := io.Pipe()
			channelId := nextChannel
			connSinks[channelId] = w

			go func() {
				defer conn.Close()
				defer delete(connSinks, channelId)
				defer ChannelCtlMessage{"close", []string{strconv.FormatUint(nextChannel, 10)}}.Write(res.Stdout())

				err := copy2(conn, stdin, ChanneledWriter{res.Stdout(), channelId}, conn)
				if err != nil {
					if err != io.EOF {
						res.SetError(err, cmds.ErrNormal)
					}
					return
				}
			}()

			nextChannel++
		}
	},
}

var DialCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Dial a remote service",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("NodeID", true, false, "IPFS node to connect to"),
		cmds.StringArg("Name", true, false, "Name of service to connect to"),
		cmds.StringArg("Command", false, true, "Command to expose"),
	},
	Options: []cmds.Option{
		cmds.StringOption("auth", "a", "reserved for future authentication or authorization"),
	},
	PreRun: func(req cmds.Request) error {
		if req.Option("auth").Found() {
			return fmt.Errorf("--auth not implemented")
		}

		return nil
	},
	Interact: interactWithRemote,
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// Must be online!
		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		target, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		conn, err := corenet.Dial(n, target, "/app/"+req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		defer conn.Close()

		_, err = ChannelCtlMessage{"accept", []string{"1"}}.Write(res.Stdout())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		stdin, w := io.Pipe()

		defer ChannelCtlMessage{"close", []string{"1"}}.Write(res.Stdout())

		go func() {
			msgIn := req.Stdin().(cmds.MessageIO)
			for {
				rawMsg, err := msgIn.ReadMessage()

				if err != nil {
					return
				}

				var msg ChannelMessage
				if err := json.Unmarshal(rawMsg, &msg); err != nil {
					return
				}

				switch msg.ID {
				case 0:
					var action ChannelCtlMessage
					if err := json.Unmarshal(msg.Data, &action); err != nil {
						return
					}
				case 1:
					w.Write(msg.Data)
				default:
					log.Debugf("Service write to non-existent channel %d", msg.ID)
					conn.Close()
				}
			}
		}()

		err = copy2(conn, stdin, ChanneledWriter{res.Stdout(), 1}, conn)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

// Client part
func clientHandler(id uint64, stdin io.ReadCloser, conn cmds.MessageIO, req cmds.Request) {
	argc := len(req.Command().Arguments) - 1
	args := req.Arguments()
	stdout := ChanneledWriter{conn, id}
	defer stdin.Close()
	defer ChannelCtlMessage{"close", []string{strconv.FormatUint(id, 10)}}.Write(conn)

	if len(args) > argc {
		path, err := exec.LookPath(args[argc])
		if err != nil {
			path = args[argc]
		}
		cmd := exec.Cmd{
			Path:   path,
			Args:   args[argc+1:],
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: os.Stderr,
		}
		cmd.Run()
	} else {
		copy2(stdout, os.Stdin, os.Stdout, stdin)
	}
}

func interactWithRemote(req cmds.Request, conn cmds.MessageIO) error {
	outputs := map[uint64]io.WriteCloser{}

	for {
		rawMsg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var msg ChannelMessage
		if err := json.Unmarshal(rawMsg, &msg); err != nil {
			return err
		}

		switch msg.ID {
		case 0:
			var action ChannelCtlMessage
			if err := json.Unmarshal(msg.Data, &action); err != nil {
				return err
			}
			switch action.Name {
			case "accept":
				channel, _ := strconv.ParseUint(action.Args[0], 10, 64)

				r, w := io.Pipe()
				outputs[channel] = w

				go clientHandler(channel, r, conn, req)
			case "close":
				channel, _ := strconv.ParseUint(action.Args[0], 10, 64)

				if out := outputs[channel]; out != nil {
					out.Close()
				}
			}
		default:
			if out := outputs[msg.ID]; out != nil {
				out.Write(msg.Data)
			}
		}
	}
	return nil
}

type ChanneledWriter struct {
	Orig    io.Writer
	Channel uint64
}

func (w ChanneledWriter) Write(p []byte) (int, error) {
	n := len(p)
	if _, err := (ChannelMessage{w.Channel, p}).Write(w.Orig); err != nil {
		return 0, err
	}
	return n, nil
}

type ChannelCtlMessage struct {
	Name string
	Args []string
}

func (m ChannelCtlMessage) Write(dst io.Writer) (int, error) {
	strMessage, _ := json.Marshal(m)
	return ChannelMessage{0, strMessage}.Write(dst)
}

type ChannelMessage struct {
	ID   uint64
	Data []byte
}

func (m ChannelMessage) Write(dst io.Writer) (int, error) {
	strMessage, _ := json.Marshal(m)
	return dst.Write(strMessage)
}

func copy2(dst1 io.Writer, src1 io.Reader, dst2 io.Writer, src2 io.Reader) error {
	done1 := make(chan error)
	done2 := make(chan error)
	go func() {
		_, err := io.Copy(dst1, src1)
		done1 <- err
	}()
	go func() {
		_, err := io.Copy(dst2, src2)
		done2 <- err
	}()
	ok := 0
	for {
		var err error
		select {
		case err = <-done1:
		case err = <-done2:
		}
		if err != nil {
			return err
		}
		ok += 1
		if ok == 2 {
			return nil
		}
	}
}
