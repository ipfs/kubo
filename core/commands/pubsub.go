package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	options "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	mbase "github.com/multiformats/go-multibase"
)

var PubsubCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "An experimental publish-subscribe system on ipfs.",
		ShortDescription: `
ipfs pubsub allows you to publish messages to a given topic, and also to
subscribe to new messages on a given topic.

EXPERIMENTAL FEATURE

  This is an opt-in feature optimized for IPNS over PubSub
  (https://specs.ipfs.tech/ipns/ipns-pubsub-router/).

  The default message validator is designed for IPNS record protocol.
  For custom pubsub applications requiring different validation logic,
  use go-libp2p-pubsub (https://github.com/libp2p/go-libp2p-pubsub)
  directly in a dedicated binary.

  To enable, set 'Pubsub.Enabled' config to true.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"pub":   PubsubPubCmd,
		"sub":   PubsubSubCmd,
		"ls":    PubsubLsCmd,
		"peers": PubsubPeersCmd,
		"reset": PubsubResetCmd,
	},
}

type pubsubMessage struct {
	From     string   `json:"from,omitempty"`
	Data     string   `json:"data,omitempty"`
	Seqno    string   `json:"seqno,omitempty"`
	TopicIDs []string `json:"topicIDs,omitempty"`
}

var PubsubSubCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Subscribe to messages on a given topic.",
		ShortDescription: `
ipfs pubsub sub subscribes to messages on a given topic.

EXPERIMENTAL FEATURE

  This is an opt-in feature optimized for IPNS over PubSub
  (https://specs.ipfs.tech/ipns/ipns-pubsub-router/).

  To enable, set 'Pubsub.Enabled' config to true.

PEER ENCODING

  Peer IDs in From fields are encoded using the default text representation
  from go-libp2p. This ensures the same string values as in 'ipfs pubsub peers'.

TOPIC AND DATA ENCODING

  Topics, Data and Seqno are binary data. To ensure all bytes are transferred
  correctly the RPC client and server will use multibase encoding behind
  the scenes.

  You can inspect the format by passing --enc=json. The ipfs multibase commands
  can be used for encoding/decoding multibase strings in the userland.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "Name of topic to subscribe to (multibase encoded when sent over HTTP RPC)."),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		topic := req.Arguments[0]

		sub, err := api.PubSub().Subscribe(req.Context, topic)
		if err != nil {
			return err
		}
		defer sub.Close()

		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		}

		for {
			msg, err := sub.Next(req.Context)
			if err == io.EOF || err == context.Canceled {
				return nil
			} else if err != nil {
				return err
			}

			// turn bytes into strings
			encoder, _ := mbase.EncoderByName("base64url")
			psm := pubsubMessage{
				Data:  encoder.Encode(msg.Data()),
				From:  msg.From().String(),
				Seqno: encoder.Encode(msg.Seq()),
			}
			for _, topic := range msg.Topics() {
				psm.TopicIDs = append(psm.TopicIDs, encoder.Encode([]byte(topic)))
			}
			if err := res.Emit(&psm); err != nil {
				return err
			}
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, psm *pubsubMessage) error {
			_, dec, err := mbase.Decode(psm.Data)
			if err != nil {
				return err
			}
			_, err = w.Write(dec)
			return err
		}),
		// DEPRECATED, undocumented format we used in tests, but not anymore
		// <message.payload>\n<message.payload>\n
		"ndpayload": cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, psm *pubsubMessage) error {
			return errors.New("--enc=ndpayload was removed, use --enc=json instead")
		}),
		// DEPRECATED, uncodumented format we used in tests, but not anymore
		// <varint-len><message.payload><varint-len><message.payload>
		"lenpayload": cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, psm *pubsubMessage) error {
			return errors.New("--enc=lenpayload was removed, use --enc=json instead")
		}),
	},
	Type: pubsubMessage{},
}

var PubsubPubCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Publish data to a given pubsub topic.",
		ShortDescription: `
ipfs pubsub pub publishes a message to a specified topic.
It reads binary data from stdin or a file.

EXPERIMENTAL FEATURE

  This is an opt-in feature optimized for IPNS over PubSub
  (https://specs.ipfs.tech/ipns/ipns-pubsub-router/).

  To enable, set 'Pubsub.Enabled' config to true.

HTTP RPC ENCODING

  The data to be published is sent in HTTP request body as multipart/form-data.

  Topic names are binary data too. To ensure all bytes are transferred
  correctly via URL params, the RPC client and server will use multibase
  encoding behind the scenes.

`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "Topic to publish to (multibase encoded when sent over HTTP RPC)."),
		cmds.FileArg("data", true, false, "The data to be published.").EnableStdin(),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		topic := req.Arguments[0]

		// read data passed as a file
		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		// publish
		return api.PubSub().Publish(req.Context, topic, data)
	},
}

var PubsubLsCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "List subscribed topics by name.",
		ShortDescription: `
ipfs pubsub ls lists out the names of topics you are currently subscribed to.

EXPERIMENTAL FEATURE

  This is an opt-in feature optimized for IPNS over PubSub
  (https://specs.ipfs.tech/ipns/ipns-pubsub-router/).

  To enable, set 'Pubsub.Enabled' config to true.

TOPIC ENCODING

  Topic names are a binary data. To ensure all bytes are transferred
  correctly RPC client and server will use multibase encoding behind
  the scenes.

  You can inspect the format by passing --enc=json. ipfs multibase commands
  can be used for encoding/decoding multibase strings in the userland.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		l, err := api.PubSub().Ls(req.Context)
		if err != nil {
			return err
		}

		// emit topics encoded in multibase
		encoder, _ := mbase.EncoderByName("base64url")
		for n, topic := range l {
			l[n] = encoder.Encode([]byte(topic))
		}

		return cmds.EmitOnce(res, stringList{l})
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(multibaseDecodedStringListEncoder),
	},
}

func multibaseDecodedStringListEncoder(req *cmds.Request, w io.Writer, list *stringList) error {
	for n, mb := range list.Strings {
		_, data, err := mbase.Decode(mb)
		if err != nil {
			return err
		}
		list.Strings[n] = string(data)
	}
	return safeTextListEncoder(req, w, list)
}

// converts list of strings to text representation where each string is placed
// in separate line with non-printable/unsafe characters escaped
// (this protects terminal output from being mangled by non-ascii topic names)
func safeTextListEncoder(req *cmds.Request, w io.Writer, list *stringList) error {
	for _, str := range list.Strings {
		_, err := fmt.Fprintf(w, "%s\n", cmdenv.EscNonPrint(str))
		if err != nil {
			return err
		}
	}
	return nil
}

var PubsubPeersCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "List peers we are currently pubsubbing with.",
		ShortDescription: `
ipfs pubsub peers with no arguments lists out the pubsub peers you are
currently connected to. If given a topic, it will list connected peers who are
subscribed to the named topic.

EXPERIMENTAL FEATURE

  This is an opt-in feature optimized for IPNS over PubSub
  (https://specs.ipfs.tech/ipns/ipns-pubsub-router/).

  To enable, set 'Pubsub.Enabled' config to true.

TOPIC AND DATA ENCODING

  Topic names are a binary data. To ensure all bytes are transferred
  correctly RPC client and server will use multibase encoding behind
  the scenes.

  You can inspect the format by passing --enc=json. ipfs multibase commands
  can be used for encoding/decoding multibase strings in the userland.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", false, false, "Topic to list connected peers of."),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		var topic string
		if len(req.Arguments) == 1 {
			topic = req.Arguments[0]
		}

		peers, err := api.PubSub().Peers(req.Context, options.PubSub.Topic(topic))
		if err != nil {
			return err
		}

		list := &stringList{make([]string, 0, len(peers))}

		for _, peer := range peers {
			list.Strings = append(list.Strings, peer.String())
		}
		slices.Sort(list.Strings)
		return cmds.EmitOnce(res, list)
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(safeTextListEncoder),
	},
}

// TODO: move to cmdenv?
// Encode binary data to be passed as multibase string in URL arguments.
// (avoiding issues described in https://github.com/ipfs/kubo/issues/7939)
func urlArgsEncoder(req *cmds.Request, env cmds.Environment) error {
	encoder, _ := mbase.EncoderByName("base64url")
	for n, arg := range req.Arguments {
		req.Arguments[n] = encoder.Encode([]byte(arg))
	}
	return nil
}

// Decode binary data passed as multibase string in URL arguments.
// (avoiding issues described in https://github.com/ipfs/kubo/issues/7939)
func urlArgsDecoder(req *cmds.Request, env cmds.Environment) error {
	for n, arg := range req.Arguments {
		encoding, data, err := mbase.Decode(arg)
		if err != nil {
			return fmt.Errorf("URL arg must be multibase encoded: %w", err)
		}

		// Enforce URL-safe encoding is used for data passed via URL arguments
		// - without this we get data corruption similar to https://github.com/ipfs/kubo/issues/7939
		// - we can't just deny base64, because there may be other bases that
		//   are not URL-safe – better to force base64url which is known to be
		//   safe in URL context
		if encoding != mbase.Base64url {
			return errors.New("URL arg must be base64url encoded")
		}

		req.Arguments[n] = string(data)
	}
	return nil
}

type pubsubResetResult struct {
	Deleted int64 `json:"deleted"`
}

var PubsubResetCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Reset pubsub validator state.",
		ShortDescription: `
Clears persistent sequence number state used by the pubsub validator.

WARNING: FOR TESTING ONLY - DO NOT USE IN PRODUCTION

Resets validator state that protects against replay attacks. After reset,
previously seen messages may be accepted again until their sequence numbers
are re-learned.

Use cases:
- Testing pubsub functionality
- Recovery from a peer sending artificially high sequence numbers
  (which would cause subsequent messages from that peer to be rejected)

The --peer flag limits the reset to a specific peer's state.
Without --peer, all validator state is cleared.

NOTE: This only resets the persistent seqno validator state. The in-memory
seen messages cache (Pubsub.SeenMessagesTTL) auto-expires and can only be
fully cleared by restarting the daemon.
`,
	},
	Options: []cmds.Option{
		cmds.StringOption(peerOptionName, "p", "Only reset state for this peer ID"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		ds := n.Repo.Datastore()
		ctx := req.Context

		peerOpt, _ := req.Options[peerOptionName].(string)

		var deleted int64
		if peerOpt != "" {
			// Reset specific peer
			pid, err := peer.Decode(peerOpt)
			if err != nil {
				return fmt.Errorf("invalid peer ID: %w", err)
			}
			key := datastore.NewKey(libp2p.SeqnoStorePrefix + pid.String())
			exists, err := ds.Has(ctx, key)
			if err != nil {
				return fmt.Errorf("failed to check seqno state: %w", err)
			}
			if exists {
				if err := ds.Delete(ctx, key); err != nil {
					return fmt.Errorf("failed to delete seqno state: %w", err)
				}
				deleted = 1
			}
		} else {
			// Reset all peers using batched delete for efficiency
			q := query.Query{
				Prefix:   libp2p.SeqnoStorePrefix,
				KeysOnly: true,
			}
			results, err := ds.Query(ctx, q)
			if err != nil {
				return fmt.Errorf("failed to query seqno state: %w", err)
			}
			defer results.Close()

			batch, err := ds.Batch(ctx)
			if err != nil {
				return fmt.Errorf("failed to create batch: %w", err)
			}

			for result := range results.Next() {
				if result.Error != nil {
					return fmt.Errorf("query error: %w", result.Error)
				}
				if err := batch.Delete(ctx, datastore.NewKey(result.Key)); err != nil {
					return fmt.Errorf("failed to batch delete key %s: %w", result.Key, err)
				}
				deleted++
			}

			if err := batch.Commit(ctx); err != nil {
				return fmt.Errorf("failed to commit batch delete: %w", err)
			}
		}

		// Sync to ensure deletions are persisted
		if err := ds.Sync(ctx, datastore.NewKey(libp2p.SeqnoStorePrefix)); err != nil {
			return fmt.Errorf("failed to sync datastore: %w", err)
		}

		return cmds.EmitOnce(res, &pubsubResetResult{Deleted: deleted})
	},
	Type: pubsubResetResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result *pubsubResetResult) error {
			peerOpt, _ := req.Options[peerOptionName].(string)
			if peerOpt != "" {
				if result.Deleted == 0 {
					_, err := fmt.Fprintf(w, "No validator state found for peer %s\n", peerOpt)
					return err
				}
				_, err := fmt.Fprintf(w, "Reset validator state for peer %s\n", peerOpt)
				return err
			}
			_, err := fmt.Fprintf(w, "Reset validator state for %d peer(s)\n", result.Deleted)
			return err
		}),
	},
}
