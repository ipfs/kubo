package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/updates"
)

// UpdateApply applys an update of the ipfs binary and shuts down the node if successful
func UpdateApply(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	fmt.Fprintln(out, "Current Version:", updates.Version)
	u, err := updates.CheckForUpdate()
	if err != nil {
		return err
	}

	if u == nil {
		fmt.Fprintln(out, "No update available")
		return nil
	}
	fmt.Fprintln(out, "New Version:", u.Version)

	_, onDaemon := opts["onDaemon"]
	force := opts["force"].(bool)
	if onDaemon && !force {
		return fmt.Errorf(`Error: update must stop running ipfs service.
You may want to abort the update, or shut the service down manually.
To shut it down automatically, run:

  ipfs update --force
`)
	}

	if err = updates.Apply(u); err != nil {
		fmt.Fprint(out, err.Error())
		return fmt.Errorf("Couldn't apply update: %v", err)
	}

	fmt.Fprintln(out, "Updated applied!")
	if onDaemon {
		if force {
			fmt.Fprintln(out, "Shutting down ipfs service.")
			os.Exit(1) // is there a cleaner shutdown routine?
		} else {
			fmt.Fprintln(out, "You can now restart the ipfs service.")
		}
	}

	return nil
}

// UpdateCheck checks wether there is an update available
func UpdateCheck(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	fmt.Fprintln(out, "Current Version:", updates.Version)
	u, err := updates.CheckForUpdate()
	if err != nil {
		return err
	}

	if u == nil {
		fmt.Fprintln(out, "No update available")
		return nil
	}

	fmt.Fprintln(out, "New Version:", u.Version)
	return nil
}

// UpdateLog lists the version available online
func UpdateLog(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	return errors.New("Not yet implemented")
}
