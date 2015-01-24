package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	notif "github.com/jbenet/go-ipfs/notifications"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	ipdht "github.com/jbenet/go-ipfs/routing/dht"
	u "github.com/jbenet/go-ipfs/util"
)

var ErrNotDHT = errors.New("routing service is not a DHT")

var DhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Issue commands directly through the DHT",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"query":     queryDhtCmd,
		"findprovs": findProvidersDhtCmd,
		"findpeer":  findPeerDhtCmd,
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
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmds.ErrNormal)
			return
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
		res.SetOutput((<-chan interface{})(outChan))

		go func() {
			defer close(outChan)
			for e := range events {
				outChan <- e
			}
		}()
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

				verbose, _, _ := res.Request().Option("v").Bool()

				buf := new(bytes.Buffer)
				if verbose {
					fmt.Fprintf(buf, "%s: ", time.Now().Format("15:04:05.000"))
				}
				switch obj.Type {
				case notif.FinalPeer:
					fmt.Fprintf(buf, "%s\n", obj.ID)
				case notif.PeerResponse:
					if verbose {
						fmt.Fprintf(buf, "* %s says use ", obj.ID)
						for _, p := range obj.Responses {
							fmt.Fprintf(buf, "%s ", p.ID)
						}
						fmt.Fprintln(buf)
					}
				case notif.SendingQuery:
					if verbose {
						fmt.Fprintf(buf, "* querying %s\n", obj.ID)
					}
				case notif.QueryError:
					fmt.Fprintf(buf, "error: %s\n", obj.Extra)
				default:
					fmt.Fprintf(buf, "unrecognized event type: %d\n", obj.Type)
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

var findProvidersDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Run a 'FindProviders' query through the DHT",
		ShortDescription: `
FindProviders will return a list of peers who are able to provide the value requested.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, true, "The key to find providers for"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("verbose", "v", "Write extra information"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmds.ErrNormal)
			return
		}

		numProviders := 20

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context().Context, events)

		pchan := dht.FindProvidersAsync(ctx, u.B58KeyDecode(req.Arguments()[0]), numProviders)
		go func() {
			defer close(outChan)
			for e := range events {
				outChan <- e
			}
		}()

		go func() {
			defer close(events)
			for p := range pchan {
				np := p
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:      notif.Provider,
					Responses: []*peer.PeerInfo{&np},
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			verbose, _, _ := res.Request().Option("v").Bool()

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				if verbose {
					fmt.Fprintf(buf, "%s: ", time.Now().Format("15:04:05.000"))
				}
				switch obj.Type {
				case notif.FinalPeer:
					if verbose {
						fmt.Fprintf(buf, "* closest peer %s\n", obj.ID)
					}
				case notif.Provider:
					fmt.Fprintf(buf, "%s\n", obj.ID.Pretty())
				case notif.PeerResponse:
					if verbose {
						fmt.Fprintf(buf, "* %s says use ", obj.ID)
						for _, p := range obj.Responses {
							fmt.Fprintf(buf, "%s ", p.ID)
						}
						fmt.Fprintln(buf)
					}
				case notif.SendingQuery:
					if verbose {
						fmt.Fprintf(buf, "* querying %s\n", obj.ID)
					}
				case notif.QueryError:
					fmt.Fprintf(buf, "error: %s\n", obj.Extra)
				default:
					fmt.Fprintf(buf, "unrecognized event type: %d\n", obj.Type)
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

var findPeerDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Run a 'FindPeer' query through the DHT",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, true, "The peer to search for"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmds.ErrNormal)
			return
		}

		pid, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context().Context, events)

		go func() {
			defer close(outChan)
			for v := range events {
				outChan <- v
			}
		}()

		go func() {
			defer close(events)
			pi, err := dht.FindPeer(ctx, pid)
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
				return
			}

			notif.PublishQueryEvent(ctx, &notif.QueryEvent{
				Type:      notif.FinalPeer,
				Responses: []*peer.PeerInfo{&pi},
			})
		}()
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
					pi := obj.Responses[0]
					fmt.Fprintf(buf, "%s\n", pi.ID)
					for _, a := range pi.Addrs {
						fmt.Fprintf(buf, "\t%s\n", a)
					}
				case notif.PeerResponse:
					fmt.Fprintf(buf, "* %s says use ", obj.ID)
					for _, p := range obj.Responses {
						fmt.Fprintf(buf, "%s ", p.ID)
					}
					fmt.Fprintln(buf)
				case notif.SendingQuery:
					fmt.Fprintf(buf, "* querying %s\n", obj.ID)
				case notif.QueryError:
					fmt.Fprintf(buf, "error: %s\n", obj.Extra)
				default:
					fmt.Fprintf(buf, "unrecognized event type: %d\n", obj.Type)
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
