package commands

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ipfs/boxo/path"
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/mount"
	"github.com/ipfs/go-datastore/query"
	cmds "github.com/ipfs/go-ipfs-cmds"
	oldcmds "github.com/ipfs/kubo/commands"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	node "github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/core/shutdown"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"
)

// diagHealthyProbeCIDStr is the well-known empty UnixFS directory,
// built into every kubo node. Fetching it succeeds regardless of peers,
// DHT, or user content, so it isolates the DAG/blockstore pipeline.
const diagHealthyProbeCIDStr = "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"

var DiagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Generate diagnostic reports.",
	},

	Subcommands: map[string]*cmds.Command{
		"sys":       sysDiagCmd,
		"cmds":      ActiveReqsCmd,
		"profile":   sysProfileCmd,
		"datastore": diagDatastoreCmd,
		"healthy":   diagHealthyCmd,
	},
}

// diagHealthyCmd is a container-healthcheck probe. It fails when shutdown
// has been initiated (even if the RPC API still answers) or when the DAG
// pipeline cannot resolve a built-in CID.
var diagHealthyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Report whether the daemon is operational.",
		ShortDescription: `
Exits 0 if the daemon is running and can resolve the well-known empty
UnixFS directory. Exits non-zero if shutdown has started or the DAG
pipeline is broken. Intended for container healthchecks.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		if t := shutdown.StartedAt(); !t.IsZero() {
			return fmt.Errorf("daemon is shutting down (started %s ago)", time.Since(t).Round(time.Second))
		}
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		probeCID, err := cid.Decode(diagHealthyProbeCIDStr)
		if err != nil {
			return fmt.Errorf("invalid probe CID: %w", err)
		}
		if _, _, err := api.ResolvePath(req.Context, path.FromCid(probeCID)); err != nil {
			return fmt.Errorf("probe resolve: %w", err)
		}
		if _, err := api.Dag().Get(req.Context, probeCID); err != nil {
			return fmt.Errorf("probe fetch: %w", err)
		}
		return cmds.EmitOnce(res, "ok")
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

When the provider keystore datastores exist on disk (nodes with
Provide.DHT.SweepEnabled=true), they are automatically mounted into the
datastore view under /provider/keystore/0/ and /provider/keystore/1/.

EXAMPLES

Inspecting pubsub seqno validator state:

  $ ipfs diag datastore count /pubsub/seqno/
  2
  $ ipfs diag datastore get --hex /pubsub/seqno/12D3KooW...
  Key: /pubsub/seqno/12D3KooW...
  Hex Dump:
  00000000  18 81 81 c8 91 c0 ea f6  |........|

Writing a test key (debugging only):

  $ ipfs diag datastore put /test/mykey "hello"

Inspecting provider keystore (requires SweepEnabled):

  $ ipfs diag datastore count /provider/keystore/0/
  $ ipfs diag datastore count /provider/keystore/1/
`,
	},
	Subcommands: map[string]*cmds.Command{
		"get":   diagDatastoreGetCmd,
		"put":   diagDatastorePutCmd,
		"count": diagDatastoreCountCmd,
	},
}

const diagDatastoreHexOptionName = "hex"

type diagDatastoreGetResult struct {
	Key     string `json:"key"`
	Value   []byte `json:"value"`
	HexDump string `json:"hex_dump,omitempty"`
}

// openDiagDatastore opens the repo datastore and conditionally mounts any
// provider keystore datastores that exist on disk. It returns the composite
// datastore and a cleanup function that must be called when done.
func openDiagDatastore(env cmds.Environment) (datastore.Datastore, func(), error) {
	cctx := env.(*oldcmds.Context)
	repo, err := fsrepo.Open(cctx.ConfigRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open repo: %w", err)
	}

	extraMounts, extraCloser, err := node.MountKeystoreDatastores(repo)
	if err != nil {
		repo.Close()
		return nil, nil, err
	}

	closer := func() {
		extraCloser()
		repo.Close()
	}

	if len(extraMounts) == 0 {
		return repo.Datastore(), closer, nil
	}

	mounts := []mount.Mount{{Prefix: datastore.NewKey("/"), Datastore: repo.Datastore()}}
	mounts = append(mounts, extraMounts...)
	return mount.New(mounts), closer, nil
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
		ds, closer, err := openDiagDatastore(env)
		if err != nil {
			return err
		}
		defer closer()

		keyStr := req.Arguments[0]
		key := datastore.NewKey(keyStr)

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

var diagDatastorePutCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Write a raw key-value pair to the datastore.",
		ShortDescription: `
Stores the given value at the specified datastore key.

The daemon must not be running when using this command.

WARNING: FOR DEBUGGING/TESTING ONLY
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Datastore key (e.g., /test/mykey)"),
		cmds.StringArg("value", true, false, "Value to store (as a string)"),
	},
	NoRemote: true,
	PreRun:   DaemonNotRunning,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ds, closer, err := openDiagDatastore(env)
		if err != nil {
			return err
		}
		defer closer()

		key := datastore.NewKey(req.Arguments[0])
		if err := ds.Put(req.Context, key, []byte(req.Arguments[1])); err != nil {
			return fmt.Errorf("failed to put key: %w", err)
		}
		if err := ds.Sync(req.Context, key); err != nil {
			return fmt.Errorf("failed to sync: %w", err)
		}
		return nil
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
		ds, closer, err := openDiagDatastore(env)
		if err != nil {
			return err
		}
		defer closer()

		prefix := req.Arguments[0]

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
