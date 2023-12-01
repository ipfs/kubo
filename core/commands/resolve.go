package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	ns "github.com/ipfs/boxo/namesys"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	ncmd "github.com/ipfs/kubo/core/commands/name"

	"github.com/ipfs/boxo/path"
	cmds "github.com/ipfs/go-ipfs-cmds"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

const (
	resolveRecursiveOptionName      = "recursive"
	resolveDhtRecordCountOptionName = "dht-record-count"
	resolveDhtTimeoutOptionName     = "dht-timeout"
)

var ResolveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Resolve the value of names to IPFS.",
		ShortDescription: `
There are a number of mutable name protocols that can link among
themselves and into IPNS. This command accepts any of these
identifiers and resolves them to the referenced item.
`,
		LongDescription: `
There are a number of mutable name protocols that can link among
themselves and into IPNS. For example IPNS references can (currently)
point at an IPFS object, and DNS links can point at other DNS links, IPNS
entries, or IPFS objects. This command accepts any of these
identifiers and resolves them to the referenced item.

EXAMPLES

Resolve the value of your identity:

  $ ipfs resolve /ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

Resolve the value of another name:

  $ ipfs resolve /ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  /ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve the value of another name recursively:

  $ ipfs resolve -r /ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

Resolve the value of an IPFS DAG path:

  $ ipfs resolve /ipfs/QmeZy1fGbwgVSrqbfh9fKQrAWgeyRnj7h8fsHS1oy3k99x/beep/boop
  /ipfs/QmYRMjyvAiHKN9UTi8Bzt1HUspmSRD8T8DwxfSMzLgBon1

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "The name to resolve.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(resolveRecursiveOptionName, "r", "Resolve until the result is an IPFS name.").WithDefault(true),
		cmds.IntOption(resolveDhtRecordCountOptionName, "dhtrc", "Number of records to request for DHT resolution."),
		cmds.StringOption(resolveDhtTimeoutOptionName, "dhtt", "Max time to collect values during DHT resolution e.g. \"30s\". Pass 0 for no timeout."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		name := req.Arguments[0]
		recursive, _ := req.Options[resolveRecursiveOptionName].(bool)

		// the case when ipns is resolved step by step
		if strings.HasPrefix(name, "/ipns/") && !recursive {
			rc, rcok := req.Options[resolveDhtRecordCountOptionName].(uint)
			dhtt, dhttok := req.Options[resolveDhtTimeoutOptionName].(string)
			ropts := []options.NameResolveOption{
				options.Name.ResolveOption(ns.ResolveWithDepth(1)),
			}

			if rcok {
				ropts = append(ropts, options.Name.ResolveOption(ns.ResolveWithDhtRecordCount(rc)))
			}
			if dhttok {
				d, err := time.ParseDuration(dhtt)
				if err != nil {
					return err
				}
				if d < 0 {
					return errors.New("DHT timeout value must be >= 0")
				}
				ropts = append(ropts, options.Name.ResolveOption(ns.ResolveWithDhtTimeout(d)))
			}
			p, err := api.Name().Resolve(req.Context, name, ropts...)
			// ErrResolveRecursion is fine
			if err != nil && err != ns.ErrResolveRecursion {
				return err
			}
			return cmds.EmitOnce(res, &ncmd.ResolvedPath{Path: p.String()})
		}

		var enc cidenc.Encoder
		switch {
		case !cmdenv.CidBaseDefined(req) && !strings.HasPrefix(name, "/ipns/"):
			// Not specified, check the path.
			enc, err = cmdenv.CidEncoderFromPath(name)
			if err == nil {
				break
			}
			// Nope, fallback on the default.
			fallthrough
		default:
			enc, err = cmdenv.GetCidEncoder(req)
			if err != nil {
				return err
			}
		}

		p, err := cmdutils.PathOrCidPath(name)
		if err != nil {
			return err
		}

		// else, ipfs path or ipns with recursive flag
		rp, remainder, err := api.ResolvePath(req.Context, p)
		if err != nil {
			return err
		}

		// Trick to encode path with correct encoding.
		encodedPath := "/" + rp.Namespace() + "/" + enc.Encode(rp.RootCid())
		if len(remainder) != 0 {
			encodedPath += path.SegmentsToString(remainder...)
		}

		// Ensure valid and sanitized.
		ep, err := path.NewPath(encodedPath)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ncmd.ResolvedPath{Path: ep.String()})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, rp *ncmd.ResolvedPath) error {
			fmt.Fprintln(w, rp.Path)
			return nil
		}),
	},
	Type: ncmd.ResolvedPath{},
}
