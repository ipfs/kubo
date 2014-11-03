package commands

import (
	"errors"
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	nsys "github.com/jbenet/go-ipfs/namesys"
	u "github.com/jbenet/go-ipfs/util"
)

type PublishOutput struct {
	Name, Value string
}

var publishCmd = &cmds.Command{
	Help: "TODO",
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node
		args := req.Arguments()

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

		default:
			res.SetError(fmt.Errorf("Publish expects 1 or 2 args; got %d.", len(args)), cmds.ErrClient)
		}
		// later, n.Keychain.Get(name).PrivKey
		k := n.Identity.PrivKey()

		pub := nsys.NewRoutingPublisher(n.Routing)
		err := pub.Publish(k, ref)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		hash, err := k.GetPublic().Hash()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&PublishOutput{
			Name:  u.Key(hash).String(),
			Value: ref,
		})
	},
	Format: func(res cmds.Response) (string, error) {
		v := res.Output().(*PublishOutput)
		return fmt.Sprintf("Published name %s to %s\n", v.Name, v.Value), nil
	},
	Type: &PublishOutput{},
}
