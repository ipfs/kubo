package commands

import (
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
)

func Pin(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {

	// if recursive, set flag
	depth := 1
	if r, ok := opts["r"].(bool); r && ok {
		depth = -1
	}

	for _, fn := range args {
		dagnode, err := n.Resolver.ResolvePath(fn)
		if err != nil {
			return fmt.Errorf("pin error: %v", err)
		}

		err = n.PinDagNodeRecursively(dagnode, depth)
		if err != nil {
			return fmt.Errorf("pin: %v", err)
		}
	}
	return nil
}
