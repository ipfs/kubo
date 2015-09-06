package commands

import (
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	namesys "github.com/ipfs/go-ipfs/namesys"
	path "github.com/ipfs/go-ipfs/path"
	u "github.com/ipfs/go-ipfs/util"
)

type ResolvedPath struct {
	Path path.Path
}

var ResolveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Resolve the value of names to IPFS",
		ShortDescription: `
There are a number of mutable name protocols that can link among
themselves and into IPNS.  This command accepts any of these
identifiers and resolves them to the referenced item.
`,
		LongDescription: `
There are a number of mutable name protocols that can link among
themselves and into IPNS.  For example IPNS references can (currently)
point at IPFS object, and DNS links can point at other DNS links, IPNS
entries, or IPFS objects.  This command accepts any of these
identifiers and resolves them to the referenced item.

Examples:

Resolve the value of your identity:

  > ipfs resolve /ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

Resolve the value of another name:

  > ipfs resolve /ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  /ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve the value of another name recursively:

  > ipfs resolve -r /ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  /ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj

Resolve the value of an IPFS DAG path:

  > ipfs resolve /ipfs/QmeZy1fGbwgVSrqbfh9fKQrAWgeyRnj7h8fsHS1oy3k99x/beep/boop
  /ipfs/QmYRMjyvAiHKN9UTi8Bzt1HUspmSRD8T8DwxfSMzLgBon1

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "The name to resolve.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Resolve until the result is an IPFS name"),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			err := n.SetupOfflineRouting()
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		name := req.Arguments()[0]
		recursive, _, _ := req.Option("recursive").Bool()
		depth := 1
		if recursive {
			depth = namesys.DefaultDepthLimit
		}

		if strings.HasPrefix(name, "/ipfs/") || !strings.HasPrefix(name, "/") {
			resolved, err := resolveIpfsPath(req.Context(), n.Resolver, name)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&ResolvedPath{resolved})
			return
		}

		output, err := n.Namesys.ResolveN(req.Context(), name, depth)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&ResolvedPath{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			output, ok := res.Output().(*ResolvedPath)
			if !ok {
				return nil, u.ErrCast()
			}
			return strings.NewReader(output.Path.String()), nil
		},
	},
	Type: ResolvedPath{},
}

func resolveIpfsPath (ctx context.Context, r *path.Resolver, name string) (path.Path, error) {
	p, err := path.ParsePath(name)
	if err != nil {
		return "", err
	}

	node, err := r.ResolvePath(ctx, p)
	if err != nil {
		return "", err
	}

	key, err := node.Key()
	if err != nil {
		return "", err
	}

	return path.FromKey(key), nil
}
