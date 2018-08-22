package commands

import (
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	namesys "github.com/ipfs/go-ipfs/namesys"
	nsopts "github.com/ipfs/go-ipfs/namesys/opts"

	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
)

var DNSCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Resolve DNS links.",
		ShortDescription: `
Multihashes are hard to remember, but domain names are usually easy to
remember.  To create memorable aliases for multihashes, DNS TXT
records can point to other DNS links, IPFS objects, IPNS keys, etc.
This command resolves those links to the referenced object.
`,
		LongDescription: `
Multihashes are hard to remember, but domain names are usually easy to
remember.  To create memorable aliases for multihashes, DNS TXT
records can point to other DNS links, IPFS objects, IPNS keys, etc.
This command resolves those links to the referenced object.

For example, with this DNS TXT record:

	> dig +short TXT _dnslink.ipfs.io
	dnslink=/ipfs/QmRzTuh2Lpuz7Gr39stNr6mTFdqAghsZec1JoUnfySUzcy

The resolver will give:

	> ipfs dns ipfs.io
	/ipfs/QmRzTuh2Lpuz7Gr39stNr6mTFdqAghsZec1JoUnfySUzcy

The resolver can recursively resolve:

	> dig +short TXT recursive.ipfs.io
	dnslink=/ipns/ipfs.io
	> ipfs dns -r recursive.ipfs.io
	/ipfs/QmRzTuh2Lpuz7Gr39stNr6mTFdqAghsZec1JoUnfySUzcy
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("domain-name", true, false, "The domain-name name to resolve.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("recursive", "r", "Resolve until the result is not a DNS link."),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		recursive, _, _ := req.Option("recursive").Bool()
		name := req.Arguments()[0]
		resolver := namesys.NewDNSResolver()

		var ropts []nsopts.ResolveOpt
		if !recursive {
			ropts = append(ropts, nsopts.Depth(1))
		}

		output, err := resolver.Resolve(req.Context(), name, ropts...)
		if err == namesys.ErrResolveFailed {
			res.SetError(err, cmdkit.ErrNotFound)
			return
		}
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		res.SetOutput(&ResolvedPath{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			output, ok := v.(*ResolvedPath)
			if !ok {
				return nil, e.TypeErr(output, v)
			}
			return strings.NewReader(output.Path.String() + "\n"), nil
		},
	},
	Type: ResolvedPath{},
}
