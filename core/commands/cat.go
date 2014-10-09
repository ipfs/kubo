package commands

import (
	"fmt"
	"io"

	"github.com/jbenet/go-ipfs/core"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
)

func Cat(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	for _, fn := range args {
		dagnode, err := n.Resolver.ResolvePath(fn)
		if err != nil {
			return fmt.Errorf("catFile error: %v", err)
		}

		read, err := uio.NewDagReader(dagnode, n.DAG)
		if err != nil {
			return fmt.Errorf("cat error: %v", err)
		}

		_, err = io.Copy(out, read)
		if err != nil {
			return fmt.Errorf("cat error: %v", err)
		}
	}
	return nil
}
