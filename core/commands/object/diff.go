package objectcmd

import (
	"fmt"
	"io"

	"github.com/ipfs/boxo/ipld/merkledag/dagutils"
	"github.com/ipfs/boxo/path"
	cmds "github.com/ipfs/go-ipfs-cmds"

	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
)

const (
	verboseOptionName = "verbose"
)

type Changes struct {
	Changes []*dagutils.Change
}

var ObjectDiffCmd = &cmds.Command{
	Status: cmds.Deprecated, // https://github.com/ipfs/kubo/issues/7936
	Helptext: cmds.HelpText{
		Tagline: "Display the diff between two IPFS objects.",
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
	Arguments: []cmds.Argument{
		cmds.StringArg("obj_a", true, false, "Object to diff against."),
		cmds.StringArg("obj_b", true, false, "Object to diff."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(verboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		pa, err := cmdutils.PathOrCidPath(req.Arguments[0])
		if err != nil {
			return err
		}

		pb, err := cmdutils.PathOrCidPath(req.Arguments[1])
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
				Type: dagutils.ChangeType(change.Type),
				Path: change.Path,
			}

			if (change.Before != path.ImmutablePath{}) {
				out[i].Before = change.Before.RootCid()
			}

			if (change.After != path.ImmutablePath{}) {
				out[i].After = change.After.RootCid()
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
