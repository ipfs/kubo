package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jbenet/go-ipfs/core"
)

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
	return nil
}
