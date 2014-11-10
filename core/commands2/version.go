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
	Description: "Outputs the current version of IPFS",
	Help: `Returns the version number of IPFS and exits.
`,

	Options: []cmds.Option{
		cmds.Option{[]string{"number", "n"}, cmds.Bool,
			"Only output the version number"},
	},
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
