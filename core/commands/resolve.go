package commands

import (
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
)

func Resolve(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	res, err := n.Namesys.Resolve(args[0])
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s -> %s\n", args[0], res)
	return nil
}
