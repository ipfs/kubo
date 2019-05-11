package commands

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

	oldcmds "github.com/ipfs/go-ipfs/commands"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

const (
	verboseOptionName = "verbose"
)

var ActiveReqsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List commands run on this IPFS node.",
		ShortDescription: `
Lists running and recently run commands.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := env.(*oldcmds.Context)
		return cmds.EmitOnce(res, ctx.ReqLog.Report())
	},
	Options: []cmds.Option{
		cmds.BoolOption(verboseOptionName, "v", "Print extra information."),
	},
	Subcommands: map[string]*cmds.Command{
		"clear":    clearInactiveCmd,
		"set-time": setRequestClearCmd,
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *[]*cmds.ReqLogEntry) error {
			verbose, _ := req.Options[verboseOptionName].(bool)

			tw := tabwriter.NewWriter(w, 4, 4, 2, ' ', 0)
			if verbose {
				fmt.Fprint(tw, "ID\t")
			}
			fmt.Fprint(tw, "Command\t")
			if verbose {
				fmt.Fprint(tw, "Arguments\tOptions\t")
			}
			fmt.Fprintln(tw, "Active\tStartTime\tRunTime")

			for _, req := range *out {
				if verbose {
					fmt.Fprintf(tw, "%d\t", req.ID)
				}
				fmt.Fprintf(tw, "%s\t", req.Command)
				if verbose {
					fmt.Fprintf(tw, "%v\t[", req.Args)
					var keys []string
					for k := range req.Options {
						keys = append(keys, k)
					}
					sort.Strings(keys)

					for _, k := range keys {
						fmt.Fprintf(tw, "%s=%v,", k, req.Options[k])
					}
					fmt.Fprintf(tw, "]\t")
				}

				var live time.Duration
				if req.Active {
					live = time.Since(req.StartTime)
				} else {
					live = req.EndTime.Sub(req.StartTime)
				}
				t := req.StartTime.Format(time.Stamp)
				fmt.Fprintf(tw, "%t\t%s\t%s\n", req.Active, t, live)
			}
			tw.Flush()

			return nil
		}),
	},
	Type: []*cmds.ReqLogEntry{},
}

var clearInactiveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Clear inactive requests from the log.",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := env.(*oldcmds.Context)
		ctx.ReqLog.ClearInactive()
		return nil
	},
}

var setRequestClearCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Set how long to keep inactive requests in the log.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("time", true, false, "Time to keep inactive requests in log."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		tval, err := time.ParseDuration(req.Arguments[0])
		if err != nil {
			return err
		}
		ctx := env.(*oldcmds.Context)
		ctx.ReqLog.SetKeepTime(tval)

		return nil
	},
}
