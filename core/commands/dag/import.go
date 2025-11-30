package dagcmd

import (
	"errors"

	"github.com/ipfs/boxo/files"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/coreiface/options"
)

func dagImport(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	node, err := cmdenv.GetNode(env)
	if err != nil {
		return err
	}

	cfg, err := node.Repo.Config()
	if err != nil {
		return err
	}

	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	// Ensure offline mode - import should not reach out to network
	api, err = api.WithOptions(options.Api.Offline(true))
	if err != nil {
		return err
	}

	// Parse options
	doPinRoots, _ := req.Options[pinRootsOptionName].(bool)
	doStats, _ := req.Options[statsOptionName].(bool)
	fastProvideRoot, fastProvideRootSet := req.Options[fastProvideRootOptionName].(bool)
	fastProvideWait, fastProvideWaitSet := req.Options[fastProvideWaitOptionName].(bool)

	// Resolve fast-provide options from config if not explicitly set
	fastProvideRoot = config.ResolveBoolFromConfig(fastProvideRoot, fastProvideRootSet, cfg.Import.FastProvideRoot, config.DefaultFastProvideRoot)
	fastProvideWait = config.ResolveBoolFromConfig(fastProvideWait, fastProvideWaitSet, cfg.Import.FastProvideWait, config.DefaultFastProvideWait)

	// Build CoreAPI options
	dagOpts := []options.DagImportOption{
		options.Dag.PinRoots(doPinRoots),
		options.Dag.Stats(doStats),
		options.Dag.FastProvideRoot(fastProvideRoot),
		options.Dag.FastProvideWait(fastProvideWait),
	}

	// Process each file
	it := req.Files.Entries()
	for it.Next() {
		file := files.FileFromEntry(it)
		if file == nil {
			return errors.New("expected a file handle")
		}

		// Call CoreAPI to import the file
		resultChan, err := api.Dag().Import(req.Context, file, dagOpts...)
		if err != nil {
			return err
		}

		// Stream results back to user
		for result := range resultChan {
			// Check for errors from CoreAPI
			if result.Err != nil {
				return result.Err
			}

			// Emit root results
			if result.Root != nil {
				err := res.Emit(&CarImportOutput{
					Root: &RootMeta{
						Cid:         result.Root.Cid,
						PinErrorMsg: result.Root.PinErrorMsg,
					},
				})
				if err != nil {
					return err
				}
			}

			// Emit stats results
			if result.Stats != nil {
				err := res.Emit(&CarImportOutput{
					Stats: &CarImportStats{
						BlockCount:      result.Stats.BlockCount,
						BlockBytesCount: result.Stats.BlockBytesCount,
					},
				})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
