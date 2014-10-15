package commands

import (
	"io"

	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
)

// Log changes the log level of a subsystem
func Log(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	if err := u.SetLogLevel(args[0], args[1]); err != nil {
		return err
	}

	log.Info("Changed Log of '%q' to '%q'", args[0], args[1])
	return nil
}
