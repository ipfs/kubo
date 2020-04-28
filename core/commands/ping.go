package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	ping "github.com/libp2p/go-libp2p/p2p/protocol/ping"
	ma "github.com/multiformats/go-multiaddr"
)

const kPingTimeout = 10 * time.Second

type PingResult struct {
	Success bool
	Time    time.Duration
	Text    string
}

const (
	pingCountOptionName = "count"
)

// ErrPingSelf is returned when the user attempts to ping themself.
var ErrPingSelf = errors.New("error: can't ping self")

var PingCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Send echo request packets to IPFS hosts.",
		ShortDescription: `
'ipfs ping' is a tool to test sending data to other nodes. It finds nodes
via the routing system, sends pings, waits for pongs, and prints out round-
trip latency information.
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peer ID", true, true, "ID of peer to be pinged.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.IntOption(pingCountOptionName, "n", "Number of ping messages to send.").WithDefault(10),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		// Must be online!
		if !n.IsOnline {
			return ErrNotOnline
		}

		addr, pid, err := ParsePeerParam(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("failed to parse peer address '%s': %s", req.Arguments[0], err)
		}

		if pid == n.Identity {
			return ErrPingSelf
		}

		if addr != nil {
			n.Peerstore.AddAddr(pid, addr, pstore.TempAddrTTL) // temporary
		}

		numPings, _ := req.Options[pingCountOptionName].(int)
		if numPings <= 0 {
			return fmt.Errorf("ping count must be greater than 0, was %d", numPings)
		}

		if len(n.Peerstore.Addrs(pid)) == 0 {
			// Make sure we can find the node in question
			if err := res.Emit(&PingResult{
				Text:    fmt.Sprintf("Looking up peer %s", pid.Pretty()),
				Success: true,
			}); err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(req.Context, kPingTimeout)
			p, err := n.Routing.FindPeer(ctx, pid)
			cancel()
			if err != nil {
				return fmt.Errorf("peer lookup failed: %s", err)
			}
			n.Peerstore.AddAddrs(p.ID, p.Addrs, pstore.TempAddrTTL)
		}

		if err := res.Emit(&PingResult{
			Text:    fmt.Sprintf("PING %s.", pid.Pretty()),
			Success: true,
		}); err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(req.Context, kPingTimeout*time.Duration(numPings))
		defer cancel()
		pings := ping.Ping(ctx, n.PeerHost, pid)

		var (
			count int
			total time.Duration
		)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for i := 0; i < numPings; i++ {
			r, ok := <-pings
			if !ok {
				break
			}

			if r.Error != nil {
				err = res.Emit(&PingResult{
					Success: false,
					Text:    fmt.Sprintf("Ping error: %s", r.Error),
				})
			} else {
				count++
				total += r.RTT
				err = res.Emit(&PingResult{
					Success: true,
					Time:    r.RTT,
				})
			}
			if err != nil {
				return err
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if count == 0 {
			return fmt.Errorf("ping failed")
		}
		averagems := total.Seconds() * 1000 / float64(count)
		return res.Emit(&PingResult{
			Success: true,
			Text:    fmt.Sprintf("Average latency: %.2fms", averagems),
		})
	},
	Type: PingResult{},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			var (
				total time.Duration
				count int
			)

			for {
				event, err := res.Next()
				switch err {
				case nil:
				case io.EOF:
					return nil
				case context.Canceled, context.DeadlineExceeded:
					if count == 0 {
						return err
					}
					averagems := total.Seconds() * 1000 / float64(count)
					return re.Emit(&PingResult{
						Success: true,
						Text:    fmt.Sprintf("Average latency: %.2fms", averagems),
					})
				default:
					return err
				}

				pr := event.(*PingResult)
				if pr.Success && pr.Text == "" {
					total += pr.Time
					count++
				}
				err = re.Emit(event)
				if err != nil {
					return err
				}
			}
		},
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *PingResult) error {
			if len(out.Text) > 0 {
				fmt.Fprintln(w, out.Text)
			} else if out.Success {
				fmt.Fprintf(w, "Pong received: time=%.2f ms\n", out.Time.Seconds()*1000)
			} else {
				fmt.Fprintf(w, "Pong failed\n")
			}
			return nil
		}),
	},
}

func ParsePeerParam(text string) (ma.Multiaddr, peer.ID, error) {
	// Multiaddr
	if strings.HasPrefix(text, "/") {
		maddr, err := ma.NewMultiaddr(text)
		if err != nil {
			return nil, "", err
		}
		transport, id := peer.SplitAddr(maddr)
		if id == "" {
			return nil, "", peer.ErrInvalidAddr
		}
		return transport, id, nil
	}
	// Raw peer ID
	p, err := peer.Decode(text)
	return nil, p, err
}
