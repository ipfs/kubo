package objectcmd

import (
	"bytes"
	"fmt"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	path "github.com/ipfs/go-ipfs/path"
)

var ObjectDiffCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "takes a diff of the two given objects",
		ShortDescription: `
ipfs object diff is a command used to show the differences between
two ipfs objects.

Example:

   $ ls foo
   bar baz/ giraffe
   $ ipfs add -r foo
   ...
   added QmegHcnrPgMwC7tBiMxChD54fgQMBUecNw9nE9UUU4x1bz foo
   $ OBJ_A=QmegHcnrPgMwC7tBiMxChD54fgQMBUecNw9nE9UUU4x1bz
   $ echo "different content" > foo/bar
   $ ipfs add -r foo
   ...
   added QmcmRptkSPWhptCttgHg27QNDmnV33wAJyUkCnAvqD3eCD foo
   $ OBJ_B=QmcmRptkSPWhptCttgHg27QNDmnV33wAJyUkCnAvqD3eCD
   $ ipfs object diff $OBJ_A $OBJ_B
   changed "bar" from QmNgd5cz2jNftnAHBhcRUGdtiaMzb5Rhjqd4etondHHST8 to QmRfFVsjSXkhFxrfWnLpMae2M4GBVsry6VAuYYcji5MiZb
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("obj_a", true, false, "object to diff against"),
		cmds.StringArg("obj_b", true, false, "object to diff"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		a := req.Arguments()[0]
		b := req.Arguments()[1]

		pa, err := path.ParsePath(a)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		pb, err := path.ParsePath(b)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		ctx := req.Context()

		obj_a, err := node.Resolver.ResolvePath(ctx, pa)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		obj_b, err := node.Resolver.ResolvePath(ctx, pb)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		changes, err := dagutils.Diff(ctx, node.DAG, obj_a, obj_b)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(changes)
	},
	Type: []*dagutils.Change{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			changes := res.Output().([]*dagutils.Change)
			buf := new(bytes.Buffer)
			for _, change := range changes {
				switch change.Type {
				case dagutils.Add:
					fmt.Fprintf(buf, "added new link %q pointing to %s\n", change.Path, change.After)
				case dagutils.Mod:
					fmt.Fprintf(buf, "changed %q from %s to %s\n", change.Path, change.Before, change.After)
				case dagutils.Remove:
					fmt.Fprintf(buf, "removed link %q (was %s)\n", change.Path, change.Before)
				}
			}
			return buf, nil
		},
	},
}
