package name

import (
	"fmt"
	"io"
	"time"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	nc "github.com/ipfs/go-ipfs/namecache"

	"gx/ipfs/QmR77mMvvh8mJBBWQmBfQBu8oD38NUN4KE9SL2gDgAQNc6/go-ipfs-cmds"
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
		cmdkit.StringOption("refresh-interval", "Follow refresh interval; defaults to 1hr."),
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if n.Namecache == nil {
			return cmdkit.Errorf(cmdkit.ErrClient, "IPNS Namecache is not available")
		}

		prefetch, _ := req.Options["prefetch"].(bool)
		refrS, _ := req.Options["refresh-interval"].(string)
		refr := nc.DefaultFollowInterval

		if refrS != "" {
			refr, err = time.ParseDuration(refrS)
			if err != nil {
				return err
			}
		}

		for _, name := range req.Arguments {
			err = n.Namecache.Follow(name, prefetch, refr)
			if err != nil {
				return err
			}
		}

		return cmds.EmitOnce(res, &ipnsFollowResult{"ok"})
	},
	Type: ipnsFollowResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: encodeFollowResult(),
	},
}

var ipnsFollowListCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List names followed by the daemon",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if n.Namecache == nil {
			return cmdkit.Errorf(cmdkit.ErrClient, "IPNS Namecache is not available")
		}

		return cmds.EmitOnce(res, &stringList{n.Namecache.ListFollows()})
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: stringListEncoder(),
	},
}

var ipnsFollowCancelCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Cancels a follow",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", true, true, "Name follow to cancel."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if n.Namecache == nil {
			return cmdkit.Errorf(cmdkit.ErrClient, "IPNS Namecache is not available")
		}

		for _, name := range req.Arguments {
			err = n.Namecache.Unfollow(name)
			if err != nil {
				return err
			}
		}

		return cmds.EmitOnce(res, &ipnsFollowResult{"ok"})
	},
	Type: ipnsFollowResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: encodeFollowResult(),
	},
}

func encodeFollowResult() cmds.EncoderFunc {
	return cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, list *ipnsFollowResult) error {
		_, err := fmt.Fprintf(w, "%s\n", list.Result)
		return err
	})
}
