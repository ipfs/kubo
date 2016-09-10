package commands

import (
	"bytes"
	"encoding/binary"
	"io"

	cmds "github.com/ipfs/go-ipfs/commands"

	floodsub "gx/ipfs/QmQriRMW5cCJyLrzDnXi7fZ5mVbetiEZjPjbqoJhuSL94m/floodsub"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

var PubsubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An experimental publish-subscribe system on ipfs.",
		ShortDescription: `
ipfs pubsub allows you to publish messages to a given topic, and also to
subscribe to new messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"pub": PubsubPubCmd,
		"sub": PubsubSubCmd,
	},
}

var PubsubSubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Subscribe to messages on a given topic.",
		ShortDescription: `
ipfs pubsub sub subscribes to messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "String name of topic to subscribe to."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// Must be online!
		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		topic := req.Arguments()[0]
		msgs, err := n.Floodsub.Subscribe(topic)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		out := make(chan interface{})
		res.SetOutput((<-chan interface{})(out))

		ctx := req.Context()
		go func() {
			defer close(out)
			for {
				select {
				case msg, ok := <-msgs:
					if !ok {
						return
					}
					out <- msg
				case <-ctx.Done():
					n.Floodsub.Unsub(topic)
				}
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: getPsMsgMarshaler(func(m *floodsub.Message) (io.Reader, error) {
			log.Error("FROM: ", m.GetFrom())
			return bytes.NewReader(m.Data), nil
		}),
		"ndpayload": getPsMsgMarshaler(func(m *floodsub.Message) (io.Reader, error) {
			m.Data = append(m.Data, '\n')
			return bytes.NewReader(m.Data), nil
		}),
		"lenpayload": getPsMsgMarshaler(func(m *floodsub.Message) (io.Reader, error) {
			buf := make([]byte, 8)
			n := binary.PutUvarint(buf, uint64(len(m.Data)))
			return io.MultiReader(bytes.NewReader(buf[:n]), bytes.NewReader(m.Data)), nil
		}),
	},
	Type: floodsub.Message{},
}

func getPsMsgMarshaler(f func(m *floodsub.Message) (io.Reader, error)) func(cmds.Response) (io.Reader, error) {
	return func(res cmds.Response) (io.Reader, error) {
		outChan, ok := res.Output().(<-chan interface{})
		if !ok {
			return nil, u.ErrCast()
		}

		marshal := func(v interface{}) (io.Reader, error) {
			obj, ok := v.(*floodsub.Message)
			if !ok {
				return nil, u.ErrCast()
			}

			return f(obj)
		}

		return &cmds.ChannelMarshaler{
			Channel:   outChan,
			Marshaler: marshal,
			Res:       res,
		}, nil
	}
}

var PubsubPubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Publish a message to a given pubsub topic.",
		ShortDescription: `
ipfs pubsub pub publishes a message to a specified topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "Topic to publish to."),
		cmds.StringArg("data", true, true, "Payload of message to publish.").EnableStdin(),
	},
	Options: []cmds.Option{},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// Must be online!
		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		topic := req.Arguments()[0]

		for _, data := range req.Arguments()[1:] {
			if err := n.Floodsub.Publish(topic, []byte(data)); err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}
	},
}
