package commands

import (
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
)

func Pin(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	for _, fn := range args {
		dagnode, err := n.Resolver.ResolvePath(fn)
		if err != nil {
			return fmt.Errorf("pin error: %v", err)
		}

		err = n.PinDagNode(dagnode)
		if err != nil {
			return fmt.Errorf("pin: %v", err)
		}
	}
	return nil
}
