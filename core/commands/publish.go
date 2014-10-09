package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"

	nsys "github.com/jbenet/go-ipfs/namesys"
)

func Publish(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	log.Debug("Begin Publish")

	if n.Identity == nil {
		return errors.New("Identity not loaded!")
	}

	// name := ""
	ref := ""

	switch len(args) {
	case 2:
		// name = args[0]
		ref = args[1]
		return errors.New("keychains not yet implemented")
	case 1:
		// name = n.Identity.ID.String()
		ref = args[0]

	default:
		return fmt.Errorf("Publish expects 1 or 2 args; got %d.", len(args))
	}

	// later, n.Keychain.Get(name).PrivKey
	k := n.Identity.PrivKey

	pub := nsys.NewRoutingPublisher(n.Routing)
	err := pub.Publish(k, ref)
	if err != nil {
		return err
	}

	hash, err := k.GetPublic().Hash()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "published name %s to %s\n", u.Key(hash), ref)

	return nil
}
