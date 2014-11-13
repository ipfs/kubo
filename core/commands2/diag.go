package commands

import (
	"bytes"
	"io"
	"text/template"
	"time"

	cmds "github.com/jbenet/go-ipfs/commands"
	util "github.com/jbenet/go-ipfs/util"
)

type DiagnosticConnection struct {
	ID string
	// TODO use milliseconds or microseconds for human readability
	NanosecondsLatency uint64
}

type DiagnosticPeer struct {
	ID                string
	UptimeSeconds     uint64
	BandwidthBytesIn  uint64
	BandwidthBytesOut uint64
	Connections       []DiagnosticConnection
}

type DiagnosticOutput struct {
	Peers []DiagnosticPeer
}

var diagCmd = &cmds.Command{
	Description: "Generates diagnostic reports",

	Subcommands: map[string]*cmds.Command{
		"net": diagNetCmd,
	},
}

var diagNetCmd = &cmds.Command{
	Description: "Generates a network diagnostics report",
	Help: `Sends out a message to each node in the network recursively
requesting a listing of data about them including number of
connected peers and latencies between them.
`,

	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		if !n.OnlineMode() {
			return nil, errNotOnline
		}

		info, err := n.Diagnostics.GetDiagnostic(time.Second * 20)
		if err != nil {
			return nil, err
		}

		output := make([]DiagnosticPeer, len(info))
		for i, peer := range info {
			connections := make([]DiagnosticConnection, len(peer.Connections))
			for j, conn := range peer.Connections {
				connections[j] = DiagnosticConnection{
					ID:                 conn.ID,
					NanosecondsLatency: uint64(conn.Latency.Nanoseconds()),
				}
			}

			output[i] = DiagnosticPeer{
				ID:                peer.ID,
				UptimeSeconds:     uint64(peer.LifeSpan.Seconds()),
				BandwidthBytesIn:  peer.BwIn,
				BandwidthBytesOut: peer.BwOut,
				Connections:       connections,
			}
		}

		return &DiagnosticOutput{output}, nil
	},
	Type: &DiagnosticOutput{},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(r cmds.Response) ([]byte, error) {
			output, ok := r.Output().(*DiagnosticOutput)
			if !ok {
				return nil, util.ErrCast()
			}
			var buf bytes.Buffer
			err := printDiagnostics(&buf, output)
			if err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		},
	},
}

func printDiagnostics(out io.Writer, info *DiagnosticOutput) error {

	diagTmpl := `
{{ range $peer := .Peers }}
ID {{ $peer.ID }}
	up {{ $peer.UptimeSeconds }} seconds
	connected to {{ len .Connections }}...
		{{ range $connection := .Connections }}
		ID {{ $connection.ID }}
		latency: {{ $connection.NanosecondsLatency }} ns
		{{ end }}
{{end}}
`

	templ, err := template.New("DiagnosticOutput").Parse(diagTmpl)
	if err != nil {
		return err
	}

	err = templ.Execute(out, info)
	if err != nil {
		return err
	}

	return nil
}
