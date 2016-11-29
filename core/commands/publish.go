package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	path "github.com/ipfs/go-ipfs/path"

	multibase "gx/ipfs/QmShp7G5GEsLVZ52imm6VP4nukpc5ipdHbscrxJMNasmSd/go-multibase"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	crypto "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
)

var errNotOnline = errors.New("This command must be run in online mode. Try running 'ipfs daemon' first.")

type UploadResult struct {
	Peer    string
	OldSeq  uint64
	NewSeq  uint64
	NewPath path.Path
}

var UploadNameCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Upload a signed IPNS record to an IPFS node",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipns-rec", true, false, "binary IPNS record").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("key", "Public key of the author who signed the IPNS record"),
	},

	Run: func(req cmds.Request, res cmds.Response) {
		log.Debug("begin name upload")
		n, err := getNodeWithNamesys(req, res)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if len(req.Arguments()) != 1 {
			res.SetError(errors.New("Must provide the IPNS record as the single argument"), cmds.ErrNormal)
			return
		}

		_, record, err := multibase.Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		ctx := req.Context()
		pubkeyString, found, err := req.Option("key").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if !found {
			res.SetError(errors.New("Must provide a public key as the --key option"), cmds.ErrNormal)
			return
		}

		_, pubkeyBytes, err := multibase.Decode(pubkeyString)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		pubkey, err := crypto.UnmarshalPublicKey(pubkeyBytes)
		crypto.MarshalPublicKey(pubkey)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		id, oldSeq, newSeq, newPath, err := n.Namesys.Upload(ctx, pubkey, record)
		res.SetOutput(&UploadResult{Peer: id.Pretty(), OldSeq: oldSeq, NewSeq: newSeq, NewPath: newPath})
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			o := res.Output().(*UploadResult)
			return strings.NewReader(fmt.Sprintf("/ipns/%s was set to %s (old seq=%d, new seq=%d)\n", o.Peer, o.NewPath, o.OldSeq, o.NewSeq)), nil
		},
	},
	Type: UploadResult{},
}

var PublishCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Publish an object to IPNS.",
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

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, false, "ipfs path of the object to be published.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("resolve", "Resolve given path before publishing.").Default(true),
		cmds.StringOption("lifetime", "t",
			`Time duration that the record will be valid for. <<default>>
    This accepts durations such as "300s", "1.5h" or "2h45m". Valid time units are
    "ns", "us" (or "µs"), "ms", "s", "m", "h".`).Default("24h"),
		cmds.StringOption("ttl", "Time duration this record should be cached for (caution: experimental)."),
		cmds.StringOption("key", "k", "name of key to use").Default("self"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		log.Debug("begin publish")
		n, err := getNodeWithNamesys(req, res)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		pstr := req.Arguments()[0]
		popts := new(publishOpts)

		popts.verifyExists, _, _ = req.Option("resolve").Bool()

		validtime, _, _ := req.Option("lifetime").String()
		d, err := time.ParseDuration(validtime)
		if err != nil {
			res.SetError(fmt.Errorf("error parsing lifetime option: %s", err), cmds.ErrNormal)
			return
		}

		popts.pubValidTime = d

		ctx := req.Context()
		if ttl, found, _ := req.Option("ttl").String(); found {
			d, err := time.ParseDuration(ttl)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			ctx = context.WithValue(ctx, "ipns-publish-ttl", d)
		}

		kname, _, _ := req.Option("key").String()
		k, err := n.GetKey(kname)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output, err := publish(ctx, n, k, path.Path(pstr), popts)
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

func getNodeWithNamesys(req cmds.Request, res cmds.Response) (n *core.IpfsNode, err error) {
	n, err = req.InvocContext().GetNode()
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	if !n.OnlineMode() {
		err = n.SetupOfflineRouting()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	}

	if n.Mounts.Ipns != nil && n.Mounts.Ipns.IsActive() {
		err = errors.New("You cannot manually publish while IPNS is mounted.")
		res.SetError(err, cmds.ErrNormal)
		return
	}

	if n.Identity == "" {
		err = errors.New("Identity not loaded!")
		res.SetError(err, cmds.ErrNormal)
	}

	return
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
