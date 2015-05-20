package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	crypto "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
	u "github.com/ipfs/go-ipfs/util"
)

var errNotOnline = errors.New("This command must be run in online mode. Try running 'ipfs daemon' first.")

var publishCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Publish an object to IPNS",
		ShortDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In publish, the
default value of <name> is your own identity public key.
`,
		LongDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In publish, the
default value of <name> is your own identity public key.

Examples:

Publish an <ipfs-path> to your identity name:

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Publish an <ipfs-path> to another public key (not implemented):

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("name", false, false, "The IPNS name to publish to. Defaults to your node's peerID"),
		cmds.StringArg("ipfs-path", true, false, "IPFS path of the obejct to be published at <name>").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		log.Debug("Begin Publish")
		n, err := req.Context().GetNode()
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

		args := req.Arguments()

		if n.Identity == "" {
			res.SetError(errors.New("Identity not loaded!"), cmds.ErrNormal)
			return
		}

		var pstr string

		switch len(args) {
		case 2:
			// name = args[0]
			pstr = args[1]
			res.SetError(errors.New("keychains not yet implemented"), cmds.ErrNormal)
			return
		case 1:
			// name = n.Identity.ID.String()
			pstr = args[0]
		}

		p, err := path.ParsePath(pstr)
		if err != nil {
			res.SetError(fmt.Errorf("failed to validate path: %v", err), cmds.ErrNormal)
			return
		}

		// TODO n.Keychain.Get(name).PrivKey
		// TODO(cryptix): is req.Context().Context a child of n.Context()?
		output, err := publish(req.Context().Context, n, n.PrivateKey, p)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v := res.Output().(*IpnsEntry)
			s := fmt.Sprintf("Published to %s: %s\n", v.Name, v.Value)
			return strings.NewReader(s), nil
		},
	},
	Type: IpnsEntry{},
}

func publish(ctx context.Context, n *core.IpfsNode, k crypto.PrivKey, ref path.Path) (*IpnsEntry, error) {
	// First, verify the path exists
	_, err := core.Resolve(ctx, n, ref)
	if err != nil {
		return nil, err
	}

	err = n.Namesys.Publish(ctx, k, ref)
	if err != nil {
		return nil, err
	}

	hash, err := k.GetPublic().Hash()
	if err != nil {
		return nil, err
	}

	return &IpnsEntry{
		Name:  u.Key(hash).String(),
		Value: ref.String(),
	}, nil
}
