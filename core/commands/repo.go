package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"text/tabwriter"

	oldcmds "github.com/ipfs/kubo/commands"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	corerepo "github.com/ipfs/kubo/core/corerepo"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"
	"github.com/ipfs/kubo/repo/fsrepo/migrations/ipfsfetcher"

	humanize "github.com/dustin/go-humanize"
	bstore "github.com/ipfs/boxo/blockstore"
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

type VerifyProgress struct {
	Msg      string
	Progress int
}

func verifyWorkerRun(ctx context.Context, wg *sync.WaitGroup, keys <-chan cid.Cid, results chan<- string, bs bstore.Blockstore) {
	defer wg.Done()

	for k := range keys {
		_, err := bs.Get(ctx, k)
		if err != nil {
			select {
			case results <- fmt.Sprintf("block %s was corrupt (%s)", k, err):
			case <-ctx.Done():
				return
			}

			continue
		}

		select {
		case results <- "":
		case <-ctx.Done():
			return
		}
	}
}

func verifyResultChan(ctx context.Context, keys <-chan cid.Cid, bs bstore.Blockstore) <-chan string {
	results := make(chan string)

	go func() {
		defer close(results)

		var wg sync.WaitGroup

		for i := 0; i < runtime.NumCPU()*2; i++ {
			wg.Add(1)
			go verifyWorkerRun(ctx, &wg, keys, results, bs)
		}

		wg.Wait()
	}()

	return results
}

var repoVerifyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify all blocks in repo are not corrupted.",
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		bs := bstore.NewBlockstore(nd.Repo.Datastore())
		bs.HashOnRead(true)

		keys, err := bs.AllKeysChan(req.Context)
		if err != nil {
			log.Error(err)
			return err
		}

		results := verifyResultChan(req.Context, keys, bs)

		var fails int
		var i int
		for msg := range results {
			if msg != "" {
				if err := res.Emit(&VerifyProgress{Msg: msg}); err != nil {
					return err
				}
				fails++
			}
			i++
			if err := res.Emit(&VerifyProgress{Progress: i}); err != nil {
				return err
			}
		}

		if err := req.Context.Err(); err != nil {
			return err
		}

		if fails != 0 {
			return errors.New("verify complete, some blocks were corrupt")
		}

		return res.Emit(&VerifyProgress{Msg: "verify complete, all blocks validated."})
	},
	Type: &VerifyProgress{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, obj *VerifyProgress) error {
			if strings.Contains(obj.Msg, "was corrupt") {
				fmt.Fprintln(os.Stdout, obj.Msg)
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
		Tagline: "Apply any outstanding migrations to the repo.",
	},
	Options: []cmds.Option{
		cmds.BoolOption(repoAllowDowngradeOptionName, "Allow downgrading to a lower repo version"),
	},
	NoRemote: true,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cctx := env.(*oldcmds.Context)
		allowDowngrade, _ := req.Options[repoAllowDowngradeOptionName].(bool)

		_, err := fsrepo.Open(cctx.ConfigRoot)

		if err == nil {
			fmt.Println("Repo does not require migration.")
			return nil
		} else if err != fsrepo.ErrNeedMigration {
			return err
		}

		fmt.Println("Found outdated fs-repo, starting migration.")

		// Read Migration section of IPFS config
		configFileOpt, _ := req.Options[ConfigFileOption].(string)
		migrationCfg, err := migrations.ReadMigrationConfig(cctx.ConfigRoot, configFileOpt)
		if err != nil {
			return err
		}

		// Define function to create IPFS fetcher.  Do not supply an
		// already-constructed IPFS fetcher, because this may be expensive and
		// not needed according to migration config. Instead, supply a function
		// to construct the particular IPFS fetcher implementation used here,
		// which is called only if an IPFS fetcher is needed.
		newIpfsFetcher := func(distPath string) migrations.Fetcher {
			return ipfsfetcher.NewIpfsFetcher(distPath, 0, &cctx.ConfigRoot, configFileOpt)
		}

		// Fetch migrations from current distribution, or location from environ
		fetchDistPath := migrations.GetDistPathEnv(migrations.CurrentIpfsDist)

		// Create fetchers according to migrationCfg.DownloadSources
		fetcher, err := migrations.GetMigrationFetcher(migrationCfg.DownloadSources, fetchDistPath, newIpfsFetcher)
		if err != nil {
			return err
		}
		defer fetcher.Close()

		err = migrations.RunMigration(cctx.Context(), fetcher, fsrepo.RepoVersion, "", allowDowngrade)
		if err != nil {
			fmt.Println("The migrations of fs-repo failed:")
			fmt.Printf("  %s\n", err)
			fmt.Println("If you think this is a bug, please file an issue and include this whole log output.")
			fmt.Println("  https://github.com/ipfs/fs-repo-migrations")
			return err
		}

		fmt.Printf("Success: fs-repo has been migrated to version %d.\n", fsrepo.RepoVersion)
		return nil
	},
}
