package commands

import (
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
)

func Pin(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {

	// set recursive flag
	recursive, _ := opts["r"].(bool) // false if cast fails.

	// if recursive, set depth flag
	depth := 1 // default (non recursive)
	if d, ok := opts["d"].(int); recursive && ok {
		depth = d
	}
	if depth < -1 {
		return fmt.Errorf("ipfs pin: called with invalid depth: %v", depth)
	}

	fmt.Printf("recursive, depth: %v, %v\n", recursive, depth)

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
