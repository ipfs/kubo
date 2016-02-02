package commands

import (
	"bytes"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
)

var ActiveReqsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List commands run on this ipfs node",
		ShortDescription: `
Lists running and recently run commands.
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetOutput(req.InvocContext().ReqLog.Report())
	},
	Options: []cmds.Option{
		cmds.BoolOption("v", "verbose", "print more verbose output"),
	},
	Marshalers: map[cmds.EncodingType]cmds.Marshaler{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out, ok := res.Output().(*[]*cmds.ReqLogEntry)
			if !ok {
				log.Errorf("%#v", res.Output())
				return nil, cmds.ErrIncorrectType
			}
			buf := new(bytes.Buffer)

			verbose, _, _ := res.Request().Option("v").Bool()

			w := tabwriter.NewWriter(buf, 4, 4, 2, ' ', 0)
			fmt.Fprint(w, "Command\t")
			if verbose {
				fmt.Fprint(w, "Arguments\tOptions\t")
			}
			fmt.Fprintln(w, "Active\tStartTime\tRunTime")

			for _, req := range *out {
				fmt.Fprintf(w, "%s\t", req.Command)
				if verbose {
					fmt.Fprintf(w, "%v\t[", req.Args)
					for k, v := range req.Options {
						fmt.Fprintf(w, "%s=%v,", k, v)
					}
					fmt.Fprintf(w, "]\t")
				}

				var live time.Duration
				if req.Active {
					live = time.Now().Sub(req.StartTime)
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
