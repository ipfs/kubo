package commands

import (
	"errors"
	"io"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

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
		cmds.StringArg("name", false, false, "The IPNS name to resolve. Defaults to your node's peerID."),
	},
	Run: func(req cmds.Request) (interface{}, error) {

		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		var name string

		if n.PeerHost == nil {
			return nil, errNotOnline
		}

		if len(req.Arguments()) == 0 {
			if n.Identity == "" {
				return nil, errors.New("Identity not loaded!")
			}
			name = n.Identity.Pretty()

		} else {
			name = req.Arguments()[0]
		}

		output, err := n.Namesys.Resolve(name)
		if err != nil {
			return nil, err
		}

		// TODO: better errors (in the case of not finding the name, we get "failed to find any peer in table")

		return output, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			output := res.Output().(string)
			return strings.NewReader(output), nil
		},
	},
}
