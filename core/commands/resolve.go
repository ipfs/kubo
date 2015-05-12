package commands

import (
	"errors"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	namesys "github.com/ipfs/go-ipfs/namesys"
	path "github.com/ipfs/go-ipfs/path"
	u "github.com/ipfs/go-ipfs/util"
)

type ResolvedPath struct {
	Path path.Path
}

var resolveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Gets the value currently published at an IPNS name",
		ShortDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In resolve, the
default value of <name> is your own identity public key.
`,
		LongDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In resolve, the
default value of <name> is your own identity public key.


Examples:

Resolve the value of your identity:

  > ipfs name resolve
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve te value of another name:

  > ipfs name resolve QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("name", false, false, "The IPNS name to resolve. Defaults to your node's peerID.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Resolve until the result is not an IPNS name"),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			err := n.SetupOfflineRouting()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		var name string

		if len(req.Arguments()) == 0 {
			if n.Identity == "" {
				res.SetError(errors.New("Identity not loaded!"), cmds.ErrNormal)
				return
			}
			name = n.Identity.Pretty()

		} else {
			name = req.Arguments()[0]
		}

		recursive, _, _ := req.Option("recursive").Bool()
		depth := 1
		if recursive {
			depth = namesys.DefaultDepthLimit
		}

		output, err := n.Namesys.ResolveN(n.Context(), "/ipns/"+name, depth)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// TODO: better errors (in the case of not finding the name, we get "failed to find any peer in table")

		res.SetOutput(&ResolvedPath{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			output, ok := res.Output().(*ResolvedPath)
			if !ok {
				return nil, u.ErrCast()
			}
			return strings.NewReader(output.Path.String()), nil
		},
	},
	Type: ResolvedPath{},
}
