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

var updateCmd = &cmds.Command{
	Help: `ipfs update - check for updates and apply them

    ipfs update            - apply
    ipfs update check      - just check
    ipfs update log        - list the changelogs

ipfs update is a utility command used to check for updates and apply them.
`,
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		output, err := updateApply(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(output)
	},
	Type: &UpdateOutput{},
	Subcommands: map[string]*cmds.Command{
		"check": updateCheckCmd,
		"log":   updateLogCmd,
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
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

var updateCheckCmd = &cmds.Command{
	Help: `ipfs update check <key>`,
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		output, err := updateCheck(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(output)
	},
	Type: &UpdateOutput{},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
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

var updateLogCmd = &cmds.Command{
	Help: `ipfs update log - list the last versions and their changelog`,
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		output, err := updateLog(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(output)
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
	return nil, errors.New("Not yet implemented")
}
