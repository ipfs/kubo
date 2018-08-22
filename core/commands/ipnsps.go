package commands

import (
	"errors"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	record "gx/ipfs/QmdHb9aBELnQKTVhvvA3hsQbRgUAwsWUzBP2vZ6Y5FBYvE/go-libp2p-record"
)

type ipnsPubsubState struct {
	Enabled bool
}

type ipnsPubsubCancel struct {
	Canceled bool
}

// IpnsPubsubCmd is the subcommand that allows us to manage the IPNS pubsub system
var IpnsPubsubCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "IPNS pubsub management",
		ShortDescription: `
Manage and inspect the state of the IPNS pubsub resolver.

Note: this command is experimental and subject to change as the system is refined
`,
	},
	Subcommands: map[string]*cmds.Command{
		"state":  ipnspsStateCmd,
		"subs":   ipnspsSubsCmd,
		"cancel": ipnspsCancelCmd,
	},
}

var ipnspsStateCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Query the state of IPNS pubsub",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&ipnsPubsubState{n.PSRouter != nil})
	},
	Type: ipnsPubsubState{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			output, ok := v.(*ipnsPubsubState)
			if !ok {
				return nil, e.TypeErr(output, v)
			}

			var state string
			if output.Enabled {
				state = "enabled"
			} else {
				state = "disabled"
			}

			return strings.NewReader(state + "\n"), nil
		},
	},
}

var ipnspsSubsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show current name subscriptions",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PSRouter == nil {
			res.SetError(errors.New("IPNS pubsub subsystem is not enabled"), cmdkit.ErrClient)
			return
		}
		var paths []string
		for _, key := range n.PSRouter.GetSubscriptions() {
			ns, k, err := record.SplitKey(key)
			if err != nil || ns != "ipns" {
				// Not necessarily an error.
				continue
			}
			pid, err := peer.IDFromString(k)
			if err != nil {
				log.Errorf("ipns key not a valid peer ID: %s", err)
				continue
			}
			paths = append(paths, "/ipns/"+peer.IDB58Encode(pid))
		}

		res.SetOutput(&stringList{paths})
	},
	Type: stringList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
}

var ipnspsCancelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Cancel a name subscription",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.PSRouter == nil {
			res.SetError(errors.New("IPNS pubsub subsystem is not enabled"), cmdkit.ErrClient)
			return
		}

		name := req.Arguments()[0]
		name = strings.TrimPrefix(name, "/ipns/")
		pid, err := peer.IDB58Decode(name)
		if err != nil {
			res.SetError(err, cmdkit.ErrClient)
			return
		}

		ok := n.PSRouter.Cancel("/ipns/" + string(pid))
		res.SetOutput(&ipnsPubsubCancel{ok})
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, false, "Name to cancel the subscription for."),
	},
	Type: ipnsPubsubCancel{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			output, ok := v.(*ipnsPubsubCancel)
			if !ok {
				return nil, e.TypeErr(output, v)
			}

			var state string
			if output.Canceled {
				state = "canceled"
			} else {
				state = "no subscription"
			}

			return strings.NewReader(state + "\n"), nil
		},
	},
}
