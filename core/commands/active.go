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
	Marshalers: map[cmds.EncodingType]cmds.Marshaler{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			out, ok := res.Output().(*[]*cmds.ReqLogEntry)
			if !ok {
				log.Errorf("%#v", res.Output())
				return nil, cmds.ErrIncorrectType
			}
			buf := new(bytes.Buffer)

			w := tabwriter.NewWriter(buf, 4, 4, 2, ' ', 0)
			fmt.Fprintln(w, "Command\tActive\tStartTime\tRunTime")
			for _, req := range *out {
				if req.Active {
					fmt.Fprintf(w, "%s\t%s\t%s\n", req.Command, "true", req.StartTime, time.Now().Sub(req.StartTime))
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\n", req.Command, "false", req.StartTime, req.EndTime.Sub(req.StartTime))
				}
			}
			w.Flush()

			return buf, nil
		},
	},
	Type: []*cmds.ReqLogEntry{},
}
