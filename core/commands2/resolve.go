package commands

import (
	"errors"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
)

type ResolveOutput struct {
	Entries []IpnsEntry
}

var resolveCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"name", cmds.ArgString, false, true},
	},
	Run: func(res cmds.Response, req cmds.Request) {
		name := ""
		args := req.Arguments()
		n := req.Context().Node
		var output []IpnsEntry

		if len(args) == 0 {
			if n.Identity == nil {
				res.SetError(errors.New("Identity not loaded!"), cmds.ErrNormal)
				return
			}

			name = n.Identity.ID().String()
			entry, err := resolve(name, n)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			output = []IpnsEntry{entry}

		} else {
			output = make([]IpnsEntry, len(args))

			for i, arg := range args {
				var ok bool
				name, ok = arg.(string)
				if !ok {
					res.SetError(errors.New("cast error"), cmds.ErrNormal)
					return
				}

				entry, err := resolve(name, n)
				if err != nil {
					res.SetError(err, cmds.ErrNormal)
					return
				}
				output[i] = entry
			}
		}

		res.SetOutput(&ResolveOutput{output})
	},
	Type: &ResolveOutput{},
}

func resolve(name string, n *core.IpfsNode) (IpnsEntry, error) {
	resolved, err := n.Namesys.Resolve(name)
	if err != nil {
		return IpnsEntry{}, err
	}

	return IpnsEntry{
		Name:  name,
		Value: resolved,
	}, nil
}
