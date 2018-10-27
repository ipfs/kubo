package name

import (
	"fmt"
	"io"
	"strings"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/e"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"gx/ipfs/Qma9Eqp16mNHDX1EL73pcxhFfzbyXVcAYtaDd1xdmDRDtL/go-libp2p-record"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

type ipnsPubsubState struct {
	Enabled bool
}

type ipnsPubsubCancel struct {
	Canceled bool
}

type stringList struct {
	Strings []string
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
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ipnsPubsubState{n.PSRouter != nil})
	},
	Type: ipnsPubsubState{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			output, ok := v.(*ipnsPubsubState)
			if !ok {
				return e.TypeErr(output, v)
			}

			var state string
			if output.Enabled {
				state = "enabled"
			} else {
				state = "disabled"
			}

			_, err := fmt.Fprintln(w, state)
			return err
		}),
	},
}

var ipnspsSubsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show current name subscriptions",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if n.PSRouter == nil {
			return cmdkit.Errorf(cmdkit.ErrClient, "IPNS pubsub subsystem is not enabled")
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

		return cmds.EmitOnce(res, &stringList{paths})
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: stringListMarshaler(),
	},
}

var ipnspsCancelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Cancel a name subscription",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if n.PSRouter == nil {
			return cmdkit.Errorf(cmdkit.ErrClient, "IPNS pubsub subsystem is not enabled")
		}

		name := req.Arguments[0]
		name = strings.TrimPrefix(name, "/ipns/")
		pid, err := peer.IDB58Decode(name)
		if err != nil {
			return cmdkit.Errorf(cmdkit.ErrClient, err.Error())
		}

		ok, err := n.PSRouter.Cancel("/ipns/" + string(pid))
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, &ipnsPubsubCancel{ok})
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, false, "Name to cancel the subscription for."),
	},
	Type: ipnsPubsubCancel{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			output, ok := v.(*ipnsPubsubCancel)
			if !ok {
				return e.TypeErr(output, v)
			}

			var state string
			if output.Canceled {
				state = "canceled"
			} else {
				state = "no subscription"
			}

			_, err := fmt.Fprintln(w, state)
			return err
		}),
	},
}

func stringListMarshaler() cmds.EncoderFunc {
	return cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
		list, ok := v.(*stringList)
		if !ok {
			return e.TypeErr(list, v)
		}

		for _, s := range list.Strings {
			_, err := fmt.Fprintln(w, s)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
