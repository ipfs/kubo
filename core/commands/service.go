package commands

import (
	// "bytes"
	"fmt"
	"io"
	"os"
	// "reflect"
	// "strings"
	// "time"
	exec "os/exec"

	cmds "github.com/ipfs/go-ipfs/commands"
	corenet "github.com/ipfs/go-ipfs/core/corenet"
	// core "github.com/ipfs/go-ipfs/core"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	// u "github.com/ipfs/go-ipfs/util"

	// ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	// context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

var ServiceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remote services over IPFS",
	},
	Subcommands: map[string]*cmds.Command{
		"listen": ListenCmd,
		"dial": DialCmd,
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
		cmds.BoolOption("verbose", "v", "Be verbose"),
		cmds.BoolOption("server", "s", "Keep serving requests until killed"),
		cmds.BoolOption("unique", "u", "Disallow other listeners on same name"),
		cmds.StringOption("proxy", "p", "multiaddr"),
		cmds.StringOption("auth", "a", "reserved for future authentication or authorization"),
	},
	PreRun: func(req cmds.Request) error {
		if req.Option("server").Found() { return fmt.Errorf("--server not implemented") }
		if req.Option("unique").Found() { return fmt.Errorf("--unique not implemented") }
		if req.Option("proxy").Found() { return fmt.Errorf("--proxy not implemented") }
		if req.Option("auth").Found() { return fmt.Errorf("--auth not implemented") }

		return nil
	},
	Interact: interactWithRemote,
	Run: func(req cmds.Request, res cmds.Response) {
		// ctx := req.Context()
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

		list, err := corenet.Listen(n, "/app/" + req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		conn, err := list.Accept()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		defer conn.Close()
		if val, _, _ := req.Option("verbose").Bool(); val {
			fmt.Fprintf(res.Stdout(), "Connection from: %s\n", conn.Conn().RemotePeer())
		}
		err = Copy2(conn, req.Stdin(), res.Stdout(), conn)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
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
		cmds.BoolOption("verbose", "v", "Be verbose"),
		cmds.StringOption("proxy", "p", "multiaddr"),
		cmds.StringOption("auth", "a", "reserved for future authentication or authorization"),
	},
	PreRun: func(req cmds.Request) error {
		if req.Option("proxy").Found() { return fmt.Errorf("--proxy not implemented") }
		if req.Option("auth").Found() { return fmt.Errorf("--auth not implemented") }

		return nil
	},
	Interact: interactWithRemote,
	Run: func(req cmds.Request, res cmds.Response) {
		// ctx := req.Context()
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
		
		conn, err := corenet.Dial(n, target, "/app/" + req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if val, _, _ := req.Option("verbose").Bool(); val {
			fmt.Fprintf(res.Stdout(), "Connected to: %s\n", target)
		}
		err = Copy2(conn, req.Stdin(), res.Stdout(), conn)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		} 
	},
}


func Copy2(dst1 io.Writer, src1 io.Reader, dst2 io.Writer, src2 io.Reader) error {
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

func interactWithRemote(req cmds.Request, conn io.ReadWriter) error {
	n := len(req.Command().Arguments) - 1	
	args := req.Arguments()
	if len(args) > n {
		path, err := exec.LookPath(args[n])
		if err != nil { path = args[n] }
		cmd := exec.Cmd{
			Path: path,
			Args: args[n+1:],
			Stdin: conn,
			Stdout: conn,
			Stderr: conn,
		}
		return cmd.Run()
	} else {
		return Copy2(conn, os.Stdin, os.Stdout, conn)
	}
}
