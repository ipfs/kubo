package name

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ipfs/boxo/ipns"
	ipns_pb "github.com/ipfs/boxo/ipns/pb"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/coreiface/options"
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

  > ipfs name publish /ipfs/bafkreifjjcie6lypi6ny7amxnfftagclbuxndqonfipmb64f2km2devei4
  Published to k51qzi5uqu5dgklc20hksmmzhoy5lfrn5xcnryq6xp4r50b5yc0vnivpywfu9p: /ipfs/bafk...

Publish an <ipfs-path> with another name, added by an 'ipfs key' command:

  > ipfs key gen --type=ed25519 mykey
  k51qzi5uqu5dlz49qkb657myg6f1buu6rauv8c6b489a9i1e4dkt7a3yo9j2wr
  > ipfs name publish --key=mykey /ipfs/bafkreifjjcie6lypi6ny7amxnfftagclbuxndqonfipmb64f2km2devei4
  Published to k51qzi5uqu5dlz49qkb657myg6f1buu6rauv8c6b489a9i1e4dkt7a3yo9j2wr: /ipfs/bafk...

Resolve the value of your name:

  > ipfs name resolve
  /ipfs/bafk...

Resolve the value of another name:

  > ipfs name resolve k51qzi5uqu5dlz49qkb657myg6f1buu6rauv8c6b489a9i1e4dkt7a3yo9j2wr
  /ipfs/bafk...

Resolve the value of a dnslink:

  > ipfs name resolve specs.ipfs.tech
  /ipfs/bafy...

`,
	},

	Subcommands: map[string]*cmds.Command{
		"publish": PublishCmd,
		"resolve": IpnsCmd,
		"pubsub":  IpnsPubsubCmd,
		"inspect": IpnsInspectCmd,
		"get":     IpnsGetCmd,
		"put":     IpnsPutCmd,
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
		HTTP: &cmds.HTTPHelpText{
			Description: "Request body should be `multipart/form-data` with the IPNS record bytes.",
		},
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
				fmt.Fprintf(tw, "Validity Type:\t%d\n", *out.Entry.ValidityType)
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

var IpnsGetCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Retrieve a signed IPNS record.",
		ShortDescription: `
Retrieves the signed IPNS record for a given name from the routing system.

The output is the raw IPNS record (protobuf) as defined in the IPNS spec:
https://specs.ipfs.tech/ipns/ipns-record/

The record can be inspected with 'ipfs name inspect':

    ipfs name get <name> | ipfs name inspect

This is equivalent to 'ipfs routing get /ipns/<name>' but only accepts
IPNS names (not arbitrary routing keys).

Note: The routing system returns the "best" IPNS record it knows about.
For IPNS, "best" means the record with the highest sequence number.
If multiple records exist (e.g., after using 'ipfs name put'), this command
returns the one the routing system considers most current.
`,
		HTTP: &cmds.HTTPHelpText{
			ResponseContentType: "application/vnd.ipfs.ipns-record",
		},
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "The IPNS name to look up."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// Normalize the argument: accept both "k51..." and "/ipns/k51..."
		name := req.Arguments[0]
		if !strings.HasPrefix(name, "/ipns/") {
			name = "/ipns/" + name
		}

		data, err := api.Routing().Get(req.Context, name)
		if err != nil {
			return err
		}

		res.SetEncodingType(cmds.OctetStream)
		res.SetContentType("application/vnd.ipfs.ipns-record")
		return res.Emit(bytes.NewReader(data))
	},
}

const (
	forceOptionName       = "force"
	putAllowOfflineOption = "allow-offline"
	allowDelegatedOption  = "allow-delegated"
	putQuietOptionName    = "quiet"
	maxIPNSRecordSize     = 10 << 10 // 10 KiB per IPNS spec
)

var errPutAllowOffline = errors.New("can't put while offline: pass `--allow-offline` to store locally or `--allow-delegated` if Ipns.DelegatedPublishers are set up")

var IpnsPutCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Store a pre-signed IPNS record in the routing system.",
		ShortDescription: `
Stores a pre-signed IPNS record in the routing system.

This command accepts a raw IPNS record (protobuf) as defined in the IPNS spec:
https://specs.ipfs.tech/ipns/ipns-record/

The record must be signed by the private key corresponding to the IPNS name.
Use 'ipfs name get' to retrieve records and 'ipfs name inspect' to examine.
`,
		LongDescription: `
Stores a pre-signed IPNS record in the routing system.

This command accepts a raw IPNS record (protobuf) as defined in the IPNS spec:
https://specs.ipfs.tech/ipns/ipns-record/

The record must be signed by the private key corresponding to the IPNS name.
Use 'ipfs name get' to retrieve records and 'ipfs name inspect' to examine.

Use Cases:

  - Re-publishing third-party records: store someone else's signed record
  - Cross-node sync: import records exported from another node
  - Backup/restore: export with 'name get', restore with 'name put'

Validation:

By default, the command validates that:

  - The record is a valid IPNS record (protobuf)
  - The record size is within 10 KiB limit
  - The signature matches the provided IPNS name
  - The record's sequence number is higher than any existing record
    (identical records are allowed for republishing)

The --force flag skips this command's validation and passes the record
directly to the routing system. Note that --force only affects this command;
it does not control how the routing system handles the record. The routing
system may still reject invalid records or prefer records with higher sequence
numbers. Use --force primarily for testing (e.g., to observe how the routing
system reacts to incorrectly signed or malformed records).

Important: Even after a successful 'name put', a subsequent 'name get' may
return a different record if one with a higher sequence number exists.
This is expected IPNS behavior, not a bug.

Publishing Modes:

By default, IPNS records are published to both the DHT and any configured
HTTP delegated publishers. You can control this behavior with:

  --allow-offline    Store locally without requiring network connectivity
  --allow-delegated  Publish via HTTP delegated publishers only (no DHT)

Examples:

Export and re-import a record:

  > ipfs name get k51... > record.bin
  > ipfs name put k51... record.bin

Store a record received from someone else:

  > ipfs name put k51... third-party-record.bin

Force store a record to test routing validation:

  > ipfs name put --force k51... possibly-invalid-record.bin
`,
		HTTP: &cmds.HTTPHelpText{
			Description: "Request body should be `multipart/form-data` with the IPNS record bytes.",
		},
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "The IPNS name to store the record for (e.g., k51... or /ipns/k51...)."),
		cmds.FileArg("record", true, false, "Path to file containing the signed IPNS record.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(forceOptionName, "f", "Skip validation (signature, sequence, size)."),
		cmds.BoolOption(putAllowOfflineOption, "Store locally without broadcasting to the network."),
		cmds.BoolOption(allowDelegatedOption, "Publish via HTTP delegated publishers only (no DHT)."),
		cmds.BoolOption(putQuietOptionName, "q", "Write no output."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		// Parse options
		force, _ := req.Options[forceOptionName].(bool)
		allowOffline, _ := req.Options[putAllowOfflineOption].(bool)
		allowDelegated, _ := req.Options[allowDelegatedOption].(bool)

		// Validate flag combinations
		if allowOffline && allowDelegated {
			return errors.New("cannot use both --allow-offline and --allow-delegated flags")
		}

		// Handle different publishing modes
		if allowDelegated {
			// AllowDelegated mode: check if delegated publishers are configured
			cfg, err := nd.Repo.Config()
			if err != nil {
				return fmt.Errorf("failed to read config: %w", err)
			}
			delegatedPublishers := cfg.DelegatedPublishersWithAutoConf()
			if len(delegatedPublishers) == 0 {
				return errors.New("no delegated publishers configured: add Ipns.DelegatedPublishers or use --allow-offline for local-only publishing")
			}
			// For allow-delegated mode, we proceed even if offline
			// since we're using HTTP publishing via delegated publishers
		}

		// Parse the IPNS name argument
		nameArg := req.Arguments[0]
		if !strings.HasPrefix(nameArg, "/ipns/") {
			nameArg = "/ipns/" + nameArg
		}
		// Extract the name part after /ipns/
		namePart := strings.TrimPrefix(nameArg, "/ipns/")
		name, err := ipns.NameFromString(namePart)
		if err != nil {
			return fmt.Errorf("invalid IPNS name: %w", err)
		}

		// Read raw record bytes from file/stdin
		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		// Read record data (limit to 1 MiB for memory safety)
		data, err := io.ReadAll(io.LimitReader(file, 1<<20))
		if err != nil {
			return fmt.Errorf("failed to read record: %w", err)
		}
		if len(data) == 0 {
			return errors.New("record is empty")
		}

		// Validate unless --force
		if !force {
			// Check size limit per IPNS spec
			if len(data) > maxIPNSRecordSize {
				return fmt.Errorf("record exceeds maximum size of %d bytes, use --force to skip size check", maxIPNSRecordSize)
			}
			rec, err := ipns.UnmarshalRecord(data)
			if err != nil {
				return fmt.Errorf("invalid IPNS record: %w", err)
			}

			// Validate signature against provided name
			err = ipns.ValidateWithName(rec, name)
			if err != nil {
				return fmt.Errorf("record validation failed: %w", err)
			}

			// Check for sequence conflicts with existing record
			existingData, err := api.Routing().Get(req.Context, nameArg)
			if err == nil {
				// Allow republishing the exact same record (common use case:
				// get a third-party record and put it back to refresh DHT)
				if !bytes.Equal(existingData, data) {
					existingRec, parseErr := ipns.UnmarshalRecord(existingData)
					if parseErr == nil {
						existingSeq, seqErr := existingRec.Sequence()
						newSeq, newSeqErr := rec.Sequence()
						if seqErr == nil && newSeqErr == nil && existingSeq >= newSeq {
							return fmt.Errorf("existing IPNS record has sequence %d >= new record sequence %d, use 'ipfs name put --force' to skip this check", existingSeq, newSeq)
						}
					}
				}
			}
			// If Get fails (no existing record), that's fine - proceed with put
		}

		// Publish the original bytes as-is
		// When allowDelegated is true, we set allowOffline to allow the operation
		// even without DHT connectivity (delegated publishers use HTTP)
		opts := []options.RoutingPutOption{
			options.Routing.AllowOffline(allowOffline || allowDelegated),
		}

		err = api.Routing().Put(req.Context, nameArg, data, opts...)
		if err != nil {
			if err.Error() == "can't put while offline" {
				return errPutAllowOffline
			}
			return err
		}

		// Extract value from the record for the response
		value := ""
		if rec, err := ipns.UnmarshalRecord(data); err == nil {
			if v, err := rec.Value(); err == nil {
				value = v.String()
			}
		}

		return cmds.EmitOnce(res, &IpnsEntry{
			Name:  name.String(),
			Value: value,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ie *IpnsEntry) error {
			quiet, _ := req.Options[putQuietOptionName].(bool)
			if quiet {
				return nil
			}
			_, err := fmt.Fprintln(w, cmdenv.EscNonPrint(ie.Name))
			return err
		}),
	},
	Type: IpnsEntry{},
}
