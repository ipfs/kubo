package commands

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	balanced "github.com/ipfs/go-ipfs/importer/balanced"
	ihelper "github.com/ipfs/go-ipfs/importer/helpers"

        chunk "gx/ipfs/QmbGDSVKnYJZrtUnyxwsUpCeuigshNuVFxXCpv13jXecq1/go-ipfs-chunker"
	mh "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
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
			bs := res.Output().(*BlockStat)
			return strings.NewReader(bs.Key + "\n"), nil
		},
	},
}
