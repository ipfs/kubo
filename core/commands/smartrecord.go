package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-smart-record/ir"
	"github.com/libp2p/go-smart-record/vm"
)

var ErrNoSmartRecord = errors.New("smart records are not enabled")

const smartRecordReqTimeout = 10 * time.Second

var SmartRecordCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Issue smart record commands.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"get":    getSmartRecordCmd,
		"update": updateSmartRecordCmd,
		// NOTE: Query not available yet
		// "query":  querySmartRecordCmd,
	},
}

const (
	smartRecordVerboseOptionName = "verbose"
)

type SmartRecordResult struct {
	Out []byte
	Ok  bool
}

var getSmartRecordCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Get a smart record from a peer",
		ShortDescription: "Outputs the result of the query",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, false, "PeerID of the peer we want to query"),
		cmds.StringArg("key", true, false, "Key of the record to query from peer"),
	},
	Options: []cmds.Option{
		cmds.BoolOption(smartRecordVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		// NOTE: What about implementing batch get so we can get a list of records
		// from a peer?
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if nd.SmartRecords == nil {
			return ErrNoSmartRecord
		}

		k := req.Arguments[1]
		p, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return cmds.ClientError("invalid peer ID")
		}

		smManager := nd.SmartRecords
		ctx, cancel := context.WithTimeout(req.Context, smartRecordReqTimeout)
		out, err := smManager.Get(ctx, k, p)
		cancel()
		if err != nil {
			return fmt.Errorf("record GET failed: %s", err)
		}

		b, err := vm.MarshalRecordValue(*out)
		if err != nil {
			return fmt.Errorf("Error marshalling record value: %s", err)
		}
		return res.Emit(&SmartRecordResult{Out: b, Ok: true})

	},
	Type: &SmartRecordResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *SmartRecordResult) error {
			rv, err := vm.UnmarshalRecordValue(out.Out)
			if err != nil {
				return err
			}
			if len(rv) == 0 {
				fmt.Println("No entries in record")
			} else {
				// NOTE: We can probably come up with prettier ways of outputting this.
				for k, v := range rv {
					var w bytes.Buffer
					v.WritePretty(&w)
					fmt.Printf("(%s): %s", k.String(), string(w.Bytes()))
					w.Reset()
				}
			}
			return nil
		}),
	},
}

var updateSmartRecordCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Update smart record given as input in a peer",
		ShortDescription: "Outputs the result of the query",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, false, "PeerID where we want to update the record"),
		cmds.StringArg("key", true, false, "Key of the record to update"),
		cmds.StringArg("record", true, false, "Record to be updated in JSON"),
	},
	Options: []cmds.Option{
		cmds.BoolOption(smartRecordVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if nd.SmartRecords == nil {
			return ErrNoSmartRecord
		}

		k := req.Arguments[1]
		// Decode peer.ID
		p, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return cmds.ClientError("invalid peer ID")
		}
		// Input argument
		rin := req.Arguments[2]
		rm, err := ir.Unmarshal([]byte(rin))
		if err != nil {
			return fmt.Errorf("Couldn't unmarshal record: %s", err)
		}
		// Check if it is a dict
		r, ok := rm.(ir.Dict)
		if !ok {
			return fmt.Errorf("Record is not dict type")
		}

		smManager := nd.SmartRecords
		ctx, cancel := context.WithTimeout(req.Context, smartRecordReqTimeout)
		err = smManager.Update(ctx, k, p, r)
		cancel()
		if err != nil {
			return fmt.Errorf("record UPDATE failed: %s", err)
		}

		err = res.Emit(&SmartRecordResult{Ok: true})
		if err != nil {
			return err
		}
		return nil

	},
	Type: &SmartRecordResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *SmartRecordResult) error {
			// NOTE: Consider outputting additional information about the update.
			if out.Ok {
				fmt.Println("Record updated successfully")
			} else {
				fmt.Println("Record update wasn't successful")
			}
			return nil
		}),
	},
}
