package legacy

import (
	"io"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
)

var log = logging.Logger("cmds/lgc")

// NewCommand returns a Command from an oldcmds.Command
func NewCommand(oldcmd *oldcmds.Command) *cmds.Command {
	if oldcmd == nil {
		return nil
	}
	var cmd *cmds.Command

	cmd = &cmds.Command{
		Options:   oldcmd.Options,
		Arguments: oldcmd.Arguments,
		Helptext:  oldcmd.Helptext,
		External:  oldcmd.External,
		Type:      oldcmd.Type,

		Subcommands: make(map[string]*cmds.Command),
	}

	if oldcmd.Run != nil {
		cmd.Run = func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
			oldReq := &requestWrapper{req, OldContext(env)}
			res := &fakeResponse{req: oldReq, re: re, wait: make(chan struct{})}

			errCh := make(chan error)
			go res.Send(errCh)
			oldcmd.Run(oldReq, res)
			err := <-errCh
			if err != nil {
				log.Error(err)
			}
		}
	}

	if oldcmd.PreRun != nil {
		cmd.PreRun = func(req *cmds.Request, env cmds.Environment) error {
			oldReq := &requestWrapper{req, OldContext(env)}
			return oldcmd.PreRun(oldReq)
		}
	}

	for name, sub := range oldcmd.Subcommands {
		cmd.Subcommands[name] = NewCommand(sub)
	}

	cmd.Encoders = make(cmds.EncoderMap)

	for encType, m := range oldcmd.Marshalers {
		cmd.Encoders[cmds.EncodingType(encType)] = func(m oldcmds.Marshaler, encType oldcmds.EncodingType) func(req *cmds.Request) func(io.Writer) cmds.Encoder {
			return func(req *cmds.Request) func(io.Writer) cmds.Encoder {
				return func(w io.Writer) cmds.Encoder {
					return NewMarshalerEncoder(req, m, w)
				}
			}
		}(m, encType)
	}

	return cmd
}
