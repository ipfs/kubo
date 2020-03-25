package commands

import (
	"fmt"
	"io"
	"text/template"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/doctor"

	cmds "github.com/ipfs/go-ipfs-cmds"

	libp2pNetwork "github.com/libp2p/go-libp2p-core/network"
)

type NetworkDiagnostics struct {
	doctor.Status
	IsOnline bool
}

const netDiagPublicText = `
You are online and reachable from the public network.
`

const netDiagUnknownText = `
IPFS has not yet determined if your node is reachable from the public network.
`

const netDiagPrivateText = `
{{ if .UsingRelays -}}
Your IPFS node appears to be behind a Firewall or NAT and can only be reached
through a relay.
{{- else -}}
{{if .AutoRelayEnabled -}}
Your IPFS node appears to be behind a Firewall or NAT and will try to
automatically find a relay. However, it is not currently reachable.
{{- else -}}
Your IPFS node appears to be behind a Firewall or NAT and cannot be reached by
nodes in the public network.

To join the network via a relay, please enable the "auto-relay" feature:

    ipfs config --bool Swarm.EnableAutoRelay true

And then restart IPFS.
{{- end -}}
{{end}}

Please note, relays are public and highly constrained resources.

For the best performance, please enable Universal Plug n Play (UPnP) on your
router so IPFS can automatically configure it to open an inbound port.

{{ if or .TCPPorts .UDPPorts -}}
Alternatively, you can manually configure your router. Please forward the following ports:

{{ if .TCPPorts -}}
TCP:{{ range $element := .TCPPorts}} {{$element}}{{end}}
{{- end}}{{if .UDPPorts -}}
UDP:{{ range $element := .UDPPorts}} {{$element}}{{end}}
{{- end}}

To (your current IP): {{.LocalIP}}
{{- end}}
{{if .Gateway -}}
Your router's administrative console can likely be found at:

    {{.Gateway}}
{{end -}}
`

var netDiagPublicTemplate, netDiagPrivateTemplate, netDiagUnknownTemplate *template.Template

func init() {
	netDiagPublicTemplate = template.Must(template.New("net-diag-public").Parse(netDiagPublicText))
	netDiagPrivateTemplate = template.Must(template.New("net-diag-private").Parse(netDiagPrivateText))
	netDiagUnknownTemplate = template.Must(template.New("net-diag-unknown").Parse(netDiagUnknownText))
}

var netDiag = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Diagnose networking issues.",
		ShortDescription: `
Diagnoses networking issues.
`,
		LongDescription: `
Diagnoses networking issues.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return cmds.EmitOnce(res, &NetworkDiagnostics{IsOnline: false})
		}

		status, err := nd.Doctor.GetStatus(req.Context)
		if err := cmds.EmitOnce(res, &NetworkDiagnostics{IsOnline: true, Status: *status}); err != nil {
			return err
		}
		return err
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *NetworkDiagnostics) error {
			if !out.IsOnline {
				fmt.Fprintf(w, "Your node is running in offline mode.\n")
				return nil
			}

			if !out.Listening {
				fmt.Fprintf(w, "You have configured your node to not listen on any network addresses.\n")
				return nil
			}

			var tmpl *template.Template
			switch out.Reachability {
			case libp2pNetwork.ReachabilityPrivate:
				tmpl = netDiagPrivateTemplate
			case libp2pNetwork.ReachabilityPublic:
				tmpl = netDiagPublicTemplate
			case libp2pNetwork.ReachabilityUnknown:
				tmpl = netDiagUnknownTemplate
			default:
				return fmt.Errorf("unknown reachability: %d", out.Reachability)
			}
			return tmpl.Execute(w, &out.Status)
		}),
	},
	Type: NetworkDiagnostics{},
}
