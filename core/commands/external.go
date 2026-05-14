package commands

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

func ExternalBinary(instructions string) *cmds.Command {
	return &cmds.Command{
		Arguments: []cmds.Argument{
			cmds.StringArg("args", false, true, "Arguments for subcommand."),
		},
		External: true,
		NoRemote: true,
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
						fmt.Fprintf(buf, "%s\n", instructions)
						return res.Emit(buf)
					}
				}

				return fmt.Errorf("%s not installed", binname)
			}

			r, w := io.Pipe()

			cmd := exec.Command(binname, req.Arguments...)

			// TODO: make commands lib be able to pass stdin through daemon
			// cmd.Stdin = req.Stdin()
			cmd.Stdin = io.LimitReader(nil, 0)
			cmd.Stdout = w
			cmd.Stderr = w

			// setup env of child program
			osenv := os.Environ()

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
