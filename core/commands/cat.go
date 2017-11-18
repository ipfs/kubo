package commands

import (
	"context"
	"io"
	"os"

	core "github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"

	"gx/ipfs/QmSNbH2A1evCCbJSDC6u3RV3GGDhgu6pRGbXHvrN89tMKf/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmUgr8HrEkQqXfBPtj1A2UEg1V7cvhUhDsmL44wFPCJk5k/go-ipfs-cmds"
)

const progressBarMinSize = 1024 * 1024 * 8 // show progress bar for outputs > 8MiB

var CatCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Show IPFS object data.",
		ShortDescription: "Displays the data contained by an IPFS or IPNS object(s) at the given path.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to be outputted.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.ResponseEmitter) {
		node, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !node.OnlineMode() {
			if err := node.SetupOfflineRouting(); err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		readers, length, err := cat(req.Context(), node, req.Arguments())
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		/*
			if err := corerepo.ConditionalGC(req.Context(), node, length); err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
		*/

		res.SetLength(length)
		reader := io.MultiReader(readers...)

		// Since the reader returns the error that a block is missing, and that error is
		// returned from io.Copy inside Emit, we need to take Emit errors and send
		// them to the client. Usually we don't do that because it means the connection
		// is broken or we supplied an illegal argument etc.
		err = res.Emit(reader)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
		}
	},
	PostRun: map[cmds.EncodingType]func(cmds.Request, cmds.ResponseEmitter) cmds.ResponseEmitter{
		cmds.CLI: func(req cmds.Request, re cmds.ResponseEmitter) cmds.ResponseEmitter {
			reNext, res := cmds.NewChanResponsePair(req)

			go func() {
				if res.Length() > 0 && res.Length() < progressBarMinSize {
					if err := cmds.Copy(re, res); err != nil {
						re.SetError(err, cmdkit.ErrNormal)
					}

					return
				}

				// Copy closes by itself, so we must not do this before
				defer re.Close()
				for {
					v, err := res.Next()
					if !cmds.HandleError(err, res, re) {
						break
					}

					switch val := v.(type) {
					case io.Reader:
						bar, reader := progressBarForReader(os.Stderr, val, int64(res.Length()))
						bar.Start()

						err = re.Emit(reader)
						if err != nil {
							log.Error(err)
						}
					default:
						log.Warningf("cat postrun: received unexpected type %T", val)
					}
				}
			}()

			return reNext
		},
	},
}

func cat(ctx context.Context, node *core.IpfsNode, paths []string) ([]io.Reader, uint64, error) {
	readers := make([]io.Reader, 0, len(paths))
	length := uint64(0)
	for _, fpath := range paths {
		read, err := coreunix.Cat(ctx, node, fpath)
		if err != nil {
			return nil, 0, err
		}
		readers = append(readers, read)
		length += uint64(read.Size())
	}
	return readers, length, nil
}
