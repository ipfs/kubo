package name

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	keystore "github.com/ipfs/go-ipfs/keystore"

	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	path "github.com/ipfs/go-path"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
)

var PublishCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Publish IPNS names.",
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

Publish an <ipfs-path> with your default name:

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Publish an <ipfs-path> with another name, added by an 'ipfs key' command:

  > ipfs key gen --type=rsa --size=2048 mykey
  > ipfs name publish --key=mykey /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmSrPmbaUKA3ZodhzPWZnpFgcPMFWF4QsxXbkWfEptTBJd: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Alternatively, publish an <ipfs-path> using a valid PeerID (as listed by 
'ipfs key list -l'):

 > ipfs name publish --key=QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, false, "ipfs path of the object to be published.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("resolve", "Resolve given path before publishing.").WithDefault(true),
		cmdkit.StringOption("lifetime", "t",
			`Time duration that the record will be valid for. <<default>>
    This accepts durations such as "300s", "1.5h" or "2h45m". Valid time units are
    "ns", "us" (or "µs"), "ms", "s", "m", "h".`).WithDefault("24h"),
		cmdkit.StringOption("ttl", "Time duration this record should be cached for (caution: experimental)."),
		cmdkit.StringOption("key", "k", "Name of the key to be used or a valid PeerID, as listed by 'ipfs key list -l'. Default: <<default>>.").WithDefault("self"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			err := n.SetupOfflineRouting()
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		if n.Mounts.Ipns != nil && n.Mounts.Ipns.IsActive() {
			res.SetError(errors.New("cannot manually publish while IPNS is mounted"), cmdkit.ErrNormal)
			return
		}

		pstr := req.Arguments[0]

		if n.Identity == "" {
			res.SetError(errors.New("identity not loaded"), cmdkit.ErrNormal)
			return
		}

		popts := new(publishOpts)

		popts.verifyExists, _ = req.Options["resolve"].(bool)

		validtime, _ := req.Options["lifetime"].(string)
		d, err := time.ParseDuration(validtime)
		if err != nil {
			res.SetError(fmt.Errorf("error parsing lifetime option: %s", err), cmdkit.ErrNormal)
			return
		}

		popts.pubValidTime = d

		ctx := req.Context
		if ttl, found := req.Options["ttl"].(string); found {
			d, err := time.ParseDuration(ttl)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			ctx = context.WithValue(ctx, "ipns-publish-ttl", d)
		}

		kname, _ := req.Options["key"].(string)
		k, err := keylookup(n, kname)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		pth, err := path.ParsePath(pstr)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		output, err := publish(ctx, n, k, pth, popts)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		cmds.EmitOnce(res, output)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			entry, ok := v.(*IpnsEntry)
			if !ok {
				return e.TypeErr(entry, v)
			}

			_, err := fmt.Fprintf(w, "Published to %s: %s\n", entry.Name, entry.Value)
			return err
		}),
	},
	Type: IpnsEntry{},
}

type publishOpts struct {
	verifyExists bool
	pubValidTime time.Duration
}

func publish(ctx context.Context, n *core.IpfsNode, k crypto.PrivKey, ref path.Path, opts *publishOpts) (*IpnsEntry, error) {

	if opts.verifyExists {
		// verify the path exists
		_, err := core.Resolve(ctx, n.Namesys, n.Resolver, ref)
		if err != nil {
			return nil, err
		}
	}

	eol := time.Now().Add(opts.pubValidTime)
	err := n.Namesys.PublishWithEOL(ctx, k, ref, eol)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return nil, err
	}

	return &IpnsEntry{
		Name:  pid.Pretty(),
		Value: ref.String(),
	}, nil
}

func keylookup(n *core.IpfsNode, k string) (crypto.PrivKey, error) {

	res, err := n.GetKey(k)
	if res != nil {
		return res, nil
	}

	if err != nil && err != keystore.ErrNoSuchKey {
		return nil, err
	}

	keys, err := n.Repo.Keystore().List()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		privKey, err := n.Repo.Keystore().Get(key)
		if err != nil {
			return nil, err
		}

		pubKey := privKey.GetPublic()

		pid, err := peer.IDFromPublicKey(pubKey)
		if err != nil {
			return nil, err
		}

		if pid.Pretty() == k {
			return privKey, nil
		}
	}

	return nil, fmt.Errorf("no key by the given name or PeerID was found")
}
