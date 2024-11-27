package name

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/boxo/path"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

var log = logging.Logger("core/commands/ipns")

type ResolvedPath struct {
	Path string
}

const (
	recursiveOptionName      = "recursive"
	nocacheOptionName        = "nocache"
	dhtRecordCountOptionName = "dht-record-count"
	dhtTimeoutOptionName     = "dht-timeout"
	streamOptionName         = "stream"
)

var IpnsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Resolve IPNS names.",
		ShortDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default name used is the node's own PeerID,
which is the hash of its public key.
`,
		LongDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default name used is the node's own PeerID,
which is the hash of its public key.

You can use the 'ipfs key' commands to list and generate more names and their
respective keys.

Examples:

Resolve the value of your name:

  > ipfs name resolve
  /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve the value of another name:

  > ipfs name resolve QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
  /ipfs/QmSiTko9JZyabH56y2fussEt1A5oDqsFXB3CkvAqraFryz

Resolve the value of a dnslink:

  > ipfs name resolve ipfs.io
  /ipfs/QmaBvfZooxWkrv7D3r8LS9moNjzD2o525XMZze69hhoxf5

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("name", false, false, "The IPNS name to resolve. Defaults to your node's peerID."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(recursiveOptionName, "r", "Resolve until the result is not an IPNS name.").WithDefault(true),
		cmds.BoolOption(nocacheOptionName, "n", "Do not use cached entries."),
		cmds.UintOption(dhtRecordCountOptionName, "dhtrc", "Number of records to request for DHT resolution.").WithDefault(uint(namesys.DefaultResolverDhtRecordCount)),
		cmds.StringOption(dhtTimeoutOptionName, "dhtt", "Max time to collect values during DHT resolution e.g. \"30s\". Pass 0 for no timeout.").WithDefault(namesys.DefaultResolverDhtTimeout.String()),
		cmds.BoolOption(streamOptionName, "s", "Stream entries as they are found."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		nocache, _ := req.Options["nocache"].(bool)

		var name string
		if len(req.Arguments) == 0 {
			self, err := api.Key().Self(req.Context)
			if err != nil {
				return err
			}
			name = self.ID().String()
		} else {
			name = req.Arguments[0]
		}

		recursive, _ := req.Options[recursiveOptionName].(bool)
		rc, rcok := req.Options[dhtRecordCountOptionName].(uint)
		dhtt, dhttok := req.Options[dhtTimeoutOptionName].(string)
		stream, _ := req.Options[streamOptionName].(bool)

		opts := []options.NameResolveOption{
			options.Name.Cache(!nocache),
		}

		if !recursive {
			opts = append(opts, options.Name.ResolveOption(namesys.ResolveWithDepth(1)))
		}
		if rcok {
			opts = append(opts, options.Name.ResolveOption(namesys.ResolveWithDhtRecordCount(rc)))
		}
		if dhttok {
			d, err := time.ParseDuration(dhtt)
			if err != nil {
				return err
			}
			if d < 0 {
				return errors.New("DHT timeout value must be >= 0")
			}
			opts = append(opts, options.Name.ResolveOption(namesys.ResolveWithDhtTimeout(d)))
		}

		if !strings.HasPrefix(name, "/ipns/") {
			name = "/ipns/" + name
		}

		if !stream {
			output, err := api.Name().Resolve(req.Context, name, opts...)
			if err != nil && (recursive || err != namesys.ErrResolveRecursion) {
				return err
			}

			pth, err := path.NewPath(output.String())
			if err != nil {
				return err
			}

			return cmds.EmitOnce(res, &ResolvedPath{pth.String()})
		}

		output, err := api.Name().Search(req.Context, name, opts...)
		if err != nil {
			return err
		}

		for v := range output {
			if v.Err != nil && (recursive || v.Err != namesys.ErrResolveRecursion) {
				return v.Err
			}
			if err := res.Emit(&ResolvedPath{v.Path.String()}); err != nil {
				return err
			}

		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, rp *ResolvedPath) error {
			_, err := fmt.Fprintln(w, rp.Path)
			return err
		}),
	},
	Type: ResolvedPath{},
}
