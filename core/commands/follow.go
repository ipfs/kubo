package commands

import (
	"errors"
	"io"
	"strings"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	nc "github.com/ipfs/go-ipfs/namecache"

	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

type ipnsFollowResult struct {
	Result string
}

var IpnsFollowCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Follow IPNS names.",
		ShortDescription: `
Periodically resolve and optionally pin IPNS names in the background.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"add":    ipnsFollowAddCmd,
		"list":   ipnsFollowListCmd,
		"cancel": ipnsFollowCancelCmd,
	},
}

var ipnsFollowAddCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Follow one or more names",
		ShortDescription: `
Follows an IPNS name by periodically resolving in the backround.
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, true, "IPNS Name to follow."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("pin", "Recursively pin the resolved pointer"),
		cmdkit.StringOption("refresh-interval", "Follow refresh interval."),
	},

	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Namecache == nil {
			res.SetError(errors.New("IPNS Namecache is not available"), cmdkit.ErrClient)
			return
		}

		pin, _, _ := req.Option("pin").Bool()

		refrS, _, err := req.Option("refresh-interval").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		var refr time.Duration
		if refrS == "" {
			refr = nc.DefaultFollowInterval
		} else {
			refr, err = time.ParseDuration(refrS)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		for _, name := range req.Arguments() {
			err = n.Namecache.Follow(name, pin, refr)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		res.SetOutput(&ipnsFollowResult{"ok"})
	},
	Type: ipnsFollowResult{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: marshalFollowResult,
	},
}

var ipnsFollowListCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List names followed by the daemon",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Namecache == nil {
			res.SetError(errors.New("IPNS Namecache is not available"), cmdkit.ErrClient)
			return
		}

		res.SetOutput(&stringList{n.Namecache.ListFollows()})
	},
	Type: stringList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: stringListMarshaler,
	},
}

var ipnsFollowCancelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Cancels a follow",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, true, "Name follow to cancel."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Namecache == nil {
			res.SetError(errors.New("IPNS Namecache is not available"), cmdkit.ErrClient)
			return
		}

		for _, name := range req.Arguments() {
			err = n.Namecache.Unfollow(name)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		res.SetOutput(&ipnsFollowResult{"ok"})
	},
	Type: ipnsFollowResult{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: marshalFollowResult,
	},
}

func marshalFollowResult(res cmds.Response) (io.Reader, error) {
	v, err := unwrapOutput(res.Output())
	if err != nil {
		return nil, err
	}

	output, ok := v.(*ipnsFollowResult)
	if !ok {
		return nil, e.TypeErr(output, v)
	}

	return strings.NewReader(output.Result + "\n"), nil
}
