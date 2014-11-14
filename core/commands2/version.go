package commands

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
)

type VersionOutput struct {
	Version string
}

var VersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Shows ipfs version information",
		ShortDescription: "Shows ipfs version information",
		LongDescription: `ipfs version - Show ipfs version information.

		Returns the current version of ipfs and exits.
		`,
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
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*VersionOutput)

			number, found, err := res.Request().Option("number").Bool()
			if err != nil {
				return nil, err
			}
			if found && number {
				return []byte(fmt.Sprintln(v.Version)), nil
			}
			return []byte(fmt.Sprintf("ipfs version %s\n", v.Version)), nil
		},
	},
	Type: &VersionOutput{},
}
