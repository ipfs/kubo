package commands

import (
	"errors"
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	crypto "github.com/jbenet/go-ipfs/crypto"
	nsys "github.com/jbenet/go-ipfs/namesys"
	u "github.com/jbenet/go-ipfs/util"
)

type IpnsEntry struct {
	Name  string
	Value string
}

type ResolveOutput struct {
	Entries []IpnsEntry
}

var errNotOnline = errors.New("This command must be run in online mode. Try running 'ipfs daemon' first.")

var nameCmd = &cmds.Command{
	Help: "TODO",
	Subcommands: map[string]*cmds.Command{
		"publish": publishCmd,
		"resolve": resolveCmd,
	},
}

var publishCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"name", cmds.ArgString, false, false},
		cmds.Argument{"object", cmds.ArgString, true, false},
	},
	Help: "TODO",
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node
		args := req.Arguments()

		if n.Network == nil {
			res.SetError(errNotOnline, cmds.ErrNormal)
			return
		}

		if n.Identity == nil {
			res.SetError(errors.New("Identity not loaded!"), cmds.ErrNormal)
			return
		}

		// name := ""
		ref := ""

		switch len(args) {
		case 2:
			// name = args[0]
			ref = args[1].(string)
			res.SetError(errors.New("keychains not yet implemented"), cmds.ErrNormal)
			return
		case 1:
			// name = n.Identity.ID.String()
			ref = args[0].(string)
		}

		// TODO n.Keychain.Get(name).PrivKey
		k := n.Identity.PrivKey()
		publishOutput, err := publish(n, k, ref)

		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(publishOutput)
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*IpnsEntry)
			s := fmt.Sprintf("Published name %s to %s\n", v.Name, v.Value)
			return []byte(s), nil
		},
	},
	Type: &IpnsEntry{},
}

var resolveCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"name", cmds.ArgString, false, true},
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

func publish(n *core.IpfsNode, k crypto.PrivKey, ref string) (*IpnsEntry, error) {
	pub := nsys.NewRoutingPublisher(n.Routing)
	err := pub.Publish(k, ref)
	if err != nil {
		return nil, err
	}

	hash, err := k.GetPublic().Hash()
	if err != nil {
		return nil, err
	}

	return &IpnsEntry{
		Name:  u.Key(hash).String(),
		Value: ref,
	}, nil
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
