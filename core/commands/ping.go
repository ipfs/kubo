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

type PingResult struct {
	Success bool
	Time    time.Duration
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
		cmds.StringArg("peer-id", true, true, "ID of peer to ping"),
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
				if obj.Success {
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

		if !n.OnlineMode() {
			return nil, errNotOnline
		}

		peerID, err := peer.IDB58Decode("QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ")
		if err != nil {
			return nil, err
		}
		const kPingTimeout = 10 * time.Second
		ctx, _ := context.WithTimeout(context.Background(), kPingTimeout)
		p, err := n.Routing.FindPeer(ctx, peerID)
		if err != nil {
			return nil, err
		}

		outChan := make(chan interface{})

		go func() {
			defer close(outChan)
			for i := 0; i < 10; i++ {
				ctx, _ = context.WithTimeout(context.Background(), kPingTimeout)
				before := time.Now()
				err := n.Routing.Ping(ctx, p.ID)
				if err != nil {
					outChan <- &PingResult{}
					break
				}
				took := time.Now().Sub(before)
				outChan <- &PingResult{
					Success: true,
					Time:    took,
				}
				time.Sleep(time.Second)
			}
		}()

		return outChan, nil
	},
	Type: PingResult{},
}
