package commands

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	"github.com/jbenet/go-ipfs/blocks"
	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
)

// BlockGet retrives a raw ipfs block from the node's BlockService
func BlockGet(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {

	if !u.IsValidHash(args[0]) {
		return fmt.Errorf("block get: not a valid hash")
	}

	h, err := mh.FromB58String(args[0])
	if err != nil {
		return fmt.Errorf("block get: %v", err)
	}

	k := u.Key(h)
	log.Debugf("BlockGet key: '%q'", k)
	ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
	b, err := n.Blocks.GetBlock(ctx, k)
	if err != nil {
		return fmt.Errorf("block get: %v", err)
	}

	_, err = out.Write(b.Data)
	return err
}

// BlockPut reads everything from conn and saves the data to the nodes BlockService
func BlockPut(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	// TODO: this should read from an io.Reader arg
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	b := blocks.NewBlock(data)
	log.Debugf("BlockPut key: '%q'", b.Key())

	k, err := n.Blocks.AddBlock(b)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "added as '%s'\n", k)

	return nil
}
