package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	key "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	notif "github.com/ipfs/go-ipfs/notifications"
	path "github.com/ipfs/go-ipfs/path"
	ipdht "github.com/ipfs/go-ipfs/routing/dht"
	u "github.com/ipfs/go-ipfs/util"
	peer "gx/ipfs/QmUBogf4nUefBjmYjn6jfsfPJRkmDGSeMhNj4usRKq69f4/go-libp2p/p2p/peer"
)

var ErrNotDHT = errors.New("routing service is not a DHT")

var DhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Issue commands directly through the DHT.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"query":     queryDhtCmd,
		"findprovs": findProvidersDhtCmd,
		"findpeer":  findPeerDhtCmd,
		"get":       getValueDhtCmd,
		"put":       putValueDhtCmd,
	},
}

var queryDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Run a 'findClosestPeers' query through the DHT.",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, true, "The peerID to run the query against."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("verbose", "v", "Write extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
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
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		closestPeers, err := dht.GetClosestPeers(ctx, key.Key(req.Arguments()[0]))
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

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
				printEvent(obj, buf, verbose, nil)
				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
	Type: notif.QueryEvent{},
}

var findProvidersDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Run a 'FindProviders' query through the DHT.",
		ShortDescription: `
FindProviders will return a list of peers who are able to provide the value requested.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, true, "The key to find providers for."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("verbose", "v", "Write extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
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
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		pchan := dht.FindProvidersAsync(ctx, key.B58KeyDecode(req.Arguments()[0]), numProviders)
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
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
				},
				notif.Provider: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					prov := obj.Responses[0]
					if verbose {
						fmt.Fprintf(out, "provider: ")
					}
					fmt.Fprintf(out, "%s\n", prov.ID.Pretty())
					if verbose {
						for _, a := range prov.Addrs {
							fmt.Fprintf(out, "\t%s\n", a)
						}
					}
				},
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)
				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
	Type: notif.QueryEvent{},
}

var findPeerDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Run a 'FindPeer' query through the DHT.",
		ShortDescription: ``,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, true, "The peer to search for."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
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
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

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

			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					pi := obj.Responses[0]
					fmt.Fprintf(out, "%s\n", pi.ID)
					for _, a := range pi.Addrs {
						fmt.Fprintf(out, "\t%s\n", a)
					}
				},
			}
			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, true, pfm)
				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
	Type: notif.QueryEvent{},
}

var getValueDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Run a 'GetValue' query through the DHT.",
		ShortDescription: `
GetValue will return the value stored in the dht at the given key.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, true, "The key to find a value for."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("verbose", "v", "Write extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmds.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		dhtkey, err := escapeDhtKey(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		go func() {
			defer close(outChan)
			for e := range events {
				outChan <- e
			}
		}()

		go func() {
			defer close(events)
			val, err := dht.GetValue(ctx, dhtkey)
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
			} else {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.Value,
					Extra: string(val),
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

			pfm := pfuncMap{
				notif.Value: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
					} else {
						fmt.Fprintln(out, obj.Extra)
					}
				},
			}
			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)

				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
	Type: notif.QueryEvent{},
}

var putValueDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Run a 'PutValue' query through the DHT.",
		ShortDescription: `
PutValue will store the given key value pair in the dht.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The key to store the value at."),
		cmds.StringArg("value", true, false, "The value to store.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("verbose", "v", "Write extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmds.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		key, err := escapeDhtKey(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		data := req.Arguments()[1]

		go func() {
			defer close(outChan)
			for e := range events {
				outChan <- e
			}
		}()

		go func() {
			defer close(events)
			err := dht.PutValue(ctx, key, []byte(data))
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
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
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
				},
				notif.Value: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					fmt.Fprintf(out, "storing value at %s\n", obj.ID)
				},
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
	Type: notif.QueryEvent{},
}

type printFunc func(obj *notif.QueryEvent, out io.Writer, verbose bool)
type pfuncMap map[notif.QueryEventType]printFunc

func printEvent(obj *notif.QueryEvent, out io.Writer, verbose bool, override pfuncMap) {
	if verbose {
		fmt.Fprintf(out, "%s: ", time.Now().Format("15:04:05.000"))
	}

	if override != nil {
		if pf, ok := override[obj.Type]; ok {
			pf(obj, out, verbose)
			return
		}
	}

	switch obj.Type {
	case notif.SendingQuery:
		if verbose {
			fmt.Fprintf(out, "* querying %s\n", obj.ID)
		}
	case notif.Value:
		if verbose {
			fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
		} else {
			fmt.Fprint(out, obj.Extra)
		}
	case notif.PeerResponse:
		fmt.Fprintf(out, "* %s says use ", obj.ID)
		for _, p := range obj.Responses {
			fmt.Fprintf(out, "%s ", p.ID)
		}
		fmt.Fprintln(out)
	case notif.QueryError:
		fmt.Fprintf(out, "error: %s\n", obj.Extra)
	case notif.DialingPeer:
		if verbose {
			fmt.Fprintf(out, "dialing peer: %s\n", obj.ID)
		}
	case notif.AddingPeer:
		if verbose {
			fmt.Fprintf(out, "adding peer to query: %s\n", obj.ID)
		}
	default:
		fmt.Fprintf(out, "unrecognized event type: %d\n", obj.Type)
	}
}

func escapeDhtKey(s string) (key.Key, error) {
	parts := path.SplitList(s)
	switch len(parts) {
	case 1:
		return key.B58KeyDecode(s), nil
	case 3:
		k := key.B58KeyDecode(parts[2])
		return key.Key(path.Join(append(parts[:2], k.String()))), nil
	default:
		return "", errors.New("invalid key")
	}
}
