package objectcmd

import (
	"fmt"
	"io"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/dagutils"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
)

const (
	verboseOptionName = "verbose"
)

type Changes struct {
	Changes []*dagutils.Change
}

var ObjectDiffCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Display the diff between two ipfs objects.",
		ShortDescription: `
'ipfs object diff' is a command used to show the differences between
two IPFS objects.`,
		LongDescription: `
'ipfs object diff' is a command used to show the differences between
two IPFS objects.

Example:

   > ls foo
   bar baz/ giraffe
   > ipfs add -r foo
   ...
   Added QmegHcnrPgMwC7tBiMxChD54fgQMBUecNw9nE9UUU4x1bz foo
   > OBJ_A=QmegHcnrPgMwC7tBiMxChD54fgQMBUecNw9nE9UUU4x1bz
   > echo "different content" > foo/bar
   > ipfs add -r foo
   ...
   Added QmcmRptkSPWhptCttgHg27QNDmnV33wAJyUkCnAvqD3eCD foo
   > OBJ_B=QmcmRptkSPWhptCttgHg27QNDmnV33wAJyUkCnAvqD3eCD
   > ipfs object diff -v $OBJ_A $OBJ_B
   Changed "bar" from QmNgd5cz2jNftnAHBhcRUGdtiaMzb5Rhjqd4etondHHST8 to QmRfFVsjSXkhFxrfWnLpMae2M4GBVsry6VAuYYcji5MiZb.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("obj_a", true, false, "Object to diff against."),
		cmdkit.StringArg("obj_b", true, false, "Object to diff."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(verboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		a := req.Arguments[0]
		b := req.Arguments[1]

		pa, err := coreiface.ParsePath(a)
		if err != nil {
			return err
		}

		pb, err := coreiface.ParsePath(b)
		if err != nil {
			return err
		}

		changes, err := api.Object().Diff(req.Context, pa, pb)
		if err != nil {
			return err
		}

		out := make([]*dagutils.Change, len(changes))
		for i, change := range changes {
			out[i] = &dagutils.Change{
				Type: change.Type,
				Path: change.Path,
			}

			if change.Before != nil {
				out[i].Before = change.Before.Cid()
			}

			if change.After != nil {
				out[i].After = change.After.Cid()
			}
		}

		return cmds.EmitOnce(res, &Changes{out})
	},
	Type: Changes{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Changes) error {
			verbose, _ := req.Options[verboseOptionName].(bool)

			for _, change := range out.Changes {
				if verbose {
					switch change.Type {
					case dagutils.Add:
						fmt.Fprintf(w, "Added new link %q pointing to %s.\n", change.Path, change.After)
					case dagutils.Mod:
						fmt.Fprintf(w, "Changed %q from %s to %s.\n", change.Path, change.Before, change.After)
					case dagutils.Remove:
						fmt.Fprintf(w, "Removed link %q (was %s).\n", change.Path, change.Before)
					}
				} else {
					switch change.Type {
					case dagutils.Add:
						fmt.Fprintf(w, "+ %s %q\n", change.After, change.Path)
					case dagutils.Mod:
						fmt.Fprintf(w, "~ %s %s %q\n", change.Before, change.After, change.Path)
					case dagutils.Remove:
						fmt.Fprintf(w, "- %s %q\n", change.Before, change.Path)
					}
				}
			}

			return nil
		}),
	},
}
