package commands

import (
	"io"

	"github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("commands")

type CmdFunc func(*core.IpfsNode, []string, map[string]interface{}, io.Writer) error
