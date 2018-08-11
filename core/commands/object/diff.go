package objectcmd

import (
	"bytes"
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/dagutils"

	cmdkit "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
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
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		a := req.Arguments()[0]
		b := req.Arguments()[1]

		pa, err := coreiface.ParsePath(a)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		pb, err := coreiface.ParsePath(b)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		changes, err := api.Object().Diff(req.Context(), pa, pb)

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

		res.SetOutput(&Changes{out})
	},
	Type: Changes{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			verbose, _, _ := res.Request().Option("v").Bool()
			changes, ok := v.(*Changes)
			if !ok {
				return nil, e.TypeErr(changes, v)
			}

			buf := new(bytes.Buffer)
			for _, change := range changes.Changes {
				if verbose {
					switch change.Type {
					case dagutils.Add:
						fmt.Fprintf(buf, "Added new link %q pointing to %s.\n", change.Path, change.After)
					case dagutils.Mod:
						fmt.Fprintf(buf, "Changed %q from %s to %s.\n", change.Path, change.Before, change.After)
					case dagutils.Remove:
						fmt.Fprintf(buf, "Removed link %q (was %s).\n", change.Path, change.Before)
					}
				} else {
					switch change.Type {
					case dagutils.Add:
						fmt.Fprintf(buf, "+ %s %q\n", change.After, change.Path)
					case dagutils.Mod:
						fmt.Fprintf(buf, "~ %s %s %q\n", change.Before, change.After, change.Path)
					case dagutils.Remove:
						fmt.Fprintf(buf, "- %s %q\n", change.Before, change.Path)
					}
				}
			}
			return buf, nil
		},
	},
}
