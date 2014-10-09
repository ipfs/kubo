package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
)

func Resolve(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {

	name := ""

	switch len(args) {
	case 1:
		name = args[0]
	case 0:
		if n.Identity == nil {
			return errors.New("Identity not loaded!")
		}
		name = n.Identity.ID.String()

	default:
		return fmt.Errorf("Publish expects 1 or 2 args; got %d.", len(args))
	}

	res, err := n.Namesys.Resolve(name)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s\n", res)
	return nil
}
