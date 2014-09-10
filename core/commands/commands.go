package commands

import (
	"io"

	"github.com/jbenet/go-ipfs/core"

	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/op/go-logging"
)

var log = logging.MustGetLogger("commands")

type CmdFunc func(*core.IpfsNode, []string, map[string]interface{}, io.Writer) error
