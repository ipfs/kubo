package commands

import (
	"io"

	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"

	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/op/go-logging"
)

var log = u.Logger("commands", logging.ERROR)

type CmdFunc func(*core.IpfsNode, []string, map[string]interface{}, io.Writer) error
