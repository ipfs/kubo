package name

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ipfs/boxo/ipns"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
)

type IpnsEntry struct {
	Name  string
	Value string
}

var NameCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Publish and resolve IPNS names.",
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

	Subcommands: map[string]*cmds.Command{
		"publish": PublishCmd,
		"resolve": IpnsCmd,
		"pubsub":  IpnsPubsubCmd,
		"inspect": IpnsInspectCmd,
	},
}

type IpnsInspectResult struct {
	Entry      *ipns.IpnsInspectEntry
	Validation *ipns.IpnsInspectValidation
}

var IpnsInspectCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Inspects an IPNS Record",
		ShortDescription: `
Prints values inside of IPNS Record protobuf and its DAG-CBOR Data field.
Passing --verify will verify signature against provided public key.
`,
		LongDescription: `
Prints values inside of IPNS Record protobuf and its DAG-CBOR Data field.

The input can be a file or STDIN, the output can be JSON:

  $ ipfs routing get "/ipns/$PEERID" > ipns_record
  $ ipfs name inspect --enc=json < ipns_record

Values in PublicKey, SignatureV1 and SignatureV2 fields are raw bytes encoded
in Multibase. The Data field is DAG-CBOR represented as DAG-JSON.

Passing --verify will verify signature against provided public key.

`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("record", true, false, "The IPNS record payload to be verified.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("verify", "CID of the public IPNS key to validate against."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		var b bytes.Buffer

		_, err = io.Copy(&b, file)
		if err != nil {
			return err
		}

		entry, err := ipns.UnmarshalIpnsEntry(b.Bytes())
		if err != nil {
			return err
		}

		inspectEntry, err := ipns.InspectIpnsRecord(entry)
		if err != nil {
			return err
		}

		result := &IpnsInspectResult{
			Entry: inspectEntry,
		}

		verify, ok := req.Options["verify"].(string)
		if ok {
			key := strings.TrimPrefix(verify, "/ipns/")
			validation, err := ipns.Verify(key, entry)
			if err != nil {
				return err
			}
			result.Validation = validation
		}

		return cmds.EmitOnce(res, result)
	},
	Type: IpnsInspectResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *IpnsInspectResult) error {
			tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
			defer tw.Flush()

			fmt.Fprintf(tw, "Value:\t%q\n", string(out.Entry.Value))
			fmt.Fprintf(tw, "Validity Type:\t%q\n", out.Entry.ValidityType)
			if out.Entry.Validity != nil {
				fmt.Fprintf(tw, "Validity:\t%s\n", out.Entry.Validity.Format(time.RFC3339Nano))
			}
			fmt.Fprintf(tw, "Sequence:\t%d\n", out.Entry.Sequence)
			if out.Entry.TTL != nil {
				fmt.Fprintf(tw, "TTL:\t%d\n", *out.Entry.TTL)
			}
			fmt.Fprintf(tw, "PublicKey:\t%q\n", out.Entry.PublicKey)
			fmt.Fprintf(tw, "Signature V1:\t%q\n", out.Entry.SignatureV1)
			fmt.Fprintf(tw, "Signature V2:\t%q\n", out.Entry.SignatureV2)

			data, err := json.Marshal(out.Entry.Data)
			if err != nil {
				return err
			}
			fmt.Fprintf(tw, "Data:\t%s\n", string(data))

			if out.Validation == nil {
				tw.Flush()
				fmt.Fprintf(w, "\nThis record was not validated.\n")
			} else {
				tw.Flush()
				fmt.Fprintf(w, "\nValidation results:\n")

				fmt.Fprintf(tw, "\tValid:\t%v\n", out.Validation.Valid)
				if out.Validation.Reason != "" {
					fmt.Fprintf(tw, "\tReason:\t%s\n", out.Validation.Reason)
				}
				fmt.Fprintf(tw, "\tPublicKey:\t%s\n", out.Validation.PublicKey)
			}

			return nil
		}),
	},
}
