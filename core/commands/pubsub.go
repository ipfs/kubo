package commands

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	mbase "github.com/multiformats/go-multibase"
	"github.com/pkg/errors"

	cmds "github.com/ipfs/go-ipfs-cmds"
	options "github.com/ipfs/interface-go-ipfs-core/options"
)

var PubsubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An experimental publish-subscribe system on ipfs.",
		ShortDescription: `
ipfs pubsub allows you to publish messages to a given topic, and also to
subscribe to new messages on a given topic.

EXPERIMENTAL FEATURE

  It is not intended in its current state to be used in a production
  environment.  To use, the daemon must be run with
  '--enable-pubsub-experiment'.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"pub":   PubsubPubCmd,
		"sub":   PubsubSubCmd,
		"ls":    PubsubLsCmd,
		"peers": PubsubPeersCmd,
	},
}

type pubsubMessage struct {
	From     string   `json:"from,omitempty"`
	Data     string   `json:"data,omitempty"`
	Seqno    string   `json:"seqno,omitempty"`
	TopicIDs []string `json:"topicIDs,omitempty"`
}

var PubsubSubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Subscribe to messages on a given topic.",
		ShortDescription: `
ipfs pubsub sub subscribes to messages on a given topic.

EXPERIMENTAL FEATURE

  It is not intended in its current state to be used in a production
  environment.  To use, the daemon must be run with
  '--enable-pubsub-experiment'.

TOPIC ENCODING

  Topic names are a binary data. To ensure all bytes are transferred
  correctly RPC client and server will use multibase encoding behind
  the scenes.

  You can inspect the format by passing --enc=json. ipfs multibase commands
  can be used for encoding/decoding multibase strings in the userland.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "Name of topic to subscribe to."),
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

			// encode as base64url so the same string is present in body and URL args
			// when sent over HTTP RPC API
			encoder, _ := mbase.EncoderByName("base64url")
			psm := pubsubMessage{
				Data:  encoder.Encode(msg.Data()),
				From:  encoder.Encode([]byte(msg.From())),
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
	Helptext: cmds.HelpText{
		Tagline: "Publish a message to a given pubsub topic.",
		ShortDescription: `
ipfs pubsub pub publishes a message to a specified topic.

EXPERIMENTAL FEATURE

  It is not intended in its current state to be used in a production
  environment.  To use, the daemon must be run with
  '--enable-pubsub-experiment'.

TOPIC AND DATA ENCODING

  Topic names are a binary data too. To ensure all bytes are transferred
  correctly RPC client and server will use multibase encoding behind
  the scenes.

  You can inspect the format by passing --enc=json. ipfs multibase commands
  can be used for encoding/decoding multibase strings in the userland.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "Topic to publish to."),
		cmds.StringArg("data", false, true, "Payload of message to publish."),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		// when there are no string args with data, read from stdin.
		if len(req.Arguments) == 1 {
			buf, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			req.Arguments = append(req.Arguments, string(buf))
		}
		return urlArgsEncoder(req, env)
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		topic := req.Arguments[0]
		for _, data := range req.Arguments[1:] {
			if err := api.PubSub().Publish(req.Context, topic, []byte(data)); err != nil {
				return err
			}
		}

		return nil
	},
}

var PubsubLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List subscribed topics by name.",
		ShortDescription: `
ipfs pubsub ls lists out the names of topics you are currently subscribed to.

EXPERIMENTAL FEATURE

  It is not intended in its current state to be used in a production
  environment.  To use, the daemon must be run with
  '--enable-pubsub-experiment'.

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
	return stringListEncoder(req, w, list)
}

func stringListEncoder(req *cmds.Request, w io.Writer, list *stringList) error {
	for _, str := range list.Strings {
		_, err := fmt.Fprintf(w, "%s\n", cmdenv.EscNonPrint(str))
		if err != nil {
			return err
		}
	}
	return nil
}

var PubsubPeersCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List peers we are currently pubsubbing with.",
		ShortDescription: `
ipfs pubsub peers with no arguments lists out the pubsub peers you are
currently connected to. If given a topic, it will list connected peers who are
subscribed to the named topic.

EXPERIMENTAL FEATURE

  It is not intended in its current state to be used in a production
  environment.  To use, the daemon must be run with
  '--enable-pubsub-experiment'.

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
			list.Strings = append(list.Strings, peer.Pretty())
		}
		sort.Strings(list.Strings)
		return cmds.EmitOnce(res, list)
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(stringListEncoder),
	},
}

// Encode binary data to be passed as multibase string in URL arguments.
// (avoiding issues described in https://github.com/ipfs/go-ipfs/issues/7939)
func urlArgsEncoder(req *cmds.Request, env cmds.Environment) error {
	encoder, _ := mbase.EncoderByName("base64url")
	for n, arg := range req.Arguments {
		req.Arguments[n] = encoder.Encode([]byte(arg))
	}
	return nil
}

// Decode binary data passed as multibase string in URL arguments.
// (avoiding issues described in https://github.com/ipfs/go-ipfs/issues/7939)
func urlArgsDecoder(req *cmds.Request, env cmds.Environment) error {
	err := req.ParseBodyArgs()
	if err != nil {
		return err
	}
	for n, arg := range req.Arguments {
		encoding, data, err := mbase.Decode(arg)
		if err != nil {
			return errors.Wrap(err, "URL arg must be multibase encoded")
		}

		// Enforce URL-safe encoding is used for data passed via URL arguments
		// - without this we get data corruption similar to https://github.com/ipfs/go-ipfs/issues/7939
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
