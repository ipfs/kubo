package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	notif "github.com/jbenet/go-ipfs/notifications"
	ipdht "github.com/jbenet/go-ipfs/routing/dht"
	u "github.com/jbenet/go-ipfs/util"
)

var DhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Issue commands directly through the DHT",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"query": queryDhtCmd,
	},
}

var queryDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Run a 'findClosestPeers' query through the DHT",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, true, "The peerID to run the query against"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("verbose", "v", "Write extra information"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			return nil, errors.New("Routing service was not a dht")
		}

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context().Context, events)

		closestPeers, err := dht.GetClosestPeers(ctx, u.Key(req.Arguments()[0]))

		go func() {
			defer close(events)
			for p := range closestPeers {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					ID:   p,
					Type: notif.FinalPeer,
				})
			}
		}()

		outChan := make(chan interface{})
		go func() {
			defer close(outChan)
			for e := range events {
				outChan <- e
			}
		}()
		return outChan, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				fmt.Fprintf(buf, "%s: ", time.Now().Format("15:04:05.000"))
				switch obj.Type {
				case notif.FinalPeer:
					fmt.Fprintf(buf, "%s\n", obj.ID)
				case notif.PeerResponse:
					fmt.Fprintf(buf, "* %s says use ", obj.ID)
					for _, p := range obj.Responses {
						fmt.Fprintf(buf, "%s ", p.ID)
					}
					fmt.Fprintln(buf)
				case notif.SendingQuery:
					fmt.Fprintf(buf, "* querying %s\n", obj.ID)
				}
				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
			}, nil
		},
	},
	Type: notif.QueryEvent{},
}
