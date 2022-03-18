package profile

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	version "github.com/ipfs/go-ipfs"
	"github.com/ipfs/go-log"
)

var (
	logger = log.Logger("profile")
	goos   = runtime.GOOS
)

type profile struct {
	outputFile   string
	isExecutable bool
	profileFunc  func(ctx context.Context, opts Options, writer io.Writer) error
	enabledFunc  func(opts Options) bool
}

func (p *profile) fileName() string {
	fName := p.outputFile
	if p.isExecutable {
		if goos == "windows" {
			fName += ".exe"
		}
	}
	return fName
}

var profiles = []profile{
	{
		outputFile:  "goroutines.stacks",
		profileFunc: goroutineStacksText,
		enabledFunc: func(opts Options) bool { return true },
	},
	{
		outputFile:  "goroutines.pprof",
		profileFunc: goroutineStacksProto,
		enabledFunc: func(opts Options) bool { return true },
	},
	{
		outputFile:  "version.json",
		profileFunc: versionInfo,
		enabledFunc: func(opts Options) bool { return true },
	},
	{
		outputFile:  "heap.pprof",
		profileFunc: heapProfile,
		enabledFunc: func(opts Options) bool { return true },
	},
	{
		outputFile:   "ipfs",
		isExecutable: true,
		profileFunc:  binary,
		enabledFunc:  func(opts Options) bool { return true },
	},
	{
		outputFile:  "cpu.pprof",
		profileFunc: profileCPU,
		enabledFunc: func(opts Options) bool { return opts.ProfileDuration > 0 },
	},
	{
		outputFile:  "mutex.pprof",
		profileFunc: mutexProfile,
		enabledFunc: func(opts Options) bool { return opts.ProfileDuration > 0 && opts.MutexProfileFraction > 0 },
	},
	{
		outputFile:  "block.pprof",
		profileFunc: blockProfile,
		enabledFunc: func(opts Options) bool { return opts.ProfileDuration > 0 && opts.BlockProfileRate > 0 },
	},
}

type Options struct {
	ProfileDuration      time.Duration
	MutexProfileFraction int
	BlockProfileRate     time.Duration
}

func WriteProfiles(ctx context.Context, archive *zip.Writer, opts Options) error {
	p := profiler{
		archive: archive,
		opts:    opts,
	}
	return p.runProfile(ctx)
}

// profiler runs the profiles concurrently and writes the results to the zip archive.
type profiler struct {
	archive *zip.Writer
	opts    Options
}

func (p *profiler) runProfile(ctx context.Context) error {
	type profileResult struct {
		fName string
		buf   *bytes.Buffer
		err   error
	}

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	results := make(chan profileResult, len(profiles))
	wg := sync.WaitGroup{}
	for _, prof := range profiles {
		if !prof.enabledFunc(p.opts) {
			continue
		}

		fName := prof.fileName()

		wg.Add(1)
		go func(prof profile) {
			defer wg.Done()
			logger.Infow("collecting profile", "File", fName)
			defer logger.Infow("profile done", "File", fName)
			b := bytes.Buffer{}
			err := prof.profileFunc(ctx, p.opts, &b)
			if err != nil {
				select {
				case results <- profileResult{err: fmt.Errorf("generating profile data for %q: %w", fName, err)}:
				case <-ctx.Done():
					return
				}
			}
			select {
			case results <- profileResult{buf: &b, fName: fName}:
			case <-ctx.Done():
			}
		}(prof)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			return res.err
		}
		out, err := p.archive.Create(res.fName)
		if err != nil {
			return fmt.Errorf("creating output file %q: %w", res.fName, err)
		}
		_, err = io.Copy(out, res.buf)
		if err != nil {
			return fmt.Errorf("compressing result %q: %w", res.fName, err)
		}
	}

	return nil
}

func goroutineStacksText(ctx context.Context, _ Options, w io.Writer) error {
	return WriteAllGoroutineStacks(w)
}

func goroutineStacksProto(ctx context.Context, _ Options, w io.Writer) error {
	return pprof.Lookup("goroutine").WriteTo(w, 0)
}

func heapProfile(ctx context.Context, _ Options, w io.Writer) error {
	return pprof.Lookup("heap").WriteTo(w, 0)
}

func versionInfo(ctx context.Context, _ Options, w io.Writer) error {
	return json.NewEncoder(w).Encode(version.GetVersionInfo())
}

func binary(ctx context.Context, _ Options, w io.Writer) error {
	var (
		path string
		err  error
	)
	if goos == "linux" {
		pid := os.Getpid()
		path = fmt.Sprintf("/proc/%d/exe", pid)
	} else {
		path, err = os.Executable()
		if err != nil {
			return fmt.Errorf("finding binary path: %w", err)
		}
	}
	fi, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening binary %q: %w", path, err)
	}
	_, err = io.Copy(w, fi)
	_ = fi.Close()
	if err != nil {
		return fmt.Errorf("copying binary %q: %w", path, err)
	}
	return nil
}

func mutexProfile(ctx context.Context, opts Options, w io.Writer) error {
	prev := runtime.SetMutexProfileFraction(opts.MutexProfileFraction)
	defer runtime.SetMutexProfileFraction(prev)
	err := waitOrCancel(ctx, opts.ProfileDuration)
	if err != nil {
		return err
	}
	return pprof.Lookup("mutex").WriteTo(w, 2)
}

func blockProfile(ctx context.Context, opts Options, w io.Writer) error {
	runtime.SetBlockProfileRate(int(opts.BlockProfileRate.Nanoseconds()))
	defer runtime.SetBlockProfileRate(0)
	err := waitOrCancel(ctx, opts.ProfileDuration)
	if err != nil {
		return err
	}
	return pprof.Lookup("block").WriteTo(w, 2)
}

func profileCPU(ctx context.Context, opts Options, w io.Writer) error {
	err := pprof.StartCPUProfile(w)
	if err != nil {
		return err
	}
	defer pprof.StopCPUProfile()
	return waitOrCancel(ctx, opts.ProfileDuration)
}

func waitOrCancel(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
