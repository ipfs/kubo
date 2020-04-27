package commands

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/network"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kbucket "github.com/libp2p/go-libp2p-kbucket"
)

type dhtPeerInfo struct {
	ID            string
	Connected     bool
	AgentVersion  string
	LastUsefulAt  string
	LastQueriedAt string
}

type dhtStat struct {
	Name    string
	Buckets []dhtBucket
}

type dhtBucket struct {
	LastRefresh string
	Peers       []dhtPeerInfo
}

var statDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Returns statistics about the node's DHT(s)",
		ShortDescription: `
Returns statistics about the DHT(s) the node is participating in.

This interface is not stable and may change from release to release.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dht", false, true, "The DHT whose table should be listed (wan or lan). Defaults to both."),
	},
	Options: []cmds.Option{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		if nd.DHT == nil {
			return ErrNotDHT
		}

		id := kbucket.ConvertPeerID(nd.Identity)

		dhts := req.Arguments
		if len(dhts) == 0 {
			dhts = []string{"wan", "lan"}
		}

		for _, name := range dhts {
			var dht *dht.IpfsDHT
			switch name {
			case "wan":
				dht = nd.DHT.WAN
			case "lan":
				dht = nd.DHT.LAN
			default:
				return cmds.Errorf(cmds.ErrClient, "unknown dht type: %s", name)
			}

			rt := dht.RoutingTable()
			lastRefresh := rt.GetTrackedCplsForRefresh()
			infos := rt.GetPeerInfos()
			buckets := make([]dhtBucket, 0, len(lastRefresh))
			for _, pi := range infos {
				cpl := kbucket.CommonPrefixLen(id, kbucket.ConvertPeerID(pi.Id))
				if len(buckets) <= cpl {
					buckets = append(buckets, make([]dhtBucket, 1+cpl-len(buckets))...)
				}

				info := dhtPeerInfo{ID: pi.Id.String()}

				if ver, err := nd.Peerstore.Get(pi.Id, "AgentVersion"); err == nil {
					info.AgentVersion, _ = ver.(string)
				} else if err == pstore.ErrNotFound {
					// ignore
				} else {
					// this is a bug, usually.
					log.Errorw(
						"failed to get agent version from peerstore",
						"error", err,
					)
				}
				if !pi.LastUsefulAt.IsZero() {
					info.LastUsefulAt = pi.LastUsefulAt.Format(time.RFC3339)
				}

				if !pi.LastSuccessfulOutboundQueryAt.IsZero() {
					info.LastQueriedAt = pi.LastSuccessfulOutboundQueryAt.Format(time.RFC3339)
				}

				info.Connected = nd.PeerHost.Network().Connectedness(pi.Id) == network.Connected

				buckets[cpl].Peers = append(buckets[cpl].Peers, info)
			}
			for i := 0; i < len(buckets) && i < len(lastRefresh); i++ {
				refreshTime := lastRefresh[i]
				if !refreshTime.IsZero() {
					buckets[i].LastRefresh = refreshTime.Format(time.RFC3339)
				}
			}
			if err := res.Emit(dhtStat{
				Name:    name,
				Buckets: buckets,
			}); err != nil {
				return err
			}
		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out dhtStat) error {
			tw := tabwriter.NewWriter(w, 4, 4, 2, ' ', 0)
			defer tw.Flush()

			// Formats a time into XX ago and remove any decimal
			// parts. That is, change "2m3.00010101s" to "2m3s ago".
			now := time.Now()
			since := func(t time.Time) string {
				return now.Sub(t).Round(time.Second).String() + " ago"
			}

			count := 0
			for _, bucket := range out.Buckets {
				count += len(bucket.Peers)
			}

			fmt.Fprintf(tw, "DHT %s (%d peers):\t\t\t\n", out.Name, count)

			for i, bucket := range out.Buckets {
				lastRefresh := "never"
				if bucket.LastRefresh != "" {
					t, err := time.Parse(time.RFC3339, bucket.LastRefresh)
					if err != nil {
						return err
					}
					lastRefresh = since(t)
				}
				fmt.Fprintf(tw, "  Bucket %2d (%d peers) - refreshed %s:\t\t\t\n", i, len(bucket.Peers), lastRefresh)
				fmt.Fprintln(tw, "    Peer\tlast useful\tlast queried\tAgent Version")

				for _, p := range bucket.Peers {
					lastUseful := "never"
					if p.LastUsefulAt != "" {
						t, err := time.Parse(time.RFC3339, p.LastUsefulAt)
						if err != nil {
							return err
						}
						lastUseful = since(t)
					}

					lastQueried := "never"
					if p.LastUsefulAt != "" {
						t, err := time.Parse(time.RFC3339, p.LastQueriedAt)
						if err != nil {
							return err
						}
						lastQueried = since(t)
					}

					state := " "
					if p.Connected {
						state = "@"
					}
					fmt.Fprintf(tw, "  %s %s\t%s\t%s\t%s\n", state, p.ID, lastUseful, lastQueried, p.AgentVersion)
				}
				fmt.Fprintln(tw, "\t\t\t")
			}
			return nil
		}),
	},
	Type: dhtStat{},
}
