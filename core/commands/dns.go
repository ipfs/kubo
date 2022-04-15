package commands

import (
	"fmt"
	"io"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	ncmd "github.com/ipfs/go-ipfs/core/commands/name"
	namesys "github.com/ipfs/go-namesys"
	nsopts "github.com/ipfs/interface-go-ipfs-core/options/namesys"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

const (
	dnsRecursiveOptionName = "recursive"
)

var DNSCmd = &cmds.Command{
	Status: cmds.Deprecated, // https://github.com/ipfs/go-ipfs/issues/8607
	Helptext: cmds.HelpText{
		Tagline: "Resolve DNSLink records.",
		ShortDescription: `
This command can only recursively resolve DNSLink TXT records.
It will fail to recursively resolve through IPNS keys etc.

DEPRECATED: superseded by 'ipfs resolve'

For general-purpose recursive resolution, use 'ipfs resolve -r'.
It will work across multiple DNSLinks and IPNS keys.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("domain-name", true, false, "The domain-name name to resolve.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dnsRecursiveOptionName, "r", "Resolve until the result is not a DNS link.").WithDefault(true),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		recursive, _ := req.Options[dnsRecursiveOptionName].(bool)
		name := req.Arguments[0]
		resolver := namesys.NewDNSResolver(node.DNSResolver.LookupTXT)

		var routing []nsopts.ResolveOpt
		if !recursive {
			routing = append(routing, nsopts.Depth(1))
		}

		output, err := resolver.Resolve(req.Context, name, routing...)
		if err != nil && (recursive || err != namesys.ErrResolveRecursion) {
			return err
		}
		return cmds.EmitOnce(res, &ncmd.ResolvedPath{Path: output})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ncmd.ResolvedPath) error {
			fmt.Fprintln(w, cmdenv.EscNonPrint(out.Path.String()))
			return nil
		}),
	},
	Type: ncmd.ResolvedPath{},
}
