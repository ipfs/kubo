package commands

import (
	"fmt"
	"io"
	"net/url"

	filestore "github.com/ipfs/go-filestore"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/options"
)

var urlStoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with urlstore.",
	},
	Subcommands: map[string]*cmds.Command{
		"add": urlAdd,
	},
}

var urlAdd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add URL via urlstore.",
		LongDescription: `
DEPRECATED: Use 'ipfs add --nocopy --cid-version=1 URL'.

Add URLs to ipfs without storing the data locally.

The URL provided must be stable and ideally on a web server under your
control.

The file is added using raw-leaves but otherwise using the default
settings for 'ipfs add'.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(trickleOptionName, "t", "Use trickle-dag format for dag generation."),
		cmds.BoolOption(pinOptionName, "Pin this object when adding.").WithDefault(true),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("url", true, false, "URL to add to IPFS"),
	},
	Type: &BlockStat{},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		log.Error("The 'ipfs urlstore' command is deprecated, please use 'ipfs add --nocopy --cid-version=1")

		urlString := req.Arguments[0]
		if !filestore.IsURL(req.Arguments[0]) {
			return fmt.Errorf("unsupported url syntax: %s", urlString)
		}

		url, err := url.Parse(urlString)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		useTrickledag, _ := req.Options[trickleOptionName].(bool)
		dopin, _ := req.Options[pinOptionName].(bool)

		opts := []options.UnixfsAddOption{
			options.Unixfs.Pin(dopin),
			options.Unixfs.CidVersion(1),
			options.Unixfs.RawLeaves(true),
			options.Unixfs.Nocopy(true),
		}

		if useTrickledag {
			opts = append(opts, options.Unixfs.Layout(options.TrickleLayout))
		}

		file := files.NewWebFile(url)

		path, err := api.Unixfs().Add(req.Context, file, opts...)
		if err != nil {
			return err
		}
		size, _ := file.Size()
		return cmds.EmitOnce(res, &BlockStat{
			Key:  enc.Encode(path.Cid()),
			Size: int(size),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, bs *BlockStat) error {
			_, err := fmt.Fprintln(w, bs.Key)
			return err
		}),
	},
}
