package commands

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	balanced "github.com/ipfs/go-ipfs/importer/balanced"
	ihelper "github.com/ipfs/go-ipfs/importer/helpers"

	mh "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	chunk "gx/ipfs/QmXnzH7wowyLZy8XJxxaQCVTgLMcDXdMBznmsrmQWCyiQV/go-ipfs-chunker"
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
)

var UrlStoreCmd = &cmds.Command{

	Subcommands: map[string]*cmds.Command{
		"add": urlAdd,
	},
}

var urlAdd = &cmds.Command{
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("url", true, false, "URL to add to IPFS"),
	},
	Type: BlockStat{},

	Run: func(req cmds.Request, res cmds.Response) {
		url := req.Arguments()[0]
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		hreq, err := http.NewRequest("GET", url, nil)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		hres, err := http.DefaultClient.Do(hreq)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if hres.StatusCode != http.StatusOK {
			res.SetError(fmt.Errorf("expected code 200, got: %d", hres.StatusCode), cmdkit.ErrNormal)
			return
		}

		chk := chunk.NewSizeSplitter(hres.Body, chunk.DefaultBlockSize)
		prefix := cid.NewPrefixV1(cid.DagProtobuf, mh.SHA2_256)
		dbp := &ihelper.DagBuilderParams{
			Dagserv:   n.DAG,
			RawLeaves: true,
			Maxlinks:  ihelper.DefaultLinksPerBlock,
			NoCopy:    true,
			Prefix:    &prefix,
			URL:       url,
		}

		blc, err := balanced.Layout(dbp.New(chk))
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(BlockStat{
			Key:  blc.Cid().String(),
			Size: int(hres.ContentLength),
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			ch := res.Output().(<-chan interface{})
			bs0 := <-ch
			bs := bs0.(*BlockStat)
			return strings.NewReader(bs.Key + "\n"), nil
		},
	},
}
