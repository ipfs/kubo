package commands

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/jbenet/go-ipfs/blocks"
	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
)

func BlockGet(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	k := u.Key(args[0])
	u.PErr("Getting block[%s]\n", k)

	b, err := n.Blocks.GetBlock(k)
	if err != nil {
		return err
	}

	out.Write(b.Data)
	return nil
}

func BlockPut(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	b := blocks.NewBlock(data)
	u.PErr("Putting block[%s]\n", b.Key())

	key, err := n.Blocks.AddBlock(b)
	if err != nil {
		return err
	}

	u.PErr("Done. Key: %s\n", key)

	return nil
}
