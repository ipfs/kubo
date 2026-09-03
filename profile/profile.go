package profile

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"time"

	"github.com/ipfs/go-log/v2"
	version "github.com/ipfs/kubo"
)

const (
	CollectorGoroutinesStack = "goroutines-stack"
	CollectorGoroutinesPprof = "goroutines-pprof"
	CollectorGoroutineLeak   = "goroutine-leak"
	CollectorVersion         = "version"
	CollectorHeap            = "heap"
	CollectorAllocs          = "allocs"
	CollectorBin             = "bin"
	CollectorCPU             = "cpu"
	CollectorMutex           = "mutex"
	CollectorBlock           = "block"
	CollectorTrace           = "trace"
)

var (
	logger = log.Logger("profile")
	goos   = runtime.GOOS
)

type collector struct {
	outputFile   string
	isExecutable bool
	collectFunc  func(ctx context.Context, opts Options, writer io.Writer) error
	enabledFunc  func(opts Options) bool
	// availableFunc, when set, reports whether this collector can run in the
	// current build. It is checked synchronously before any collector starts,
	// so an unavailable collector fails the whole request up front instead of
	// aborting mid-run with a partially written archive.
	availableFunc func() error
}

func (p *collector) outputFileName() string {
	fName := p.outputFile
	if p.isExecutable {
		if goos == "windows" {
			fName += ".exe"
		}
	}
	return fName
}

var collectors = map[string]collector{
	CollectorGoroutinesStack: {
		outputFile:  "goroutines.stacks",
		collectFunc: goroutineStacksText,
		enabledFunc: func(opts Options) bool { return true },
	},
	CollectorGoroutinesPprof: {
		outputFile:  "goroutines.pprof",
		collectFunc: goroutineStacksProto,
		enabledFunc: func(opts Options) bool { return true },
	},
	CollectorGoroutineLeak: {
		// Deliberately not part of the CLI default collector set: collecting
		// this profile forces a goroutine-leak-detection GC cycle, and the
		// profile only exists when the binary was built with
		// GOEXPERIMENT=goroutineleakprofile. Requesting it on a build without
		// the experiment returns an error instead of silently skipping.
		outputFile:    "goroutineleak.pprof",
		collectFunc:   goroutineLeakProfile,
		enabledFunc:   func(opts Options) bool { return true },
		availableFunc: goroutineLeakAvailable,
	},
	CollectorVersion: {
		outputFile:  "version.json",
		collectFunc: versionInfo,
		enabledFunc: func(opts Options) bool { return true },
	},
	CollectorHeap: {
		outputFile:  "heap.pprof",
		collectFunc: heapProfile,
		enabledFunc: func(opts Options) bool { return true },
	},
	CollectorAllocs: {
		outputFile:  "allocs.pprof",
		collectFunc: allocsProfile,
		enabledFunc: func(opts Options) bool { return true },
	},
	CollectorBin: {
		outputFile:   "ipfs",
		isExecutable: true,
		collectFunc:  binary,
		enabledFunc:  func(opts Options) bool { return true },
	},
	CollectorCPU: {
		outputFile:  "cpu.pprof",
		collectFunc: profileCPU,
		enabledFunc: func(opts Options) bool { return opts.ProfileDuration > 0 },
	},
	CollectorMutex: {
		outputFile:  "mutex.pprof",
		collectFunc: mutexProfile,
		enabledFunc: func(opts Options) bool { return opts.ProfileDuration > 0 && opts.MutexProfileFraction > 0 },
	},
	CollectorBlock: {
		outputFile:  "block.pprof",
		collectFunc: blockProfile,
		enabledFunc: func(opts Options) bool { return opts.ProfileDuration > 0 && opts.BlockProfileRate > 0 },
	},
	CollectorTrace: {
		outputFile:  "trace",
		collectFunc: captureTrace,
		enabledFunc: func(opts Options) bool { return opts.ProfileDuration > 0 },
	},
}

type Options struct {
	Collectors           []string
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

// profiler runs the collectors concurrently and writes the results to the zip archive.
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

	collectorsToRun := make([]collector, len(p.opts.Collectors))
	for i, name := range p.opts.Collectors {
		c, ok := collectors[name]
		if !ok {
			return fmt.Errorf("unknown collector '%s'", name)
		}
		collectorsToRun[i] = c
	}

	// All names are valid; now confirm every requested collector can run in
	// this build, before any collection starts. Keeping this a separate pass
	// makes validation errors deterministic across build variants.
	for _, c := range collectorsToRun {
		if c.availableFunc != nil {
			if err := c.availableFunc(); err != nil {
				return err
			}
		}
	}

	results := make(chan profileResult, len(p.opts.Collectors))
	wg := sync.WaitGroup{}
	for _, c := range collectorsToRun {
		if !c.enabledFunc(p.opts) {
			continue
		}

		fName := c.outputFileName()

		wg.Add(1)
		go func(c collector) {
			defer wg.Done()
			logger.Infow("collecting profile", "File", fName)
			defer logger.Infow("profile done", "File", fName)
			b := bytes.Buffer{}
			err := c.collectFunc(ctx, p.opts, &b)
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
		}(c)
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

// goroutineLeakAvailable reports whether the runtime registered the
// goroutineleak profile. The profile is only present when the binary was
// built with GOEXPERIMENT=goroutineleakprofile (a Go 1.26 experiment,
// expected to be enabled by default in Go 1.27).
func goroutineLeakAvailable() error {
	if pprof.Lookup("goroutineleak") == nil {
		return errors.New("goroutineleak profile is not available in this build (requires GOEXPERIMENT=goroutineleakprofile)")
	}
	return nil
}

// goroutineLeakProfile writes the goroutineleak profile registered by the
// runtime. Collecting it triggers a goroutine-leak-detection GC cycle.
func goroutineLeakProfile(ctx context.Context, _ Options, w io.Writer) error {
	if err := goroutineLeakAvailable(); err != nil {
		return err
	}
	return pprof.Lookup("goroutineleak").WriteTo(w, 0)
}

func heapProfile(ctx context.Context, _ Options, w io.Writer) error {
	return pprof.Lookup("heap").WriteTo(w, 0)
}

func allocsProfile(ctx context.Context, _ Options, w io.Writer) error {
	return pprof.Lookup("allocs").WriteTo(w, 0)
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

func captureTrace(ctx context.Context, opts Options, w io.Writer) error {
	err := trace.Start(w)
	if err != nil {
		return err
	}
	defer trace.Stop()
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
