package commands

import (
	"errors"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
)

type VersionOutput struct {
	Version string
}

var versionCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"number", "n"}, cmds.Bool},
	},
	Help: `ipfs version - Show ipfs version information.

    Returns the current version of ipfs and exits.
  `,
	Run: func(res cmds.Response, req cmds.Request) {
		res.SetOutput(&VersionOutput{
			Version: config.CurrentVersionNumber,
		})
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*VersionOutput)
			s := ""

			opt, found := res.Request().Option("number")
			number, ok := opt.(bool)
			if found && !ok {
				return nil, errors.New("cast error")
			}

			if !number {
				s += "ipfs version "
			}
			s += v.Version
			return []byte(s), nil
		},
	},
	Type: &VersionOutput{},
}
