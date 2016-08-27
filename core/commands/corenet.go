package commands

import (
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	corenet "github.com/ipfs/go-ipfs/core/corenet"

	manet "gx/ipfs/QmPpRcbNUXauP3zWZ1NJMLWpe4QnmEHrd2ba2D3yqWznw7/go-multiaddr-net"
	pstore "gx/ipfs/QmQdnfvZQuhdT93LNc5bos52wAmdr3G2p6G8teLJMEN32P/go-libp2p-peerstore"
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
	defer dst.Close()
	defer src.Close()
	io.Copy(dst, src)
}
