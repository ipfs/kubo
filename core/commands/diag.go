package commands

import (
	"encoding/json"
	"io"
	"time"

	"github.com/jbenet/go-ipfs/core"
)

func Diag(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	info, err := n.Diagnostics.GetDiagnostic(time.Second * 20)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	err = enc.Encode(info)
	if err != nil {
		return err
	}
	return nil
}
