package commands

import (
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	corenet "github.com/ipfs/go-ipfs/core/corenet"

	manet "gx/ipfs/QmPpRcbNUXauP3zWZ1NJMLWpe4QnmEHrd2ba2D3yqWznw7/go-multiaddr-net"
	pstore "gx/ipfs/QmSZi9ygLohBUGyHMqE5N6eToPwqcg7bZQTULeVLFu7Q6d/go-libp2p-peerstore"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
)

var CorenetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Identity based p2p data transfer",
	},

	Subcommands: map[string]*cmds.Command{
		"listen": listenCmd,
		"dial":   dialCmd,
	},
}

var listenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Start listening for incoming corenet connections",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("Handler", true, false, "Address of application handling the connections"),
		cmds.StringArg("Protocol", true, false, "Protocol name"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("address", "a", "Report peer address at beginning of stream").Default(false),
		cmds.BoolOption("address-string", "s", "Make reported peer address human readable").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		reportAddress, _, _ := req.Option("address").Bool()

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		malocal, err := ma.NewMultiaddr(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		listener, err := corenet.Listen(n, "/app/"+req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		defer listener.Close()

		for {
			remote, err := listener.Accept()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			local, err := manet.Dial(malocal)
			if err != nil {
				err := remote.Close()
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}

				res.SetError(err, cmds.ErrNormal)
				return
			}

			if reportAddress {
				mremote, err := ma.NewMultiaddr("/ipfs/" + remote.Conn().RemotePeer().Pretty())
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					closePair(local, remote)
					return
				}

				var address []byte = nil
				addressHuman, _, _ := req.Option("address-string").Bool()
				if addressHuman {
					address = []byte(mremote.String() + "\n")
				} else {
					address = mremote.Bytes()
				}

				_, err = local.Write(address)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					closePair(local, remote)
					return
				}
			}

			go proxyStream(local, remote)
			go proxyStream(remote, local)
		}
	},
}

var dialCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Dial to a corenet service",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("Peer", true, false, "Peer address"),
		cmds.StringArg("Handler", true, false, "Address of application handling the connections"),
		cmds.StringArg("Protocol", true, false, "Protocol name"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		malocal, err := ma.NewMultiaddr(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		addr, peerID, err := ParsePeerParam(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if addr != nil {
			n.Peerstore.AddAddr(peerID, addr, pstore.TempAddrTTL) // temporary
		}

		remote, err := corenet.Dial(n, peerID, "/app/"+req.Arguments()[2])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		local, err := manet.Dial(malocal)
		if err != nil {
			err2 := remote.Close()
			if err2 != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetError(err, cmds.ErrNormal)
			return
		}

		go proxyStream(local, remote)
		go proxyStream(remote, local)
	},
}

func proxyStream(dst io.WriteCloser, src io.ReadCloser) {
	defer closePair(dst, src)
	io.Copy(dst, src)
}

func closePair(a io.Closer, b io.Closer) {
	a.Close()
	b.Close()
}
