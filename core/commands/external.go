package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	commands "github.com/ipfs/go-ipfs/commands"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

func ExternalBinary() *cmds.Command {
	return &cmds.Command{
		Arguments: []cmdkit.Argument{
			cmdkit.StringArg("args", false, true, "Arguments for subcommand."),
		},
		External: true,
		Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
			binname := strings.Join(append([]string{"ipfs"}, req.Path...), "-")
			_, err := exec.LookPath(binname)
			if err != nil {
				// special case for '--help' on uninstalled binaries.
				for _, arg := range req.Arguments {
					if arg == "--help" || arg == "-h" {
						buf := new(bytes.Buffer)
						fmt.Fprintf(buf, "%s is an 'external' command.\n", binname)
						fmt.Fprintf(buf, "It does not currently appear to be installed.\n")
						fmt.Fprintf(buf, "Please refer to the ipfs documentation for instructions.\n")
						return res.Emit(buf)
					}
				}

				return fmt.Errorf("%s not installed", binname)
			}

			r, w := io.Pipe()

			cmd := exec.Command(binname, req.Arguments...)

			// TODO: make commands lib be able to pass stdin through daemon
			//cmd.Stdin = req.Stdin()
			cmd.Stdin = io.LimitReader(nil, 0)
			cmd.Stdout = w
			cmd.Stderr = w

			// setup env of child program
			osenv := os.Environ()

			// Get the node iff already defined.
			if cctx, ok := env.(*commands.Context); ok && cctx.Online {
				nd, err := cctx.GetNode()
				if err != nil {
					return fmt.Errorf("failed to start ipfs node: %s", err)
				}
				osenv = append(osenv, fmt.Sprintf("IPFS_ONLINE=%t", nd.OnlineMode()))
			}

			cmd.Env = osenv

			err = cmd.Start()
			if err != nil {
				return fmt.Errorf("failed to start subcommand: %s", err)
			}

			errC := make(chan error)

			go func() {
				var err error
				defer func() { errC <- err }()
				err = cmd.Wait()
				w.Close()
			}()

			err = res.Emit(r)
			if err != nil {
				return err
			}

			return <-errC
		},
	}
}
