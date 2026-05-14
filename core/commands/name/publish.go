package name

import (
	"errors"
	"fmt"
	"io"
	"time"

	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"

	ipns "github.com/ipfs/boxo/ipns"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ke "github.com/ipfs/kubo/core/commands/keyencode"
	iface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

var errAllowOffline = errors.New("can't publish while offline: pass `--allow-offline` to override or `--allow-delegated` if Ipns.DelegatedPublishers are set up")

const (
	ipfsPathOptionName       = "ipfs-path"
	resolveOptionName        = "resolve"
	allowOfflineOptionName   = "allow-offline"
	allowDelegatedOptionName = "allow-delegated"
	lifeTimeOptionName       = "lifetime"
	ttlOptionName            = "ttl"
	keyOptionName            = "key"
	quieterOptionName        = "quieter"
	v1compatOptionName       = "v1compat"
	sequenceOptionName       = "sequence"
)

var PublishCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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

Publishing Modes:

By default, IPNS records are published to both the DHT and any configured
HTTP delegated publishers. You can control this behavior with the following flags:

  --allow-offline    Allow publishing when offline (publishes to local datastore, network operations are optional)
  --allow-delegated  Allow publishing without DHT connectivity (local + HTTP delegated publishers only)

Examples:

Publish an <ipfs-path> with your default name:

  > ipfs name publish /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Publish without DHT (HTTP delegated publishers only):

  > ipfs name publish --allow-delegated /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Publish when offline (local publish, network optional):

  > ipfs name publish --allow-offline /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  Published to QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n: /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Notes:

The --ttl option specifies the time duration for caching IPNS records.
Lower values like '1m' enable faster updates but increase network load,
while the default of 1 hour reduces traffic but may delay propagation.
Gateway operators may override this with Ipns.MaxCacheTTL configuration.

The --sequence option sets a custom sequence number for the IPNS record.
The sequence number must be monotonically increasing (greater than the
current record's sequence). This is useful for manually coordinating
updates across multiple writers. If not specified, the sequence number
increments automatically.

For faster IPNS updates, consider:
- Using a lower --ttl value (e.g., '1m' for quick updates)
- Enabling PubSub via Ipns.UsePubsub in the config

`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg(ipfsPathOptionName, true, false, "ipfs path of the object to be published.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(keyOptionName, "k", "Name of the key to be used or a valid PeerID, as listed by 'ipfs key list -l'.").WithDefault("self"),
		cmds.BoolOption(resolveOptionName, "Check if the given path can be resolved before publishing.").WithDefault(true),
		cmds.StringOption(lifeTimeOptionName, "t", `Time duration the signed record will be valid for. Accepts durations such as "300s", "1.5h" or "7d2h45m"`).WithDefault(ipns.DefaultRecordLifetime.String()),
		cmds.StringOption(ttlOptionName, "Time duration hint, akin to --lifetime, indicating how long to cache this record before checking for updates.").WithDefault(ipns.DefaultRecordTTL.String()),
		cmds.BoolOption(quieterOptionName, "Q", "Write only final IPNS Name encoded as CIDv1 (for use in /ipns content paths)."),
		cmds.BoolOption(v1compatOptionName, "Produce a backward-compatible IPNS Record by including fields for both V1 and V2 signatures.").WithDefault(true),
		cmds.BoolOption(allowOfflineOptionName, "Allow publishing when offline - publishes to local datastore without requiring network connectivity."),
		cmds.BoolOption(allowDelegatedOptionName, "Allow publishing without DHT connectivity - uses local datastore and HTTP delegated publishers only."),
		cmds.Uint64Option(sequenceOptionName, "Set a custom sequence number for the IPNS record (must be higher than current)."),
		ke.OptionIPNSBase,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		allowOffline, _ := req.Options[allowOfflineOptionName].(bool)
		allowDelegated, _ := req.Options[allowDelegatedOptionName].(bool)
		compatibleWithV1, _ := req.Options[v1compatOptionName].(bool)
		kname, _ := req.Options[keyOptionName].(string)

		// Validate flag combinations
		if allowOffline && allowDelegated {
			return errors.New("cannot use both --allow-offline and --allow-delegated flags")
		}

		validTimeOpt, _ := req.Options[lifeTimeOptionName].(string)
		validTime, err := time.ParseDuration(validTimeOpt)
		if err != nil {
			return fmt.Errorf("error parsing lifetime option: %s", err)
		}

		opts := []options.NamePublishOption{
			options.Name.AllowOffline(allowOffline),
			options.Name.AllowDelegated(allowDelegated),
			options.Name.Key(kname),
			options.Name.ValidTime(validTime),
			options.Name.CompatibleWithV1(compatibleWithV1),
		}

		if ttl, found := req.Options[ttlOptionName].(string); found {
			d, err := time.ParseDuration(ttl)
			if err != nil {
				return err
			}

			opts = append(opts, options.Name.TTL(d))
		}

		if sequence, found := req.Options[sequenceOptionName].(uint64); found {
			opts = append(opts, options.Name.Sequence(sequence))
		}

		p, err := cmdutils.PathOrCidPath(req.Arguments[0])
		if err != nil {
			return err
		}

		if verifyExists, _ := req.Options[resolveOptionName].(bool); verifyExists {
			_, err := api.ResolveNode(req.Context, p)
			if err != nil {
				return err
			}
		}

		name, err := api.Name().Publish(req.Context, p, opts...)
		if err != nil {
			if err == iface.ErrOffline {
				err = errAllowOffline
			}
			return err
		}

		return cmds.EmitOnce(res, &IpnsEntry{
			Name:  name.String(),
			Value: p.String(),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ie *IpnsEntry) error {
			var err error
			quieter, _ := req.Options[quieterOptionName].(bool)
			if quieter {
				_, err = fmt.Fprintln(w, cmdenv.EscNonPrint(ie.Name))
			} else {
				_, err = fmt.Fprintf(w, "Published to %s: %s\n", cmdenv.EscNonPrint(ie.Name), cmdenv.EscNonPrint(ie.Value))
			}
			return err
		}),
	},
	Type: IpnsEntry{},
}
