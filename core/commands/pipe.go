package commands

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	corenet "github.com/ipfs/go-ipfs/core/corenet"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	u "github.com/ipfs/go-ipfs/util"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

var PipeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "",
		Synopsis:         ``,
		ShortDescription: ``,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peer ID", true, true, "ID of peer to talk to"),
	},
	Options: []cmds.Option{
		cmds.IntOption("count", "n", "number of ping messages to send"),
		cmds.BoolOption("listen", "l", "listen for other peer to dial us"),
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out, ok := res.Output().(io.Reader)
			if !ok {
				fmt.Println(reflect.TypeOf(res.Output()))
				return nil, u.ErrCast()
			}

			return out, nil
		},
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context().Context
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// Must be online!
		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		pid, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		pinfo, err := n.Routing.FindPeer(nctx, pid)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		n.Peerstore.AddAddrs(pinfo.ID, pinfo.Addrs, time.Minute)

		listen, _, err := req.Option("listen").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var rwc io.ReadWriteCloser
		if listen {
			list, err := corenet.Listen(n, protocolForPeer(pid))
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			for {
				s, err := list.Accept()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				if s.Conn().RemotePeer() != pid {
					log.Warning("Got connection from incorrect peer")
					continue
				}

				log.Error("Got connection!")

				s.Write([]byte("HELLO!"))
				rwc = s
			}
		} else {
			s, err := corenet.Dial(n, pid, protocolForPeer(n.Identity))
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			log.Error("Got connection!")
			rwc = s
		}

		go io.Copy(os.Stdout, req.Stdin())

		res.SetOutput(rwc)
	},
	Type: PingResult{},
}

func protocolForPeer(pid peer.ID) string {
	return "/ipfs/pipe/" + string(pid)
}
