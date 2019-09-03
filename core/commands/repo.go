package commands

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"

	cid "github.com/ipfs/go-cid"
	dsq "github.com/ipfs/go-datastore/query"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cmds "github.com/ipfs/go-ipfs-cmds"
	peer "github.com/libp2p/go-libp2p-core/peer"
	base32 "github.com/whyrusleeping/base32"
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
		"fsck":    repoFsckCmd,
		"version": repoVersionCmd,
		"verify":  repoVerifyCmd,
	},
}

// GcResult is the result returned by "repo gc" command.
type GcResult struct {
	Key   cid.Cid
	Error string `json:",omitempty"`
}

const (
	repoStreamErrorsOptionName = "stream-errors"
	repoQuietOptionName        = "quiet"
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
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

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
		cmds.BoolOption(repoSizeOnlyOptionName, "Only report RepoSize and StorageMax."),
		cmds.BoolOption(repoHumanOptionName, "Print sizes in human readable format (e.g., 1K 234M 2G)"),
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

var repoListCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List values stored in the local repo.",
		ShortDescription: `
'ipfs repo list' is a plumbing command used to list values in the local repo.

This command and its subcommands are unstable and may change or go away at any time. Do not rely on them in production code.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"providers": repoListProvidersCmd,
	},
}

type providerRecord struct {
	Peer    string
	Cid     string
	Expires time.Time
}

func parseProvider(record dsq.Result) (time.Time, cid.Cid, peer.ID, error) {
	key := record.Key
	value := record.Value

	const prefix string = "/providers/"
	if !strings.HasPrefix(key, prefix) {
		return time.Time{}, cid.Undef, "", fmt.Errorf("not a provider record: %q", key)
	}
	components := strings.Split(key[len(prefix):], "/")
	if len(components) != 2 {
		return time.Time{}, cid.Undef, "", fmt.Errorf("provider record key invalid: %q", key)
	}
	bincid, err := base32.RawStdEncoding.DecodeString(components[0])
	if err != nil {
		return time.Time{}, cid.Undef, "", fmt.Errorf("provider record key has mis-encoded CID: %q", key)
	}
	c, err := cid.Cast(bincid)
	if err != nil {
		return time.Time{}, cid.Undef, "", fmt.Errorf("record key has an invalid CID: %s", key)
	}

	pid, err := base32.RawStdEncoding.DecodeString(components[1])
	if err != nil {
		return time.Time{}, cid.Undef, "", fmt.Errorf("provider record key has a mis-encoded peer ID: %s", key)
	}

	nsec, n := binary.Varint(value)
	if n <= 0 {
		return time.Time{}, cid.Undef, "", fmt.Errorf("failed to parse provider record expiration time: %q", key)
	}

	return time.Unix(0, nsec), c, peer.ID(pid), nil
}

var repoListProvidersCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List providers stored in the local datastore.",
		ShortDescription: `
'ipfs repo list providers' lists providers in the local datastore or all known providers for a specific key.

Status: Unstable. This command is for debugging and may change or be removed at any time.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", false, true, "Limit providers to the specific CIDs").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		err := req.ParseBodyArgs()
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetLowLevelCidEncoder(req)
		if err != nil {
			return err
		}

		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}
		ds := n.Repo.Datastore()

		doQuery := func(q dsq.Query) error {
			provs, err := ds.Query(q)
			if err != nil {
				return err
			}
			defer provs.Close()

			for req.Context.Err() == nil {
				rec, ok := provs.NextSync()
				if !ok {
					return nil
				}
				if rec.Error != nil {
					return cmds.Errorf(cmds.ErrImplementation, "datastore error: %s", rec.Error)
				}

				t, c, p, err := parseProvider(rec)
				if err != nil {
					return cmds.Errorf(cmds.ErrImplementation, "%s", err)
				}

				err = res.Emit(&providerRecord{
					Peer:    p.Pretty(),
					Cid:     enc.Encode(c),
					Expires: t,
				})
				if err != nil {
					return err
				}
			}
			return req.Context.Err()
		}

		if len(req.Arguments) == 0 {
			return doQuery(dsq.Query{Prefix: "/providers/"})
		} else {
			for _, arg := range req.Arguments {
				c, err := cid.Decode(arg)
				if err != nil {
					return cmds.Errorf(cmds.ErrClient, "invalid cid: %s", err)
				}
				encodedCid := base32.RawStdEncoding.EncodeToString(c.Bytes())
				err = doQuery(dsq.Query{Prefix: "/providers/" + encodedCid + "/"})
				if err != nil {
					return err
				}
			}
		}
		return nil
	},
	Type: providerRecord{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, prov *providerRecord) error {
			_, err := fmt.Fprintf(w, "%s\tprovided by\t%s\tuntil%s\n", prov.Cid, prov.Peer, prov.Expires)
			return err
		}),
	},
}

var repoFsckCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove repo lockfiles.",
		ShortDescription: `
'ipfs repo fsck' is now a no-op.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		return cmds.EmitOnce(res, &MessageOutput{"`ipfs repo fsck` is deprecated and does nothing.\n"})
	},
	Type: MessageOutput{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *MessageOutput) error {
			fmt.Fprintf(w, out.Message)
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
		_, err := bs.Get(k)
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
