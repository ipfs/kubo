package commands

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cmds "github.com/ipfs/go-ipfs-cmds"
	oldcmds "github.com/ipfs/kubo/commands"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"
)

var DiagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Generate diagnostic reports.",
	},

	Subcommands: map[string]*cmds.Command{
		"sys":       sysDiagCmd,
		"cmds":      ActiveReqsCmd,
		"profile":   sysProfileCmd,
		"datastore": diagDatastoreCmd,
	},
}

var diagDatastoreCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Low-level datastore inspection for debugging and testing.",
		ShortDescription: `
'ipfs diag datastore' provides low-level access to the datastore for debugging
and testing purposes.

WARNING: FOR DEBUGGING/TESTING ONLY

These commands expose internal datastore details and should not be used
in production workflows. The datastore format may change between versions.

The daemon must not be running when calling these commands.

EXAMPLE

Inspecting pubsub seqno validator state:

  $ ipfs diag datastore count /pubsub/seqno/
  2
  $ ipfs diag datastore get --hex /pubsub/seqno/12D3KooW...
  Key: /pubsub/seqno/12D3KooW...
  Hex Dump:
  00000000  18 81 81 c8 91 c0 ea f6  |........|
`,
	},
	Subcommands: map[string]*cmds.Command{
		"get":   diagDatastoreGetCmd,
		"count": diagDatastoreCountCmd,
	},
}

const diagDatastoreHexOptionName = "hex"

type diagDatastoreGetResult struct {
	Key     string `json:"key"`
	Value   []byte `json:"value"`
	HexDump string `json:"hex_dump,omitempty"`
}

var diagDatastoreGetCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Read a raw key from the datastore.",
		ShortDescription: `
Returns the value stored at the given datastore key.
Default output is raw bytes. Use --hex for human-readable hex dump.

The daemon must not be running when using this command.

WARNING: FOR DEBUGGING/TESTING ONLY
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Datastore key to read (e.g., /pubsub/seqno/<peerid>)"),
	},
	Options: []cmds.Option{
		cmds.BoolOption(diagDatastoreHexOptionName, "Output hex dump instead of raw bytes"),
	},
	NoRemote: true,
	PreRun:   DaemonNotRunning,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cctx := env.(*oldcmds.Context)
		repo, err := fsrepo.Open(cctx.ConfigRoot)
		if err != nil {
			return fmt.Errorf("failed to open repo: %w", err)
		}
		defer repo.Close()

		keyStr := req.Arguments[0]
		key := datastore.NewKey(keyStr)
		ds := repo.Datastore()

		val, err := ds.Get(req.Context, key)
		if err != nil {
			if errors.Is(err, datastore.ErrNotFound) {
				return fmt.Errorf("key not found: %s", keyStr)
			}
			return fmt.Errorf("failed to read key: %w", err)
		}

		result := &diagDatastoreGetResult{
			Key:   keyStr,
			Value: val,
		}

		if hexDump, _ := req.Options[diagDatastoreHexOptionName].(bool); hexDump {
			result.HexDump = hex.Dump(val)
		}

		return cmds.EmitOnce(res, result)
	},
	Type: diagDatastoreGetResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result *diagDatastoreGetResult) error {
			if result.HexDump != "" {
				fmt.Fprintf(w, "Key: %s\nHex Dump:\n%s", result.Key, result.HexDump)
				return nil
			}
			// Raw bytes output
			_, err := w.Write(result.Value)
			return err
		}),
	},
}

type diagDatastoreCountResult struct {
	Prefix string `json:"prefix"`
	Count  int64  `json:"count"`
}

var diagDatastoreCountCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Count entries matching a datastore prefix.",
		ShortDescription: `
Counts the number of datastore entries whose keys start with the given prefix.

The daemon must not be running when using this command.

WARNING: FOR DEBUGGING/TESTING ONLY
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("prefix", true, false, "Datastore key prefix (e.g., /pubsub/seqno/)"),
	},
	NoRemote: true,
	PreRun:   DaemonNotRunning,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cctx := env.(*oldcmds.Context)
		repo, err := fsrepo.Open(cctx.ConfigRoot)
		if err != nil {
			return fmt.Errorf("failed to open repo: %w", err)
		}
		defer repo.Close()

		prefix := req.Arguments[0]
		ds := repo.Datastore()

		q := query.Query{
			Prefix:   prefix,
			KeysOnly: true,
		}

		results, err := ds.Query(req.Context, q)
		if err != nil {
			return fmt.Errorf("failed to query datastore: %w", err)
		}
		defer results.Close()

		var count int64
		for result := range results.Next() {
			if result.Error != nil {
				return fmt.Errorf("query error: %w", result.Error)
			}
			count++
		}

		return cmds.EmitOnce(res, &diagDatastoreCountResult{
			Prefix: prefix,
			Count:  count,
		})
	},
	Type: diagDatastoreCountResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result *diagDatastoreCountResult) error {
			_, err := fmt.Fprintf(w, "%d\n", result.Count)
			return err
		}),
	},
}
