package commands

import (
	"io"

	"github.com/jbenet/go-ipfs/core"
)

type CmdFunc func(*core.IpfsNode, []string, map[string]interface{}, io.Writer) error
