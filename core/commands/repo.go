package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	oldcmds "github.com/ipfs/kubo/commands"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	corerepo "github.com/ipfs/kubo/core/corerepo"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"

	humanize "github.com/dustin/go-humanize"
	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/path"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

type RepoVersion struct {
	Version string
}

var RepoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate the IPFS repo.",
		ShortDescription: `
'ipfs repo' is a plumbing command used to manipulate the repo.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"stat":    repoStatCmd,
		"gc":      repoGcCmd,
		"version": repoVersionCmd,
		"verify":  repoVerifyCmd,
		"migrate": repoMigrateCmd,
		"ls":      RefsLocalCmd,
	},
}

// GcResult is the result returned by "repo gc" command.
type GcResult struct {
	Key   cid.Cid
	Error string `json:",omitempty"`
}

const (
	repoStreamErrorsOptionName   = "stream-errors"
	repoQuietOptionName          = "quiet"
	repoSilentOptionName         = "silent"
	repoAllowDowngradeOptionName = "allow-downgrade"
	repoToVersionOptionName      = "to"
)

var repoGcCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Perform a garbage collection sweep on the repo.",
		ShortDescription: `
'ipfs repo gc' is a plumbing command that will sweep the local
set of stored objects and remove ones that are not pinned in
order to reclaim hard disk space.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(repoStreamErrorsOptionName, "Stream errors."),
		cmds.BoolOption(repoQuietOptionName, "q", "Write minimal output."),
		cmds.BoolOption(repoSilentOptionName, "Write no output."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		silent, _ := req.Options[repoSilentOptionName].(bool)
		streamErrors, _ := req.Options[repoStreamErrorsOptionName].(bool)

		gcOutChan := corerepo.GarbageCollectAsync(n, req.Context)

		if streamErrors {
			errs := false
			for res := range gcOutChan {
				if res.Error != nil {
					if err := re.Emit(&GcResult{Error: res.Error.Error()}); err != nil {
						return err
					}
					errs = true
				} else {
					if err := re.Emit(&GcResult{Key: res.KeyRemoved}); err != nil {
						return err
					}
				}
			}
			if errs {
				return errors.New("encountered errors during gc run")
			}
		} else {
			err := corerepo.CollectResult(req.Context, gcOutChan, func(k cid.Cid) {
				if silent {
					return
				}
				// Nothing to do with this error, really. This
				// most likely means that the client is gone but
				// we still need to let the GC finish.
				_ = re.Emit(&GcResult{Key: k})
			})
			if err != nil {
				return err
			}
		}

		return nil
	},
	Type: GcResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, gcr *GcResult) error {
			quiet, _ := req.Options[repoQuietOptionName].(bool)
			silent, _ := req.Options[repoSilentOptionName].(bool)

			if silent {
				return nil
			}

			if gcr.Error != "" {
				_, err := fmt.Fprintf(w, "Error: %s\n", gcr.Error)
				return err
			}

			prefix := "removed "
			if quiet {
				prefix = ""
			}

			_, err := fmt.Fprintf(w, "%s%s\n", prefix, gcr.Key)
			return err
		}),
	},
}

const (
	repoSizeOnlyOptionName = "size-only"
	repoHumanOptionName    = "human"
)

var repoStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get stats for the currently used repo.",
		ShortDescription: `
'ipfs repo stat' provides information about the local set of
stored objects. It outputs:

RepoSize        int Size in bytes that the repo is currently taking.
StorageMax      string Maximum datastore size (from configuration)
NumObjects      int Number of objects in the local repo.
RepoPath        string The path to the repo being currently used.
Version         string The repo version.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(repoSizeOnlyOptionName, "s", "Only report RepoSize and StorageMax."),
		cmds.BoolOption(repoHumanOptionName, "H", "Print sizes in human readable format (e.g., 1K 234M 2G)"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		sizeOnly, _ := req.Options[repoSizeOnlyOptionName].(bool)
		if sizeOnly {
			sizeStat, err := corerepo.RepoSize(req.Context, n)
			if err != nil {
				return err
			}
			return cmds.EmitOnce(res, &corerepo.Stat{
				SizeStat: sizeStat,
			})
		}

		stat, err := corerepo.RepoStat(req.Context, n)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &stat)
	},
	Type: &corerepo.Stat{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, stat *corerepo.Stat) error {
			wtr := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
			defer wtr.Flush()

			human, _ := req.Options[repoHumanOptionName].(bool)
			sizeOnly, _ := req.Options[repoSizeOnlyOptionName].(bool)

			printSize := func(name string, size uint64) {
				sizeStr := fmt.Sprintf("%d", size)
				if human {
					sizeStr = humanize.Bytes(size)
				}

				fmt.Fprintf(wtr, "%s:\t%s\n", name, sizeStr)
			}

			if !sizeOnly {
				fmt.Fprintf(wtr, "NumObjects:\t%d\n", stat.NumObjects)
			}

			printSize("RepoSize", stat.RepoSize)
			printSize("StorageMax", stat.StorageMax)

			if !sizeOnly {
				fmt.Fprintf(wtr, "RepoPath:\t%s\n", stat.RepoPath)
				fmt.Fprintf(wtr, "Version:\t%s\n", stat.Version)
			}

			return nil
		}),
	},
}

// VerifyProgress reports verification progress to the user.
// It contains either a message about a corrupt block or a progress counter.
type VerifyProgress struct {
	Msg      string // Message about a corrupt/healed block (empty for valid blocks)
	Progress int    // Number of blocks processed so far
}

// verifyState represents the state of a block after verification.
// States track both the verification result and any remediation actions taken.
type verifyState int

const (
	verifyStateValid               verifyState = iota // Block is valid and uncorrupted
	verifyStateCorrupt                                // Block is corrupt, no action taken
	verifyStateCorruptRemoved                         // Block was corrupt and successfully removed
	verifyStateCorruptRemoveFailed                    // Block was corrupt but removal failed
	verifyStateCorruptHealed                          // Block was corrupt, removed, and successfully re-fetched
	verifyStateCorruptHealFailed                      // Block was corrupt and removed, but re-fetching failed
)

const (
	// verifyWorkerMultiplier determines worker pool size relative to CPU count.
	// Since block verification is I/O-bound (disk reads + potential network fetches),
	// we use more workers than CPU cores to maximize throughput.
	verifyWorkerMultiplier = 2
)

// verifyResult contains the outcome of verifying a single block.
// It includes the block's CID, its verification state, and an optional
// human-readable message describing what happened.
type verifyResult struct {
	cid   cid.Cid     // CID of the block that was verified
	state verifyState // Final state after verification and any remediation
	msg   string      // Human-readable message (empty for valid blocks)
}

// verifyWorkerRun processes CIDs from the keys channel, verifying their integrity.
// If shouldDrop is true, corrupt blocks are removed from the blockstore.
// If shouldHeal is true (implies shouldDrop), removed blocks are re-fetched from the network.
// The api parameter must be non-nil when shouldHeal is true.
// healTimeout specifies the maximum time to wait for each block heal (0 = no timeout).
func verifyWorkerRun(ctx context.Context, wg *sync.WaitGroup, keys <-chan cid.Cid, results chan<- *verifyResult, bs bstore.Blockstore, api coreiface.CoreAPI, shouldDrop, shouldHeal bool, healTimeout time.Duration) {
	defer wg.Done()

	sendResult := func(r *verifyResult) bool {
		select {
		case results <- r:
			return true
		case <-ctx.Done():
			return false
		}
	}

	for k := range keys {
		_, err := bs.Get(ctx, k)
		if err != nil {
			// Block is corrupt
			result := &verifyResult{cid: k, state: verifyStateCorrupt}

			if !shouldDrop {
				result.msg = fmt.Sprintf("block %s was corrupt (%s)", k, err)
				if !sendResult(result) {
					return
				}
				continue
			}

			// Try to delete
			if delErr := bs.DeleteBlock(ctx, k); delErr != nil {
				result.state = verifyStateCorruptRemoveFailed
				result.msg = fmt.Sprintf("block %s was corrupt (%s), failed to remove (%s)", k, err, delErr)
				if !sendResult(result) {
					return
				}
				continue
			}

			if !shouldHeal {
				result.state = verifyStateCorruptRemoved
				result.msg = fmt.Sprintf("block %s was corrupt (%s), removed", k, err)
				if !sendResult(result) {
					return
				}
				continue
			}

			// Try to heal by re-fetching from network (api is guaranteed non-nil here)
			healCtx := ctx
			var healCancel context.CancelFunc
			if healTimeout > 0 {
				healCtx, healCancel = context.WithTimeout(ctx, healTimeout)
			}

			if _, healErr := api.Block().Get(healCtx, path.FromCid(k)); healErr != nil {
				result.state = verifyStateCorruptHealFailed
				result.msg = fmt.Sprintf("block %s was corrupt (%s), removed, failed to heal (%s)", k, err, healErr)
			} else {
				result.state = verifyStateCorruptHealed
				result.msg = fmt.Sprintf("block %s was corrupt (%s), removed, healed", k, err)
			}

			if healCancel != nil {
				healCancel()
			}

			if !sendResult(result) {
				return
			}
			continue
		}

		// Block is valid
		if !sendResult(&verifyResult{cid: k, state: verifyStateValid}) {
			return
		}
	}
}

// verifyResultChan creates a channel of verification results by spawning multiple worker goroutines
// to process blocks in parallel. It returns immediately with a channel that will receive results.
func verifyResultChan(ctx context.Context, keys <-chan cid.Cid, bs bstore.Blockstore, api coreiface.CoreAPI, shouldDrop, shouldHeal bool, healTimeout time.Duration) <-chan *verifyResult {
	results := make(chan *verifyResult)

	go func() {
		defer close(results)

		var wg sync.WaitGroup

		for i := 0; i < runtime.NumCPU()*verifyWorkerMultiplier; i++ {
			wg.Add(1)
			go verifyWorkerRun(ctx, &wg, keys, results, bs, api, shouldDrop, shouldHeal, healTimeout)
		}

		wg.Wait()
	}()

	return results
}

var repoVerifyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify all blocks in repo are not corrupted.",
		ShortDescription: `
'ipfs repo verify' checks integrity of all blocks in the local datastore.
Each block is read and validated against its CID to ensure data integrity.

Without any flags, this is a SAFE, read-only check that only reports corrupt
blocks without modifying the repository. This can be used as a "dry run" to
preview what --drop or --heal would do.

Use --drop to remove corrupt blocks, or --heal to remove and re-fetch from
the network.

Examples:
  ipfs repo verify          # safe read-only check, reports corrupt blocks
  ipfs repo verify --drop   # remove corrupt blocks
  ipfs repo verify --heal   # remove and re-fetch corrupt blocks

Exit Codes:
  0: All blocks are valid, OR all corrupt blocks were successfully remediated
     (with --drop or --heal)
  1: Corrupt blocks detected (without flags), OR remediation failed (block
     removal or healing failed with --drop or --heal)

Note: --heal requires the daemon to be running in online mode with network
connectivity to nodes that have the missing blocks. Make sure the daemon is
online and connected to other peers. Healing will attempt to re-fetch each
corrupt block from the network after removing it. If a block cannot be found
on the network, it will remain deleted.

WARNING: Both --drop and --heal are DESTRUCTIVE operations that permanently
delete corrupt blocks from your repository. Once deleted, blocks cannot be
recovered unless --heal successfully fetches them from the network. Blocks
that cannot be healed will remain permanently deleted. Always backup your
repository before using these options.
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("drop", "Remove corrupt blocks from datastore (destructive operation)."),
		cmds.BoolOption("heal", "Remove corrupt blocks and re-fetch from network (destructive operation, implies --drop)."),
		cmds.StringOption("heal-timeout", "Maximum time to wait for each block heal (e.g., \"30s\"). Only applies with --heal.").WithDefault("30s"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		drop, _ := req.Options["drop"].(bool)
		heal, _ := req.Options["heal"].(bool)

		if heal {
			drop = true // heal implies drop
		}

		// Parse and validate heal-timeout
		timeoutStr, _ := req.Options["heal-timeout"].(string)
		healTimeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid heal-timeout: %w", err)
		}
		if healTimeout < 0 {
			return errors.New("heal-timeout must be >= 0")
		}

		// Check online mode and API availability for healing operation
		var api coreiface.CoreAPI
		if heal {
			if !nd.IsOnline {
				return ErrNotOnline
			}
			api, err = cmdenv.GetApi(env, req)
			if err != nil {
				return err
			}
			if api == nil {
				return fmt.Errorf("healing requested but API is not available - make sure daemon is online and connected to other peers")
			}
		}

		bs := &bstore.ValidatingBlockstore{Blockstore: bstore.NewBlockstore(nd.Repo.Datastore())}

		keys, err := bs.AllKeysChan(req.Context)
		if err != nil {
			log.Error(err)
			return err
		}

		results := verifyResultChan(req.Context, keys, bs, api, drop, heal, healTimeout)

		// Track statistics for each type of outcome
		var corrupted, removed, removeFailed, healed, healFailed int
		var i int

		for result := range results {
			// Update counters based on the block's final state
			switch result.state {
			case verifyStateCorrupt:
				// Block is corrupt but no action was taken (--drop not specified)
				corrupted++
			case verifyStateCorruptRemoved:
				// Block was corrupt and successfully removed (--drop specified)
				corrupted++
				removed++
			case verifyStateCorruptRemoveFailed:
				// Block was corrupt but couldn't be removed
				corrupted++
				removeFailed++
			case verifyStateCorruptHealed:
				// Block was corrupt, removed, and successfully re-fetched (--heal specified)
				corrupted++
				removed++
				healed++
			case verifyStateCorruptHealFailed:
				// Block was corrupt and removed, but re-fetching failed
				corrupted++
				removed++
				healFailed++
			default:
				// verifyStateValid blocks are not counted (they're the expected case)
			}

			// Emit progress message for corrupt blocks
			if result.state != verifyStateValid && result.msg != "" {
				if err := res.Emit(&VerifyProgress{Msg: result.msg}); err != nil {
					return err
				}
			}

			i++
			if err := res.Emit(&VerifyProgress{Progress: i}); err != nil {
				return err
			}
		}

		if err := req.Context.Err(); err != nil {
			return err
		}

		if corrupted > 0 {
			// Build a summary of what happened with corrupt blocks
			summary := fmt.Sprintf("verify complete, %d blocks corrupt", corrupted)
			if removed > 0 {
				summary += fmt.Sprintf(", %d removed", removed)
			}
			if removeFailed > 0 {
				summary += fmt.Sprintf(", %d failed to remove", removeFailed)
			}
			if healed > 0 {
				summary += fmt.Sprintf(", %d healed", healed)
			}
			if healFailed > 0 {
				summary += fmt.Sprintf(", %d failed to heal", healFailed)
			}

			// Determine success/failure based on operation mode
			shouldFail := false

			if !drop {
				// Detection-only mode: always fail if corruption found
				shouldFail = true
			} else if heal {
				// Heal mode: fail if any removal or heal failed
				shouldFail = (removeFailed > 0 || healFailed > 0)
			} else {
				// Drop mode: fail if any removal failed
				shouldFail = (removeFailed > 0)
			}

			if shouldFail {
				return errors.New(summary)
			}

			// Success: emit summary as a message instead of error
			return res.Emit(&VerifyProgress{Msg: summary})
		}

		return res.Emit(&VerifyProgress{Msg: "verify complete, all blocks validated."})
	},
	Type: &VerifyProgress{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, obj *VerifyProgress) error {
			if strings.Contains(obj.Msg, "was corrupt") {
				fmt.Fprintln(w, obj.Msg)
				return nil
			}

			if obj.Msg != "" {
				if len(obj.Msg) < 20 {
					obj.Msg += "             "
				}
				fmt.Fprintln(w, obj.Msg)
				return nil
			}

			fmt.Fprintf(w, "%d blocks processed.\r", obj.Progress)
			return nil
		}),
	},
}

var repoVersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show the repo version.",
		ShortDescription: `
'ipfs repo version' returns the current repo version.
`,
	},

	Options: []cmds.Option{
		cmds.BoolOption(repoQuietOptionName, "q", "Write minimal output."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return cmds.EmitOnce(res, &RepoVersion{
			Version: fmt.Sprint(fsrepo.RepoVersion),
		})
	},
	Type: RepoVersion{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *RepoVersion) error {
			quiet, _ := req.Options[repoQuietOptionName].(bool)

			if quiet {
				fmt.Fprintf(w, "fs-repo@%s\n", out.Version)
			} else {
				fmt.Fprintf(w, "ipfs repo version fs-repo@%s\n", out.Version)
			}
			return nil
		}),
	},
}

var repoMigrateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Apply repository migrations to a specific version.",
		ShortDescription: `
'ipfs repo migrate' applies repository migrations to bring the repository
to a specific version. By default, migrates to the latest version supported
by this IPFS binary.

Examples:
  ipfs repo migrate                # Migrate to latest version
  ipfs repo migrate --to=17       # Migrate to version 17
  ipfs repo migrate --to=16 --allow-downgrade  # Downgrade to version 16

WARNING: Downgrading a repository may cause data loss and requires using
an older IPFS binary that supports the target version. After downgrading,
you must use an IPFS implementation compatible with that repository version.

Repository versions 16+ use embedded migrations for faster, more reliable
migration. Versions below 16 require external migration tools.
`,
	},
	Options: []cmds.Option{
		cmds.IntOption(repoToVersionOptionName, "Target repository version").WithDefault(fsrepo.RepoVersion),
		cmds.BoolOption(repoAllowDowngradeOptionName, "Allow downgrading to a lower repo version"),
	},
	NoRemote: true,
	// SetDoesNotUseRepo(true) might seem counter-intuitive since migrations
	// do access the repo, but it's correct - we need direct filesystem access
	// without going through the daemon. Migrations handle their own locking.
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cctx := env.(*oldcmds.Context)
		allowDowngrade, _ := req.Options[repoAllowDowngradeOptionName].(bool)
		targetVersion, _ := req.Options[repoToVersionOptionName].(int)

		// Get current repo version
		currentVersion, err := migrations.RepoVersion(cctx.ConfigRoot)
		if err != nil {
			return fmt.Errorf("could not get current repo version: %w", err)
		}

		// Check if migration is needed
		if currentVersion == targetVersion {
			fmt.Printf("Repository is already at version %d.\n", targetVersion)
			return nil
		}

		// Validate downgrade request
		if targetVersion < currentVersion && !allowDowngrade {
			return fmt.Errorf("downgrade from version %d to %d requires --allow-downgrade flag", currentVersion, targetVersion)
		}

		fmt.Printf("Migrating repository from version %d to %d...\n", currentVersion, targetVersion)

		// Use hybrid migration strategy that intelligently combines external and embedded migrations
		// Use req.Context instead of cctx.Context() to avoid opening the repo before migrations run,
		// which would acquire the lock that migrations need
		err = migrations.RunHybridMigrations(req.Context, targetVersion, cctx.ConfigRoot, allowDowngrade)
		if err != nil {
			fmt.Println("Repository migration failed:")
			fmt.Printf("  %s\n", err)
			fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
			fmt.Println("  https://github.com/ipfs/kubo")
			return err
		}

		fmt.Printf("Repository successfully migrated to version %d.\n", targetVersion)
		if targetVersion < fsrepo.RepoVersion {
			fmt.Println("WARNING: After downgrading, you must use an IPFS binary compatible with this repository version.")
		}
		return nil
	},
}
