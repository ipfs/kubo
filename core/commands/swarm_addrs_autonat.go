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

func multiaddrsToStrings(addrs []ma.Multiaddr) []string {
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}

func writeAddrSection(w io.Writer, label string, addrs []string) {
	if len(addrs) > 0 {
		fmt.Fprintf(w, "  %s:\n", label)
		for _, addr := range addrs {
			fmt.Fprintf(w, "    %s\n", addr)
		}
	}
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
- Reachability: Overall status (Public, Private, or Unknown)
- Reachable: Addresses confirmed to be publicly reachable
- Unreachable: Addresses that failed reachability checks
- Unknown: Addresses that haven't been tested yet

For more information on AutoNAT V2, see:
https://github.com/libp2p/specs/blob/master/autonat/autonat-v2.md

Example:

    > ipfs swarm addrs autonat
    AutoNAT V2 Status:
      Reachability: Public

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

		result := autoNATResult{
			Reachability: network.ReachabilityUnknown.String(),
		}

		// Get per-address reachability from AutoNAT V2.
		// The host embeds *BasicHost (closableBasicHost, closableRoutedHost)
		// which implements ConfirmedAddrs.
		if h, ok := nd.PeerHost.(confirmedAddrsHost); ok {
			reachable, unreachable, unknown := h.ConfirmedAddrs()
			result.Reachable = multiaddrsToStrings(reachable)
			result.Unreachable = multiaddrsToStrings(unreachable)
			result.Unknown = multiaddrsToStrings(unknown)
		}

		// Get overall reachability status.
		if h, ok := nd.PeerHost.(reachabilityHost); ok {
			result.Reachability = h.Reachability().String()
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

			writeAddrSection(w, "Reachable", result.Reachable)
			writeAddrSection(w, "Unreachable", result.Unreachable)
			writeAddrSection(w, "Unknown", result.Unknown)

			if len(result.Reachable) == 0 && len(result.Unreachable) == 0 && len(result.Unknown) == 0 {
				fmt.Fprintln(w, "  (no address reachability data available)")
			}

			return nil
		}),
	},
}
