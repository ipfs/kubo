package commands

import (
	"context"
	"errors"
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	peer "github.com/libp2p/go-libp2p/core/peer"
	routing "github.com/libp2p/go-libp2p/core/routing"
)

var ErrNotDHT = errors.New("routing service is not a DHT")

var DhtCmd = &cmds.Command{
	Status: cmds.Deprecated,
	Helptext: cmds.HelpText{
		Tagline:          "Issue commands directly through the DHT.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"query":     queryDhtCmd,
		"findprovs": RemovedDHTCmd,
		"findpeer":  RemovedDHTCmd,
		"get":       RemovedDHTCmd,
		"put":       RemovedDHTCmd,
		"provide":   RemovedDHTCmd,
	},
}

// kademlia extends the routing interface with a command to get the peers closest to the target
type kademlia interface {
	routing.Routing
	GetClosestPeers(ctx context.Context, key string) ([]peer.ID, error)
}

var queryDhtCmd = &cmds.Command{
	Status: cmds.Deprecated,
	Helptext: cmds.HelpText{
		Tagline:          "Find the closest Peer IDs to a given Peer ID by querying the DHT.",
		ShortDescription: "Outputs a list of newline-delimited Peer IDs.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, true, "The peerID to run the query against."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if nd.DHTClient == nil {
			return ErrNotDHT
		}

		id, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return cmds.ClientError("invalid peer ID")
		}

		ctx, cancel := context.WithCancel(req.Context)
		defer cancel()
		ctx, events := routing.RegisterForQueryEvents(ctx)

		client := nd.DHTClient
		if client == nd.DHT {
			client = nd.DHT.WAN
			if !nd.DHT.WANActive() {
				client = nd.DHT.LAN
			}
		}

		if d, ok := client.(kademlia); !ok {
			return errors.New("dht client does not support GetClosestPeers")
		} else {
			errCh := make(chan error, 1)
			go func() {
				defer close(errCh)
				defer cancel()
				closestPeers, err := d.GetClosestPeers(ctx, string(id))
				for _, p := range closestPeers {
					routing.PublishQueryEvent(ctx, &routing.QueryEvent{
						ID:   p,
						Type: routing.FinalPeer,
					})
				}

				if err != nil {
					errCh <- err
					return
				}
			}()

			for e := range events {
				if err := res.Emit(e); err != nil {
					return err
				}
			}

			return <-errCh
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.FinalPeer: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					fmt.Fprintf(out, "%s\n", obj.ID)
					return nil
				},
			}
			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			return printEvent(out, w, verbose, pfm)
		}),
	},
	Type: routing.QueryEvent{},
}
var RemovedDHTCmd = &cmds.Command{
	Status: cmds.Removed,
	Helptext: cmds.HelpText{
		Tagline: "Removed, use 'ipfs routing' instead.",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return errors.New("removed, use 'ipfs routing' instead")
	},
}
