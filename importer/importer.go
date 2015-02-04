// package importer implements utilities used to create ipfs DAGs from files
// and readers
package importer

import (
	"fmt"
	"io"
	"os"

	bal "github.com/jbenet/go-ipfs/importer/balanced"
	"github.com/jbenet/go-ipfs/importer/chunk"
	h "github.com/jbenet/go-ipfs/importer/helpers"
	trickle "github.com/jbenet/go-ipfs/importer/trickle"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/pin"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("importer")

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

	return BuildDagFromReader(f, ds, mp, chunk.DefaultSplitter)
}

func BuildDagFromReader(r io.Reader, ds dag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*dag.Node, error) {
	// Start the splitter
	blkch := spl.Split(r)

	dbp := h.DagBuilderParams{
		Dagserv:  ds,
		Maxlinks: h.DefaultLinksPerBlock,
		Pinner:   mp,
	}

	return bal.BalancedLayout(dbp.New(blkch))
}

func BuildTrickleDagFromReader(r io.Reader, ds dag.DAGService, mp pin.ManualPinner, spl chunk.BlockSplitter) (*dag.Node, error) {
	// Start the splitter
	blkch := spl.Split(r)

	dbp := h.DagBuilderParams{
		Dagserv:  ds,
		Maxlinks: h.DefaultLinksPerBlock,
		Pinner:   mp,
	}

	return trickle.TrickleLayout(dbp.New(blkch))
}
