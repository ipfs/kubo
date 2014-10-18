package commands

import (
	"errors"
	"io"

	"github.com/jbenet/go-ipfs/core"
)

// UpdateApply applys an update of the ipfs binary and shuts down the node if successful
func UpdateApply(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	return errors.New("TODOUpdateApply")
}

// UpdateCheck checks wether there is an update available
func UpdateCheck(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	return errors.New("TODOUpdateCheck")
}

func UpdateLog(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	return errors.New("TODOUpdateLog")
}
