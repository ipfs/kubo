package commands

import (
	"bytes"
	"fmt"
	"io"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	u "github.com/jbenet/go-ipfs/util"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
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
ipfs ping <peer.ID> - Send pings to a peer using the routing system to discover its address
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
		cmds.IntOption("count", "n"),
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
					fmt.Fprintf(buf, "Pong took %.2fms\n", obj.Time.Seconds()*1000)
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

		if len(req.Arguments()) == 0 {
			return nil, cmds.ClientError("no peer specified!")
		}

		outChan := make(chan interface{}, 5)

		// Set up number of pings
		numPings := 10
		val, found, err := req.Option("count").Int()
		if err != nil {
			return nil, err
		}
		if found {
			numPings = val
		}

		// One argument of input required, must be base58 encoded peerID
		peerID, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			return nil, err
		}

		go func() {
			defer close(outChan)

			// Make sure we can find the node in question
			outChan <- &PingResult{
				Text: fmt.Sprintf("Looking up peer %s", peerID.Pretty()),
			}
			ctx, _ := context.WithTimeout(context.Background(), kPingTimeout)
			p, err := n.Routing.FindPeer(ctx, peerID)
			n.Peerstore.AddPeerInfo(p)
			if err != nil {
				outChan <- &PingResult{Text: "Peer lookup error!"}
				outChan <- &PingResult{Text: err.Error()}
				return
			}
			outChan <- &PingResult{
				Text: fmt.Sprintf("Peer found, starting pings."),
			}

			var total time.Duration
			for i := 0; i < numPings; i++ {
				ctx, _ = context.WithTimeout(context.Background(), kPingTimeout)
				took, err := n.Routing.Ping(ctx, p.ID)
				if err != nil {
					log.Errorf("Ping error: %s", err)
					outChan <- &PingResult{}
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
		}()

		return outChan, nil
	},
	Type: PingResult{},
}
