package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"

	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
)

func ExternalBinary() *cmds.Command {
	return &cmds.Command{
		Arguments: []cmdkit.Argument{
			cmdkit.StringArg("args", false, true, "Arguments for subcommand."),
		},
		External: true,
		Run: func(req cmds.Request, res cmds.Response) {
			binname := strings.Join(append([]string{"ipfs"}, req.Path()...), "-")
			_, err := exec.LookPath(binname)
			if err != nil {
				// special case for '--help' on uninstalled binaries.
				for _, arg := range req.Arguments() {
					if arg == "--help" || arg == "-h" {
						buf := new(bytes.Buffer)
						fmt.Fprintf(buf, "%s is an 'external' command.\n", binname)
						fmt.Fprintf(buf, "It does not currently appear to be installed.\n")
						fmt.Fprintf(buf, "Please refer to the ipfs documentation for instructions.\n")
						res.SetOutput(buf)
						return
					}
				}

				res.SetError(fmt.Errorf("%s not installed", binname), cmdkit.ErrNormal)
				return
			}

			r, w := io.Pipe()

			cmd := exec.Command(binname, req.Arguments()...)

			// TODO: make commands lib be able to pass stdin through daemon
			//cmd.Stdin = req.Stdin()
			cmd.Stdin = io.LimitReader(nil, 0)
			cmd.Stdout = w
			cmd.Stderr = w

			// setup env of child program
			env := os.Environ()

			// Get the node iff already defined.
			if req.InvocContext().Online {
				nd, err := req.InvocContext().GetNode()
				if err != nil {
					res.SetError(fmt.Errorf(
						"failed to start ipfs node: %s",
						err,
					), cmdkit.ErrFatal)
					return
				}
				env = append(env, fmt.Sprintf("IPFS_ONLINE=%t", nd.OnlineMode()))
			}

			cmd.Env = env

			err = cmd.Start()
			if err != nil {
				res.SetError(fmt.Errorf("failed to start subcommand: %s", err), cmdkit.ErrNormal)
				return
			}

			res.SetOutput(r)

			go func() {
				err = cmd.Wait()
				if err != nil {
					res.SetError(err, cmdkit.ErrNormal)
				}

				w.Close()
			}()
		},
	}
}
