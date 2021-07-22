package commands

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/e"
)

// time format that works in filenames on windows.
var timeFormat = strings.ReplaceAll(time.RFC3339, ":", "_")

type profileResult struct {
	File string
}

const cpuProfileTimeOption = "cpu-profile-time"

var sysProfileCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Collect a performance profile for debugging.",
		ShortDescription: `
Collects cpu, heap, and goroutine profiles from a running go-ipfs daemon
into a single zip file. To aid in debugging, this command also attempts to
include a copy of the running go-ipfs binary.
`,
		LongDescription: `
Collects cpu, heap, and goroutine profiles from a running go-ipfs daemon
into a single zipfile. To aid in debugging, this command also attempts to
include a copy of the running go-ipfs binary.

Profile's can be examined using 'go tool pprof', some tips can be found at
https://github.com/ipfs/go-ipfs/blob/master/docs/debug-guide.md.

Privacy Notice:

The output file includes:

- A list of running goroutines.
- A CPU profile.
- A heap profile.
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
		cmds.StringOption(outputOptionName, "o", "The path where the output should be stored."),
		cmds.StringOption(cpuProfileTimeOption, "The amount of time spent profiling CPU usage.").WithDefault("30s"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cpuProfileTimeStr, _ := req.Options[cpuProfileTimeOption].(string)
		cpuProfileTime, err := time.ParseDuration(cpuProfileTimeStr)
		if err != nil {
			return fmt.Errorf("failed to parse CPU profile duration %q: %w", cpuProfileTimeStr, err)
		}

		r, w := io.Pipe()
		go func() {
			_ = w.CloseWithError(writeProfiles(req.Context, cpuProfileTime, w))
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

func writeProfiles(ctx context.Context, cpuProfileTime time.Duration, w io.Writer) error {
	archive := zip.NewWriter(w)

	// Take some profiles.
	type profile struct {
		name  string
		file  string
		debug int
	}

	profiles := []profile{{
		name:  "goroutine",
		file:  "goroutines.stacks",
		debug: 2,
	}, {
		name: "goroutine",
		file: "goroutines.pprof",
	}, {
		name: "heap",
		file: "heap.pprof",
	}}

	for _, profile := range profiles {
		prof := pprof.Lookup(profile.name)
		out, err := archive.Create(profile.file)
		if err != nil {
			return err
		}
		err = prof.WriteTo(out, profile.debug)
		if err != nil {
			return err
		}
	}

	// Take a CPU profile.
	if cpuProfileTime != 0 {
		out, err := archive.Create("cpu.pprof")
		if err != nil {
			return err
		}

		err = writeCPUProfile(ctx, cpuProfileTime, out)
		if err != nil {
			return err
		}
	}

	// Collect version info
	// I'd use diag sysinfo, but that includes some more sensitive information
	// (GOPATH, etc.).
	{
		out, err := archive.Create("version.json")
		if err != nil {
			return err
		}

		err = json.NewEncoder(out).Encode(getVersionInfo())
		if err != nil {
			return err
		}
	}

	// Collect binary
	if fi, err := openIPFSBinary(); err == nil {
		fname := "ipfs"
		if runtime.GOOS == "windows" {
			fname += ".exe"
		}

		out, err := archive.Create(fname)
		if err != nil {
			return err
		}

		_, err = io.Copy(out, fi)
		_ = fi.Close()
		if err != nil {
			return err
		}
	}
	return archive.Close()
}

func writeCPUProfile(ctx context.Context, d time.Duration, w io.Writer) error {
	if err := pprof.StartCPUProfile(w); err != nil {
		return err
	}
	defer pprof.StopCPUProfile()

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func openIPFSBinary() (*os.File, error) {
	if runtime.GOOS == "linux" {
		pid := os.Getpid()
		fi, err := os.Open(fmt.Sprintf("/proc/%d/exe", pid))
		if err == nil {
			return fi, nil
		}
	}
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
