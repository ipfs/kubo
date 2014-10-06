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

	k := n.Identity.PrivKey

	pub := nsys.NewPublisher(n.DAG, n.Routing)
	err := pub.Publish(k, args[0])
	if err != nil {
		return err
	}

	hash, err := k.GetPublic().Hash()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "published mapping %s to %s\n", u.Key(hash), args[0])

	return nil
}
