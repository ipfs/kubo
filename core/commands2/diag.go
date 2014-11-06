package commands

import (
	"errors"
	"fmt"
	"io"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	diagn "github.com/jbenet/go-ipfs/diagnostics"
)

type DiagnosticConnection struct {
	ID      string
	Latency int64
}

type DiagnosticPeer struct {
	ID           string
	LifeSpan     float64
	BandwidthIn  uint64
	BandwidthOut uint64
	Connections  []DiagnosticConnection
}

type DiagnosticOutput struct {
	Peers []DiagnosticPeer
}

var diagCmd = &cmds.Command{
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		if !n.Online() {
			res.SetError(errors.New("Cannot run diagnostic in offline mode!"), cmds.ErrNormal)
			return
		}

		info, err := n.Diagnostics.GetDiagnostic(time.Second * 20)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output := make([]DiagnosticPeer, len(info))
		for i, peer := range info {
			connections := make([]DiagnosticConnection, len(peer.Connections))
			for j, conn := range peer.Connections {
				connections[j] = DiagnosticConnection{
					ID:      conn.ID,
					Latency: conn.Latency.Nanoseconds(),
				}
			}

			output[i] = DiagnosticPeer{
				ID:           peer.ID,
				LifeSpan:     peer.LifeSpan.Minutes(),
				BandwidthIn:  peer.BwIn,
				BandwidthOut: peer.BwOut,
				Connections:  connections,
			}
		}

		res.SetOutput(&DiagnosticOutput{output})
	},
	Type: &DiagnosticOutput{},
}

func PrintDiagnostics(info []*diagn.DiagInfo, out io.Writer) {
	for _, i := range info {
		fmt.Fprintf(out, "Peer: %s\n", i.ID)
		fmt.Fprintf(out, "\tUp for: %s\n", i.LifeSpan.String())
		fmt.Fprintf(out, "\tConnected To:\n")
		for _, c := range i.Connections {
			fmt.Fprintf(out, "\t%s\n\t\tLatency = %s\n", c.ID, c.Latency.String())
		}
		fmt.Fprintln(out)
	}
}
