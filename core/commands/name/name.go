package name

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/ipfs/boxo/ipns"
	ipns_pb "github.com/ipfs/boxo/ipns/pb"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"google.golang.org/protobuf/proto"
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

type IpnsInspectValidation struct {
	Valid  bool
	Reason string
	Name   string
}

// IpnsInspectEntry contains the deserialized values from an IPNS Entry:
// https://github.com/ipfs/specs/blob/main/ipns/IPNS.md#record-serialization-format
type IpnsInspectEntry struct {
	Value        string
	ValidityType *ipns.ValidityType
	Validity     *time.Time
	Sequence     *uint64
	TTL          *time.Duration
}

type IpnsInspectResult struct {
	Entry         IpnsInspectEntry
	PbSize        int
	SignatureType string
	HexDump       string
	Validation    *IpnsInspectValidation
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
		cmds.BoolOption("dump", "Include a full hex dump of the raw Protobuf record.").WithDefault(true),
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

		rec, err := ipns.UnmarshalRecord(b.Bytes())
		if err != nil {
			return err
		}

		result := &IpnsInspectResult{
			Entry: IpnsInspectEntry{},
		}

		// Best effort to get the fields. Show everything we can.
		if v, err := rec.Value(); err == nil {
			result.Entry.Value = v.String()
		}

		if v, err := rec.ValidityType(); err == nil {
			result.Entry.ValidityType = &v
		}

		if v, err := rec.Validity(); err == nil {
			result.Entry.Validity = &v
		}

		if v, err := rec.Sequence(); err == nil {
			result.Entry.Sequence = &v
		}

		if v, err := rec.TTL(); err == nil {
			result.Entry.TTL = &v
		}

		// Here we need the raw protobuf just to decide the version.
		var pbRecord ipns_pb.IpnsRecord
		err = proto.Unmarshal(b.Bytes(), &pbRecord)
		if err != nil {
			return err
		}
		if len(pbRecord.SignatureV1) != 0 || len(pbRecord.Value) != 0 {
			result.SignatureType = "V1+V2"
		} else if pbRecord.Data != nil {
			result.SignatureType = "V2"
		} else {
			result.SignatureType = "Unknown"
		}
		result.PbSize = proto.Size(&pbRecord)

		if verify, ok := req.Options["verify"].(string); ok {
			name, err := ipns.NameFromString(verify)
			if err != nil {
				return err
			}

			result.Validation = &IpnsInspectValidation{
				Name: name.String(),
			}

			err = ipns.ValidateWithName(rec, name)
			if err == nil {
				result.Validation.Valid = true
			} else {
				result.Validation.Reason = err.Error()
			}
		}

		if dump, ok := req.Options["dump"].(bool); ok && dump {
			result.HexDump = hex.Dump(b.Bytes())
		}

		return cmds.EmitOnce(res, result)
	},
	Type: IpnsInspectResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *IpnsInspectResult) error {
			tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
			defer tw.Flush()

			if out.Entry.Value != "" {
				fmt.Fprintf(tw, "Value:\t%q\n", out.Entry.Value)
			}

			if out.Entry.ValidityType != nil {
				fmt.Fprintf(tw, "Validity Type:\t%q\n", *out.Entry.ValidityType)
			}

			if out.Entry.Validity != nil {
				fmt.Fprintf(tw, "Validity:\t%q\n", out.Entry.Validity.Format(time.RFC3339Nano))
			}

			if out.Entry.Sequence != nil {
				fmt.Fprintf(tw, "Sequence:\t%d\n", *out.Entry.Sequence)
			}

			if out.Entry.TTL != nil {
				fmt.Fprintf(tw, "TTL:\t%s\n", out.Entry.TTL.String())
			}

			fmt.Fprintf(tw, "Protobuf Size:\t%d\n", out.PbSize)
			fmt.Fprintf(tw, "Signature Type:\t%s\n", out.SignatureType)

			if out.Validation == nil {
				tw.Flush()
				fmt.Fprintf(w, "\nThis record was not verified. Pass '--verify k51...' to verify.\n")
			} else {
				tw.Flush()
				fmt.Fprintf(w, "\nValidation results:\n")

				fmt.Fprintf(tw, "\tValid:\t%v\n", out.Validation.Valid)
				if out.Validation.Reason != "" {
					fmt.Fprintf(tw, "\tReason:\t%s\n", out.Validation.Reason)
				}
				fmt.Fprintf(tw, "\tName:\t%s\n", out.Validation.Name)
			}

			if out.HexDump != "" {
				tw.Flush()

				fmt.Fprintf(w, "\nHex Dump:\n%s", out.HexDump)
			}

			return nil
		}),
	},
}
