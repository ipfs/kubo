package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"

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
		Tagline: "Downloads and installs updates for IPFS (disabled)",
		ShortDescription: `ipfs update is disabled until we can deploy the binaries to you over ipfs itself.

		please use 'go get -u github.com/jbenet/go-ipfs/cmd/ipfs' until then.`,
	},
}

// TODO: unexported until we can deploy the binaries over ipfs
var updateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Downloads and installs updates for IPFS",
		ShortDescription: "ipfs update is a utility command used to check for updates and apply them.",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output, err := updateApply(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Type: UpdateOutput{},
	Subcommands: map[string]*cmds.Command{
		"check": UpdateCheckCmd,
		"log":   UpdateLogCmd,
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v := res.Output().(*UpdateOutput)
			var buf bytes.Buffer
			if v.NewVersion != v.OldVersion {
				buf.WriteString(fmt.Sprintf("Successfully updated to IPFS version '%s' (from '%s')\n",
					v.NewVersion, v.OldVersion))
			} else {
				buf.WriteString(fmt.Sprintf("Already updated to latest version ('%s')\n", v.NewVersion))
			}
			return &buf, nil
		},
	},
}

var UpdateCheckCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Checks if updates are available",
		ShortDescription: "'ipfs update check' checks if any updates are available for IPFS.\nNothing will be downloaded or installed.",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output, err := updateCheck(n)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Type: UpdateOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v := res.Output().(*UpdateOutput)
			var buf bytes.Buffer
			if v.NewVersion != v.OldVersion {
				buf.WriteString(fmt.Sprintf("A new version of IPFS is available ('%s', currently running '%s')\n",
					v.NewVersion, v.OldVersion))
			} else {
				buf.WriteString(fmt.Sprintf("Already updated to latest version ('%s')\n", v.NewVersion))
			}
			return &buf, nil
		},
	},
}

var UpdateLogCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "List the changelog for the latest versions of IPFS",
		ShortDescription: "This command is not yet implemented.",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

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
	// TODO
	return nil, errors.New("Not yet implemented")
}
