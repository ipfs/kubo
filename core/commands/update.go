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

	if err = updates.AbleToApply(); err != nil {
		return fmt.Errorf("Can't apply update: %v", err)
	}

	if err, errRecover := u.Update(); err != nil {
		err = fmt.Errorf("Update failed: %v\n", err)
		if errRecover != nil {
			err = fmt.Errorf("%s\nRecovery failed! Cause: %v\nYou may need to recover manually", err, errRecover)
		}
		fmt.Fprint(out, err.Error())
		return err
	}

	fmt.Fprintln(out, "Updated applied! Shutting down.")
	os.Exit(0)
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
