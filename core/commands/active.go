package commands

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	"gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
)

var ActiveReqsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List commands run on this IPFS node.",
		ShortDescription: `
Lists running and recently run commands.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetOutput(req.InvocContext().ReqLog.Report())
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
	},
	Subcommands: map[string]*cmds.Command{
		"clear":    clearInactiveCmd,
		"set-time": setRequestClearCmd,
	},
	Marshalers: map[cmds.EncodingType]cmds.Marshaler{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			out, ok := v.(*[]*cmds.ReqLogEntry)
			if !ok {
				return nil, e.TypeErr(out, v)
			}
			buf := new(bytes.Buffer)

			verbose, _, _ := res.Request().Option("v").Bool()

			w := tabwriter.NewWriter(buf, 4, 4, 2, ' ', 0)
			if verbose {
				fmt.Fprint(w, "ID\t")
			}
			fmt.Fprint(w, "Command\t")
			if verbose {
				fmt.Fprint(w, "Arguments\tOptions\t")
			}
			fmt.Fprintln(w, "Active\tStartTime\tRunTime")

			for _, req := range *out {
				if verbose {
					fmt.Fprintf(w, "%d\t", req.ID)
				}
				fmt.Fprintf(w, "%s\t", req.Command)
				if verbose {
					fmt.Fprintf(w, "%v\t[", req.Args)
					var keys []string
					for k := range req.Options {
						keys = append(keys, k)
					}
					sort.Strings(keys)

					for _, k := range keys {
						fmt.Fprintf(w, "%s=%v,", k, req.Options[k])
					}
					fmt.Fprintf(w, "]\t")
				}

				var live time.Duration
				if req.Active {
					live = time.Since(req.StartTime)
				} else {
					live = req.EndTime.Sub(req.StartTime)
				}
				t := req.StartTime.Format(time.Stamp)
				fmt.Fprintf(w, "%t\t%s\t%s\n", req.Active, t, live)
			}
			w.Flush()

			return buf, nil
		},
	},
	Type: []*cmds.ReqLogEntry{},
}

var clearInactiveCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Clear inactive requests from the log.",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		req.InvocContext().ReqLog.ClearInactive()
	},
}

var setRequestClearCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Set how long to keep inactive requests in the log.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("time", true, false, "Time to keep inactive requests in log."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		tval, err := time.ParseDuration(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		req.InvocContext().ReqLog.SetKeepTime(tval)
	},
}
