package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	key "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	crypto "github.com/ipfs/go-ipfs/p2p/crypto"
	path "github.com/ipfs/go-ipfs/path"
)

var errNotOnline = errors.New("This command must be run in online mode. Try running 'ipfs daemon' first.")

var PublishCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Publish an object to IPNS",
		ShortDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In publish, the
default value of <privkey> is your own identity private key.
`,
		LongDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In publish, the
default value of <privkey> is your own identity private key.

Examples:

Publish an <ipfs-path> to your identity name:

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Publish an <ipfs-path> to a <privkey>:

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy CAAS4AQwggJcAgEAAoGBALksWw8SwEX0DJZOkzbB597OVHkzFb8zW5LoX5Vx658ZYR9R1T5qRXa4wLegD1BPBnnfqkcbEN+Wweheib0w1i7u7zYKMyz9xwRcCPHh4eL8oCg2r3DX8rWUYvZne34bnFqnbObVIRx4pmPJXC5rNsdqTG4Dg9P8+/UXuBev9aorAgMBAAECgYEAoYVtUHKcwOgmap3Tj7oIVbNIwAeteoCD6ltDtQoP61GqBDXPeogcW3jAseuuL/EexwQwdaHIUCAiuFxubVbCG+Z3tK5fS/zHvl4mQp5THjbN36dqmgu2gpZNU+P+K45+x+1Mj+1VW0pRUFsm3FbPQQIYalz7gkc9P52AG27pubECQQDXc5uob7wbgG3H5Ca3do3g/3k2CMbVyQ0R6qIWP0v+q6z85rvftbj4UFzFDM0XuWINah6CXeaue3anmL0hKjKHAkEA3AXybn/fWfhLn49vB2Re9dTVonJeyE+fel1qI1nzaluGqSbNxuvUSArjro5q23DAGAMZCNZ4J2jTzD1xZqNgPQJAF27jhzZf5z3Yst0FuP6T/9zJei8KMUZkvYYfivvncBOMBRWzaWmCbL+Q133E8Meg+oSIPPWpmWCkTyY1q93DEQJAUSeMZT+bNYdE9YSlUleuQwSPDA0dcssTqsG7/XAXPZqmz8t1STMBKNWDZ4Y2Wdx7rh+uYzkgNoEO5h2fr1kBjQJARpRKE5vWnChCt8s6mibpjeuZM7ZbILl+b82Cq2N2KqKflBA3Al4XnZDfg2XCM7CcPMJSdNWbbm44qt0Q3z0DAA==
  Published to QmdeadvG5GdPWiea2h7jAsZdVyxawY714KDWSDBAaRgkAD: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

TODO: Publish an <ipfs-path> to a public key (in the <privkey> arg) that has a private key in the keystore:

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy QmdeadvG5GdPWiea2h7jAsZdVyxawY714KDWSDBAaRgkAD --keystore
  Published to QmdeadvG5GdPWiea2h7jAsZdVyxawY714KDWSDBAaRgkAD: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, false, "IPFS path of the object to be published").EnableStdin(),
		cmds.StringArg("privkey", false, false, "The private key to publish (default is your own key)"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("resolve", "resolve given path before publishing (default=true)"),
		// example: ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy QmdeadvG5GdPWiea2h7jAsZdVyxawY714KDWSDBAaRgkAD --keystore
		// Or, privkey could be turned into a string option, mutually exclusive with --keystore as a string option.
		// cmds.BoolOption("keystore", "Treat privkey as a public key, getting the real private key from the keystore."),
		// Another option is for it to try parsing it as a multihash first (checking length to be sure) and looking for it in the keystore.
		// That way both options could work transparently.
		cmds.StringOption("lifetime", "t", "time duration that the record will be valid for (default: 24hrs)"),
		cmds.StringOption("ttl", "time duration this record should be cached for (caution: experimental)"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		log.Debug("Begin Publish")
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

		pstr := req.Arguments()[0]
		
		if n.Identity == "" {
			res.SetError(errors.New("Identity not loaded!"), cmds.ErrNormal)
			return
		}
		
		privkey := n.PrivateKey

		if len(req.Arguments()) != 1 {
			data, err := crypto.ConfigDecodeKey(req.Arguments()[1])
			if err != nil {
				res.SetError(fmt.Errorf("error decoding privkey: %s", err), cmds.ErrNormal)
				return
			}
			privkey, err = crypto.UnmarshalPrivateKey(data)
			if err != nil {
				res.SetError(fmt.Errorf("error unmarshalling privkey: %s", err), cmds.ErrNormal)
				return
			}
		}

		popts := &publishOpts{
			verifyExists: true,
			pubValidTime: time.Hour * 24,
		}

		verif, found, _ := req.Option("resolve").Bool()
		if found {
			popts.verifyExists = verif
		}
		validtime, found, _ := req.Option("lifetime").String()
		if found {
			d, err := time.ParseDuration(validtime)
			if err != nil {
				res.SetError(fmt.Errorf("error parsing lifetime option: %s", err), cmds.ErrNormal)
				return
			}

			popts.pubValidTime = d
		}

		ctx := req.Context()
		if ttl, found, _ := req.Option("ttl").String(); found {
			d, err := time.ParseDuration(ttl)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			ctx = context.WithValue(ctx, "ipns-publish-ttl", d)
		}

		output, err := publish(ctx, n, privkey, path.Path(pstr), popts)
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

type publishOpts struct {
	verifyExists bool
	pubValidTime time.Duration
}

func publish(ctx context.Context, n *core.IpfsNode, k crypto.PrivKey, ref path.Path, opts *publishOpts) (*IpnsEntry, error) {

	if opts.verifyExists {
		// verify the path exists
		_, err := core.Resolve(ctx, n, ref)
		if err != nil {
			return nil, err
		}
	}

	eol := time.Now().Add(opts.pubValidTime)
	err := n.Namesys.PublishWithEOL(ctx, k, ref, eol)
	if err != nil {
		return nil, err
	}

	hash, err := k.GetPublic().Hash()
	if err != nil {
		return nil, err
	}

	return &IpnsEntry{
		Name:  key.Key(hash).String(),
		Value: ref.String(),
	}, nil
}
