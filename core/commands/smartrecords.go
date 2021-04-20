package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-smart-record/ir"
	vm "github.com/libp2p/go-smart-record/vm"
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
		"encodeAdd": encodeAdd,
	},
}

const (
	smartRecordVerboseOptionName = "verbose"
)

type SmartRecordResult struct {
	out vm.RecordValue
	ok  bool
	r   ir.Dict
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
		return res.Emit(&SmartRecordResult{out: *out, ok: true})

	},
	Type: &SmartRecordResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *SmartRecordResult) error {
			fmt.Println("emitted output:", out)
			if out != nil {
				if len(out.out) == 0 {
					fmt.Println("No entries in record")
					return nil
				}
				for k, v := range out.out {
					var w bytes.Buffer
					v.WritePretty(&w)
					fmt.Printf("(%s): %s", k.String(), string(w.Bytes()))
					w.Reset()
				}
			} else {
				fmt.Println("No record received from remote peer")
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
		p, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return cmds.ClientError("invalid peer ID")
		}
		rin := req.Arguments[2]
		rm, err := ir.Unmarshal([]byte(rin))
		if err != nil {
			return fmt.Errorf("Couldn't unmarshal record: %s", err)
		}

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
		fmt.Println(ok, r)
		a := &SmartRecordResult{ok: true, r: r}
		fmt.Println(a)
		err = res.Emit(a)
		if err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
		return nil

	},
	Type: &SmartRecordResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *SmartRecordResult) error {
			fmt.Println(out)
			if out.ok {
				fmt.Println("Record to update:")
				var w bytes.Buffer
				out.r.WritePretty(&w)
				fmt.Println(string(w.Bytes()))
				fmt.Println("Record upated successfully")
			} else {
				fmt.Println("Record update wasn't successful")
			}
			return nil
		}),
	},
}

// AddStatus describes the progress of the add operation
type AddStatus struct {
	// Current is the current value of the sum.
	Current int

	// Left is how many summands are left
	Left int
}

var encodeAdd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.StringArg("summands", true, true, "values that are supposed to be summed"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		sum := 0
		fmt.Println("Starting")
		for i, str := range req.Arguments {
			num, err := strconv.Atoi(str)
			if err != nil {
				return err
			}
			fmt.Println("Emitting")
			sum += num
			err = re.Emit(&AddStatus{
				Current: sum,
				Left:    len(req.Arguments) - i - 1,
			})
			if err != nil {
				return err
			}
			fmt.Println("Post-Emitting")

			time.Sleep(200 * time.Millisecond)
		}
		return nil
	},
	Type: &AddStatus{},
	Encoders: cmds.EncoderMap{
		// This defines how to encode these values as text. Other possible encodings are XML and JSON.
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			s, ok := v.(*AddStatus)
			if !ok {
				return fmt.Errorf("cast error, got type %T", v)
			}

			if s.Left == 0 {
				fmt.Fprintln(w, "total:", s.Current)
			} else {
				fmt.Fprintf(w, "intermediate result: %d; %d left\n", s.Current, s.Left)
			}
			return nil
		}),
	},
}
