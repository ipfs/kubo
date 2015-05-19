// package importer implements utilities used to create ipfs DAGs from files
// and readers
package importer

import (
	"fmt"
	"io"
	"os"

	bal "github.com/ipfs/go-ipfs/importer/balanced"
	"github.com/ipfs/go-ipfs/importer/chunk"
	h "github.com/ipfs/go-ipfs/importer/helpers"
	trickle "github.com/ipfs/go-ipfs/importer/trickle"
	dag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	u "github.com/ipfs/go-ipfs/util"
)

var log = u.Logger("importer")

// Builds a DAG from the given file, writing created blocks to disk as they are
// created
func BuildDagFromFile(fpath string, ds dag.DAGService, mp pin.ManualPinner) (*dag.Node, error) {
	stat, err := os.Stat(fpath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("`%s` is a directory", fpath)
	}

	f, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return BuildDagFromReader(f, ds, chunk.DefaultSplitter, BasicPinnerCB(mp))
}

func BuildDagFromReader(r io.Reader, ds dag.DAGService, spl chunk.BlockSplitter, bcb h.BlockCB) (*dag.Node, error) {
	// Start the splitter
	blkch := spl.Split(r)

	dbp := h.DagBuilderParams{
		Dagserv:  ds,
		Maxlinks: h.DefaultLinksPerBlock,
		BlockCB:  bcb,
	}

	return bal.BalancedLayout(dbp.New(blkch))
}

func BuildTrickleDagFromReader(r io.Reader, ds dag.DAGService, spl chunk.BlockSplitter, bcb h.BlockCB) (*dag.Node, error) {
	// Start the splitter
	blkch := spl.Split(r)

	dbp := h.DagBuilderParams{
		Dagserv:  ds,
		Maxlinks: h.DefaultLinksPerBlock,
		BlockCB:  bcb,
	}

	return trickle.TrickleLayout(dbp.New(blkch))
}

func BasicPinnerCB(p pin.ManualPinner) h.BlockCB {
	return func(k u.Key, root bool) error {
		if root {
			p.PinWithMode(k, pin.Recursive)
			return p.Flush()
		} else {
			p.PinWithMode(k, pin.Indirect)
			return nil
		}
	}
}

func PinIndirectCB(p pin.ManualPinner) h.BlockCB {
	return func(k u.Key, root bool) error {
		p.PinWithMode(k, pin.Indirect)
		return nil
	}
}
