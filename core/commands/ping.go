package commands

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

const kPingTimeout = 10 * time.Second

type PingResult struct {
	Success bool
	Time    time.Duration
	Text    string
}

var PingCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "send echo request packets to IPFS hosts",
		Synopsis: `
Send pings to a peer using the routing system to discover its address
		`,
		ShortDescription: `
		ipfs ping is a tool to find a node (in the routing system),
		send pings, wait for pongs, and print out round-trip latency information.
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peer ID", true, true, "ID of peer to be pinged"),
	},
	Options: []cmds.Option{
		cmds.IntOption("count", "n", "number of ping messages to send"),
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*PingResult)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				if len(obj.Text) > 0 {
					buf = bytes.NewBufferString(obj.Text + "\n")
				} else if obj.Success {
					fmt.Fprintf(buf, "Pong received: time=%.2f ms\n", obj.Time.Seconds()*1000)
				} else {
					fmt.Fprintf(buf, "Pong failed\n")
				}
				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
			}, nil
		},
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		// Must be online!
		if !n.OnlineMode() {
			return nil, errNotOnline
		}

		addr, peerID, err := ParsePeerParam(req.Arguments()[0])
		if err != nil {
			return nil, err
		}

		if addr != nil {
			n.Peerstore.AddAddress(peerID, addr)
		}

		// Set up number of pings
		numPings := 10
		val, found, err := req.Option("count").Int()
		if err != nil {
			return nil, err
		}
		if found {
			numPings = val
		}

		outChan := make(chan interface{})

		go pingPeer(n, peerID, numPings, outChan)

		return outChan, nil
	},
	Type: PingResult{},
}

func pingPeer(n *core.IpfsNode, pid peer.ID, numPings int, outChan chan interface{}) {
	defer close(outChan)

	if len(n.Peerstore.Addresses(pid)) == 0 {
		// Make sure we can find the node in question
		outChan <- &PingResult{
			Text: fmt.Sprintf("Looking up peer %s", pid.Pretty()),
		}

		// TODO: get master context passed in
		ctx, _ := context.WithTimeout(context.TODO(), kPingTimeout)
		p, err := n.Routing.FindPeer(ctx, pid)
		if err != nil {
			outChan <- &PingResult{Text: fmt.Sprintf("Peer lookup error: %s", err)}
			return
		}
		n.Peerstore.AddPeerInfo(p)
	}

	outChan <- &PingResult{Text: fmt.Sprintf("PING %s.", pid.Pretty())}

	var total time.Duration
	for i := 0; i < numPings; i++ {
		ctx, _ := context.WithTimeout(context.TODO(), kPingTimeout)
		took, err := n.Routing.Ping(ctx, pid)
		if err != nil {
			log.Errorf("Ping error: %s", err)
			outChan <- &PingResult{Text: fmt.Sprintf("Ping error: %s", err)}
			break
		}
		outChan <- &PingResult{
			Success: true,
			Time:    took,
		}
		total += took
		time.Sleep(time.Second)
	}
	averagems := total.Seconds() * 1000 / float64(numPings)
	outChan <- &PingResult{
		Text: fmt.Sprintf("Average latency: %.2fms", averagems),
	}
}

func ParsePeerParam(text string) (ma.Multiaddr, peer.ID, error) {
	// to be replaced with just multiaddr parsing, once ptp is a multiaddr protocol
	idx := strings.LastIndex(text, "/")
	if idx == -1 {
		pid, err := peer.IDB58Decode(text)
		if err != nil {
			return nil, "", err
		}

		return nil, pid, nil
	}

	addrS := text[:idx]
	peeridS := text[idx+1:]

	var maddr ma.Multiaddr
	var pid peer.ID

	// make sure addrS parses as a multiaddr.
	if len(addrS) > 0 {
		var err error
		maddr, err = ma.NewMultiaddr(addrS)
		if err != nil {
			return nil, "", err
		}
	}

	// make sure idS parses as a peer.ID
	var err error
	pid, err = peer.IDB58Decode(peeridS)
	if err != nil {
		return nil, "", err
	}

	return maddr, pid, nil
}
