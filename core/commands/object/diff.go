package objectcmd

import (
	"bytes"
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	path "github.com/ipfs/go-ipfs/path"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
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
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		a := req.Arguments()[0]
		b := req.Arguments()[1]

		pa, err := path.ParsePath(a)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		pb, err := path.ParsePath(b)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		ctx := req.Context()

		obj_a, err := core.Resolve(ctx, node.Namesys, node.Resolver, pa)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		obj_b, err := core.Resolve(ctx, node.Namesys, node.Resolver, pb)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		changes, err := dagutils.Diff(ctx, node.DAG, obj_a, obj_b)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&Changes{changes})
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
