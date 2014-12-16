package commands

import (
	"fmt"
	"io"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
)

type VersionOutput struct {
	Version string
}

var VersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Shows ipfs version information",
		ShortDescription: "Returns the current version of ipfs and exits.",
	},

	Options: []cmds.Option{
		cmds.BoolOption("number", "n", "Only show the version number"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		return &VersionOutput{
			Version: config.CurrentVersionNumber,
		}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v := res.Output().(*VersionOutput)

			number, found, err := res.Request().Option("number").Bool()
			if err != nil {
				return nil, err
			}
			if found && number {
				return strings.NewReader(fmt.Sprintln(v.Version)), nil
			}
			return strings.NewReader(fmt.Sprintf("ipfs version %s\n", v.Version)), nil
		},
	},
	Type: &VersionOutput{},
}
