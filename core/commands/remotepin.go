package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	cid "github.com/ipfs/go-cid"
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
		"ls":  listRemotePinCmd,
		"rm":  rmRemotePinCmd,
	},
}

const pinNameOptionName = "name"
const pinCIDsOptionName = "cid"

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
		if err != nil {
			return err
		}

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
}

var listRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects pinned to remote pinning service.",
		ShortDescription: `
Returns a list of objects that are pinned to a remote pinning service.
`,
		LongDescription: `
Returns a list of objects that are pinned to a remote pinning service.
`,
	},

	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		cmds.StringOption(pinNameOptionName, "An optional name for the pin to be listed."),
		cmds.StringsOption(pinCIDsOptionName, "An optional list of CIDs to be listed."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := []pinclient.LsOption{}
		if name, nameFound := req.Options[pinNameOptionName].(string); nameFound {
			opts = append(opts, pinclient.PinOpts.FilterName(name))
		}
		if cidsRaw, cidsFound := req.Options[pinNameOptionName].([]string); cidsFound {
			parsedCIDs := []cid.Cid{}
			for _, rawCID := range cidsRaw {
				parsedCID, err := cid.Decode(rawCID)
				if err != nil {
					return fmt.Errorf("CID %s cannot be parsed (%v)", rawCID, err)
				}
				parsedCIDs = append(parsedCIDs, parsedCID)
			}
			opts = append(opts, pinclient.PinOpts.FilterCIDs(parsedCIDs...))
		}

		c := pinclient.NewClient(remotePinURL, remotePinKey)

		psCh, errCh := c.Ls(ctx, opts...)

		for {
			select {
			case ps := <-psCh:
				if err := res.Emit(&AddRemotePinOutput{
					ID:        ps.GetId(),
					Name:      ps.GetPin().GetName(),
					Delegates: ps.GetDelegates(),
				}); err != nil {
					return err
				}
			case err := <-errCh:
				return err
			case <-ctx.Done():
				return fmt.Errorf("interrupted")
			}
		}
	},
	Type: &AddRemotePinOutput{},
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
}

var rmRemotePinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove pinned objects from remote pinning service.",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("pin-id", true, true, "ID of the pin to be removed.").EnableStdin(),
	},
	Options: []cmds.Option{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if len(req.Arguments) == 0 {
			return fmt.Errorf("missing a pin ID argument")
		}

		c := pinclient.NewClient(remotePinURL, remotePinKey)

		return c.DeleteByID(ctx, req.Arguments[0])
	},
}
