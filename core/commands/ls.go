package commands

import (
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
)

func Ls(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	for _, fn := range args {
		dagnode, err := n.Resolver.ResolvePath(fn)
		if err != nil {
			return fmt.Errorf("ls error: %v", err)
		}

		for _, link := range dagnode.Links {
			fmt.Fprintf(out, "%s %d %s\n", link.Hash.B58String(), link.Size, link.Name)
		}
	}
	return nil
}
