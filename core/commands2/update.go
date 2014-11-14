package commands

import (
	"errors"
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/updates"
)

type UpdateOutput struct {
	OldVersion string
	NewVersion string
}

var UpdateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Downloads and installs updates for IPFS",
		ShortDescription: "ipfs update is a utility command used to check for updates and apply them.",
	},

	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}
		return updateApply(n)
	},
	Type: &UpdateOutput{},
	Subcommands: map[string]*cmds.Command{
		"check": UpdateCheckCmd,
		"log":   UpdateLogCmd,
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*UpdateOutput)
			s := ""
			if v.NewVersion != v.OldVersion {
				s = fmt.Sprintf("Successfully updated to IPFS version '%s' (from '%s')",
					v.NewVersion, v.OldVersion)
			} else {
				s = fmt.Sprintf("Already updated to latest version ('%s')", v.NewVersion)
			}
			return []byte(s), nil
		},
	},
}

var UpdateCheckCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Checks if updates are available",
		ShortDescription: `
'ipfs update check' checks if any updates are available for IPFS.

Nothing will be downloaded or installed.
`,
	},

	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}
		return updateCheck(n)
	},
	Type: &UpdateOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*UpdateOutput)
			s := ""
			if v.NewVersion != v.OldVersion {
				s = fmt.Sprintf("A new version of IPFS is available ('%s', currently running '%s')",
					v.NewVersion, v.OldVersion)
			} else {
				s = fmt.Sprintf("Already updated to latest version ('%s')", v.NewVersion)
			}
			return []byte(s), nil
		},
	},
}

var UpdateLogCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "List the changelog for the latest versions of IPFS",
		ShortDescription: "This command is not yet implemented.",
	},

	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}
		return updateLog(n)
	},
}

// updateApply applies an update of the ipfs binary and shuts down the node if successful
func updateApply(n *core.IpfsNode) (*UpdateOutput, error) {
	// TODO: 'force bool' param that stops the daemon (if running) before update

	output := &UpdateOutput{
		OldVersion: updates.Version,
	}

	u, err := updates.CheckForUpdate()
	if err != nil {
		return nil, err
	}

	if u == nil {
		output.NewVersion = updates.Version
		return output, nil
	}

	output.NewVersion = u.Version

	if n.OnlineMode() {
		return nil, errors.New(`You must stop the IPFS daemon before updating.`)
	}

	if err = updates.Apply(u); err != nil {
		return nil, err
	}

	return output, nil
}

// updateCheck checks wether there is an update available
func updateCheck(n *core.IpfsNode) (*UpdateOutput, error) {
	output := &UpdateOutput{
		OldVersion: updates.Version,
	}

	u, err := updates.CheckForUpdate()
	if err != nil {
		return nil, err
	}

	if u == nil {
		output.NewVersion = updates.Version
		return output, nil
	}

	output.NewVersion = u.Version
	return output, nil
}

// updateLog lists the version available online
func updateLog(n *core.IpfsNode) (interface{}, error) {
	// TODO
	return nil, errors.New("Not yet implemented")
}
