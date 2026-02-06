package commands

import (
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

// reachabilityHost provides access to the AutoNAT reachability status.
type reachabilityHost interface {
	Reachability() network.Reachability
}

// confirmedAddrsHost provides access to per-address reachability from AutoNAT V2.
type confirmedAddrsHost interface {
	ConfirmedAddrs() (reachable, unreachable, unknown []ma.Multiaddr)
}

// autoNATResult represents the AutoNAT reachability information.
type autoNATResult struct {
	Reachability string   `json:"reachability"`
	Reachable    []string `json:"reachable,omitempty"`
	Unreachable  []string `json:"unreachable,omitempty"`
	Unknown      []string `json:"unknown,omitempty"`
}

var swarmAddrsAutoNATCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show address reachability as determined by AutoNAT V2.",
		ShortDescription: `
'ipfs swarm addrs autonat' shows the reachability status of your node's
addresses as determined by AutoNAT V2.
`,
		LongDescription: `
'ipfs swarm addrs autonat' shows the reachability status of your node's
addresses as verified by AutoNAT V2.

AutoNAT V2 probes your node's addresses to determine if they are reachable
from the public internet. This helps understand whether other peers can
dial your node directly.

The output shows:
- Reachability: Overall status (public, private, or unknown)
- Reachable: Addresses confirmed to be publicly reachable
- Unreachable: Addresses that failed reachability checks
- Unknown: Addresses that haven't been tested yet

For more information on AutoNAT V2, see:
https://github.com/libp2p/specs/blob/master/autonat/autonat-v2.md

Example:

    > ipfs swarm addrs autonat
    AutoNAT V2 Status:
      Reachability: public

    Per-Address Reachability:
      Reachable:
        /ip4/203.0.113.42/tcp/4001
        /ip4/203.0.113.42/udp/4001/quic-v1
      Unreachable:
        /ip6/2001:db8::1/tcp/4001
      Unknown:
        /ip4/203.0.113.42/udp/4001/webrtc-direct
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		result := autoNATResult{}

		// Try to get AutoNAT V2 per-address reachability
		// The host might be a BasicHost directly, or embed one (closableBasicHost, closableRoutedHost)
		if confirmedHost, ok := nd.PeerHost.(confirmedAddrsHost); ok {
			reachable, unreachable, unknown := confirmedHost.ConfirmedAddrs()
			for _, addr := range reachable {
				result.Reachable = append(result.Reachable, addr.String())
			}
			for _, addr := range unreachable {
				result.Unreachable = append(result.Unreachable, addr.String())
			}
			for _, addr := range unknown {
				result.Unknown = append(result.Unknown, addr.String())
			}
		}

		// Get overall reachability status
		if reachHost, ok := nd.PeerHost.(reachabilityHost); ok {
			result.Reachability = reachHost.Reachability().String()
		}

		return cmds.EmitOnce(res, result)
	},
	Type: autoNATResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result autoNATResult) error {
			fmt.Fprintln(w, "AutoNAT V2 Status:")
			fmt.Fprintf(w, "  Reachability: %s\n", result.Reachability)

			fmt.Fprintln(w)
			fmt.Fprintln(w, "Per-Address Reachability:")

			if len(result.Reachable) > 0 {
				fmt.Fprintln(w, "  Reachable:")
				for _, addr := range result.Reachable {
					fmt.Fprintf(w, "    %s\n", addr)
				}
			}

			if len(result.Unreachable) > 0 {
				fmt.Fprintln(w, "  Unreachable:")
				for _, addr := range result.Unreachable {
					fmt.Fprintf(w, "    %s\n", addr)
				}
			}

			if len(result.Unknown) > 0 {
				fmt.Fprintln(w, "  Unknown:")
				for _, addr := range result.Unknown {
					fmt.Fprintf(w, "    %s\n", addr)
				}
			}

			if len(result.Reachable) == 0 && len(result.Unreachable) == 0 && len(result.Unknown) == 0 {
				fmt.Fprintln(w, "  (no address reachability data available)")
			}

			return nil
		}),
	},
}
