package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jbenet/go-ipfs/core"
	diagn "github.com/jbenet/go-ipfs/diagnostics"
)

func PrintDiagnostics(info []*diagn.DiagInfo, out io.Writer) {
	for _, i := range info {
		fmt.Fprintf(out, "Peer: %s\n", i.ID)
		fmt.Fprintf(out, "\tUp for: %s\n", i.LifeSpan.String())
		fmt.Fprintf(out, "\tConnected To:\n")
		for _, c := range i.Connections {
			fmt.Fprintf(out, "\t%s\n\t\tLatency = %s\n", c.ID, c.Latency.String())
		}
		fmt.Fprintln(out)
	}

}

func Diag(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	if n.Diagnostics == nil {
		return errors.New("Cannot run diagnostic in offline mode!")
	}
	info, err := n.Diagnostics.GetDiagnostic(time.Second * 20)
	if err != nil {
		return err
	}
	raw, ok := opts["raw"].(bool)
	if !ok {
		return errors.New("incorrect value to parameter 'raw'")
	}
	if raw {
		enc := json.NewEncoder(out)
		err = enc.Encode(info)
		if err != nil {
			return err
		}
	} else {
		PrintDiagnostics(info, out)
	}
	return nil
}
