package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	pinclient "github.com/ipfs/go-pinning-service-http-client"
	path "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/multiformats/go-multiaddr"
)

var remotePinURL = os.Getenv("IPFS_REMOTE_PIN_SERVICE")
var remotePinKey = os.Getenv("IPFS_REMOTE_PIN_KEY")

var remotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pin (and unpin) objects to remote pinning service.",
	},

	Subcommands: map[string]*cmds.Command{
		"add": addRemotePinCmd,
	},
}

const pinNameOptionName = "name"

type AddRemotePinOutput struct {
	ID        string
	Name      string
	Delegates []multiaddr.Multiaddr
}

var addRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin objects to remote storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path to a remote pinning service.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "An optional name for the pin."),
	},
	Type: AddRemotePinOutput{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := []pinclient.AddOption{}
		if name, nameFound := req.Options[pinNameOptionName].(string); nameFound {
			opts = append(opts, pinclient.PinOpts.WithName(name))
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		if len(req.Arguments) != 1 {
			return fmt.Errorf("expecting one CID argument")
		}
		rp, err := api.ResolvePath(ctx, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		c := pinclient.NewClient(remotePinURL, remotePinKey)

		ps, err := c.Add(ctx, rp.Cid(), opts...)

		return res.Emit(&AddRemotePinOutput{
			ID:        ps.GetId(),
			Name:      ps.GetPin().GetName(),
			Delegates: ps.GetDelegates(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *AddRemotePinOutput) error {
			fmt.Printf("pin_id=%v\n", out.ID)
			fmt.Printf("pin_name=%q\n", out.Name)
			for _, d := range out.Delegates {
				fmt.Printf("pin_delegate=%v\n", d.String())
			}
			return nil
		}),
	},
	// PostRun: cmds.PostRunMap{
	// 	cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
	// 		XXX
	// 	},
	// },
}
