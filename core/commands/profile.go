package commands

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/e"
	"github.com/ipfs/kubo/profile"
)

// time format that works in filenames on windows.
var timeFormat = strings.ReplaceAll(time.RFC3339, ":", "_")

type profileResult struct {
	File string
}

const (
	collectorsOptionName       = "collectors"
	profileTimeOption          = "profile-time"
	mutexProfileFractionOption = "mutex-profile-fraction"
	blockProfileRateOption     = "block-profile-rate"
)

var sysProfileCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Collect a performance profile for debugging.",
		ShortDescription: `
Collects profiles from a running go-ipfs daemon into a single zip file.
To aid in debugging, this command also attempts to include a copy of
the running go-ipfs binary.
`,
		LongDescription: `
Collects profiles from a running go-ipfs daemon into a single zipfile.
To aid in debugging, this command also attempts to include a copy of
the running go-ipfs binary.

Profiles can be examined using 'go tool pprof', some tips can be found at
https://github.com/ipfs/kubo/blob/master/docs/debug-guide.md.

Privacy Notice:

The output file includes:

- A list of running goroutines.
- A CPU profile.
- A heap inuse profile.
- A heap allocation profile.
- A mutex profile.
- A block profile.
- Your copy of go-ipfs.
- The output of 'ipfs version --all'.

It does not include:

- Any of your IPFS data or metadata.
- Your config or private key.
- Your IP address.
- The contents of your computer's memory, filesystem, etc.

However, it could reveal:

- Your build path, if you built go-ipfs yourself.
- If and how a command/feature is being used (inferred from running functions).
- Memory offsets of various data structures.
- Any modifications you've made to go-ipfs.
`,
	},
	NoLocal: true,
	Options: []cmds.Option{
		cmds.StringOption(outputOptionName, "o", "The path where the output .zip should be stored. Default: ./ipfs-profile-[timestamp].zip"),
		cmds.DelimitedStringsOption(",", collectorsOptionName, "The list of collectors to use for collecting diagnostic data.").
			WithDefault([]string{
				profile.CollectorGoroutinesStack,
				profile.CollectorGoroutinesPprof,
				profile.CollectorVersion,
				profile.CollectorHeap,
				profile.CollectorAllocs,
				profile.CollectorBin,
				profile.CollectorCPU,
				profile.CollectorMutex,
				profile.CollectorBlock,
				profile.CollectorTrace,
			}),
		cmds.StringOption(profileTimeOption, "The amount of time spent profiling. If this is set to 0, then sampling profiles are skipped.").WithDefault("30s"),
		cmds.IntOption(mutexProfileFractionOption, "The fraction 1/n of mutex contention events that are reported in the mutex profile.").WithDefault(4),
		cmds.StringOption(blockProfileRateOption, "The duration to wait between sampling goroutine-blocking events for the blocking profile.").WithDefault("1ms"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		collectors := req.Options[collectorsOptionName].([]string)

		profileTimeStr, _ := req.Options[profileTimeOption].(string)
		profileTime, err := time.ParseDuration(profileTimeStr)
		if err != nil {
			return fmt.Errorf("failed to parse profile duration %q: %w", profileTimeStr, err)
		}

		blockProfileRateStr, _ := req.Options[blockProfileRateOption].(string)
		blockProfileRate, err := time.ParseDuration(blockProfileRateStr)
		if err != nil {
			return fmt.Errorf("failed to parse block profile rate %q: %w", blockProfileRateStr, err)
		}

		mutexProfileFraction, _ := req.Options[mutexProfileFractionOption].(int)

		r, w := io.Pipe()

		go func() {
			archive := zip.NewWriter(w)
			err = profile.WriteProfiles(req.Context, archive, profile.Options{
				Collectors:           collectors,
				ProfileDuration:      profileTime,
				MutexProfileFraction: mutexProfileFraction,
				BlockProfileRate:     blockProfileRate,
			})
			archive.Close()
			_ = w.CloseWithError(err)
		}()
		return res.Emit(r)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			v, err := res.Next()
			if err != nil {
				return err
			}

			outReader, ok := v.(io.Reader)
			if !ok {
				return e.New(e.TypeErr(outReader, v))
			}

			outPath, _ := res.Request().Options[outputOptionName].(string)
			if outPath == "" {
				outPath = "ipfs-profile-" + time.Now().Format(timeFormat) + ".zip"
			}
			fi, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer fi.Close()

			_, err = io.Copy(fi, outReader)
			if err != nil {
				return err
			}
			return re.Emit(&profileResult{File: outPath})
		},
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *profileResult) error {
			fmt.Fprintf(w, "Wrote profiles to: %s\n", out.File)
			return nil
		}),
	},
}
