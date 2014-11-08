package commands

import (
	"errors"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
)

type ResolveOutput struct {
	Entries []IpnsEntry
}

var resolveCmd = &cmds.Command{
	Description: "Gets the value currently published at an IPNS name",
	Help: `IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values.


Examples:

Resolve the value of your identity:

  > ipfs name resolve
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve te value of another name:

  > ipfs name resolve QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

`,

	Arguments: []cmds.Argument{
		cmds.Argument{"name", cmds.ArgString, false, true,
			"The IPNS name to resolve. Defaults to your node's peerID."},
	},
	Run: func(res cmds.Response, req cmds.Request) {

		n := req.Context().Node
		var names []string

		if n.Network == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		if len(req.Arguments()) == 0 {
			if n.Identity == nil {
				res.SetError(errors.New("Identity not loaded!"), cmds.ErrNormal)
				return
			}
			names = append(names, n.Identity.ID().String())
		} else {
			for _, arg := range req.Arguments() {
				name, ok := arg.(string)
				if !ok {
					res.SetError(errors.New("cast error"), cmds.ErrNormal)
					return
				}
				names = append(names, name)
			}
		}

		entries, err := resolve(n, names)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&ResolveOutput{entries})
	},
	Type: &ResolveOutput{},
}

func resolve(n *core.IpfsNode, names []string) ([]IpnsEntry, error) {
	var entries []IpnsEntry
	for _, name := range names {
		resolved, err := n.Namesys.Resolve(name)
		if err != nil {
			return nil, err
		}
		entries = append(entries, IpnsEntry{
			Name:  name,
			Value: resolved,
		})
	}
	return entries, nil
}
