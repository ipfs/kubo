package profile

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goroutineLeakEnabled reports whether this test binary was built with
// GOEXPERIMENT=goroutineleakprofile, which is when the runtime registers the
// goroutineleak profile.
var goroutineLeakEnabled = goroutineLeakAvailable() == nil

func TestProfiler(t *testing.T) {
	allCollectors := []string{
		CollectorGoroutinesStack,
		CollectorGoroutinesPprof,
		CollectorVersion,
		CollectorHeap,
		CollectorAllocs,
		CollectorBin,
		CollectorCPU,
		CollectorMutex,
		CollectorBlock,
		CollectorTrace,
	}
	if goroutineLeakEnabled {
		allCollectors = append(allCollectors, CollectorGoroutineLeak)
	}

	cases := []struct {
		name string
		opts Options
		goos string

		expectFiles []string
	}{
		{
			name: "happy case",
			opts: Options{
				Collectors:           allCollectors,
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 4,
				BlockProfileRate:     50 * time.Nanosecond,
			},
			expectFiles: withLeakProfile([]string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"allocs.pprof",
				"ipfs",
				"cpu.pprof",
				"mutex.pprof",
				"block.pprof",
				"trace",
			}),
		},
		{
			name: "windows",
			opts: Options{
				Collectors:           allCollectors,
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 4,
				BlockProfileRate:     50 * time.Nanosecond,
			},
			goos: "windows",
			expectFiles: withLeakProfile([]string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"allocs.pprof",
				"ipfs.exe",
				"cpu.pprof",
				"mutex.pprof",
				"block.pprof",
				"trace",
			}),
		},
		{
			name: "sampling profiling disabled",
			opts: Options{
				Collectors:           allCollectors,
				MutexProfileFraction: 4,
				BlockProfileRate:     50 * time.Nanosecond,
			},
			expectFiles: withLeakProfile([]string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"allocs.pprof",
				"ipfs",
			}),
		},
		{
			name: "Mutex profiling disabled",
			opts: Options{
				Collectors:       allCollectors,
				ProfileDuration:  1 * time.Millisecond,
				BlockProfileRate: 50 * time.Nanosecond,
			},
			expectFiles: withLeakProfile([]string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"allocs.pprof",
				"ipfs",
				"cpu.pprof",
				"block.pprof",
				"trace",
			}),
		},
		{
			name: "block profiling disabled",
			opts: Options{
				Collectors:           allCollectors,
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 4,
				BlockProfileRate:     0,
			},
			expectFiles: withLeakProfile([]string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"allocs.pprof",
				"ipfs",
				"cpu.pprof",
				"mutex.pprof",
				"trace",
			}),
		},
		{
			name: "single collector",
			opts: Options{
				Collectors:           []string{CollectorVersion},
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 4,
				BlockProfileRate:     0,
			},
			expectFiles: []string{
				"version.json",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.goos != "" {
				oldGOOS := goos
				goos = c.goos
				defer func() { goos = oldGOOS }()
			}

			buf := &bytes.Buffer{}
			archive := zip.NewWriter(buf)
			err := WriteProfiles(context.Background(), archive, c.opts)
			require.NoError(t, err)

			err = archive.Close()
			require.NoError(t, err)

			zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			require.NoError(t, err)

			for _, f := range zr.File {
				logger.Info("zip file: ", f.Name)
			}

			require.Equal(t, len(c.expectFiles), len(zr.File))

			for _, expectedFile := range c.expectFiles {
				func() {
					f, err := zr.Open(expectedFile)
					require.NoError(t, err)
					defer f.Close()
					fi, err := f.Stat()
					require.NoError(t, err)
					assert.NotZero(t, fi.Size())
				}()
			}
		})
	}
}

// withLeakProfile appends the goroutine leak profile output file when the
// runtime registered the goroutineleak profile, so the same expectations work
// with and without GOEXPERIMENT=goroutineleakprofile.
func withLeakProfile(files []string) []string {
	if goroutineLeakEnabled {
		return append(files, "goroutineleak.pprof")
	}
	return files
}

func TestUnknownCollectorBeatsAvailability(t *testing.T) {
	// The unknown-name error must win regardless of whether the goroutineleak
	// profile is available, keeping validation deterministic across builds.
	err := WriteProfiles(t.Context(), zip.NewWriter(&bytes.Buffer{}), Options{
		Collectors: []string{CollectorGoroutineLeak, "bogus"},
	})
	require.ErrorContains(t, err, "unknown collector 'bogus'")
}

func TestGoroutineLeakProfile(t *testing.T) {
	// Request the collector together with another one: on builds without the
	// experiment the whole request must fail up front with a clear error and
	// no partially written archive, not silently drop the requested profile.
	buf := &bytes.Buffer{}
	archive := zip.NewWriter(buf)
	err := WriteProfiles(t.Context(), archive, Options{
		Collectors: []string{CollectorVersion, CollectorGoroutineLeak},
	})

	if !goroutineLeakEnabled {
		require.ErrorContains(t, err, "goroutineleak profile is not available")
		require.NoError(t, archive.Close())
		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)
		assert.Empty(t, zr.File, "no collector output should be written when an unavailable collector is requested")
		return
	}

	require.NoError(t, err)
	require.NoError(t, archive.Close())

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, zr.File, 2)

	f, err := zr.Open("goroutineleak.pprof")
	require.NoError(t, err)
	defer f.Close()
	data, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Greater(t, len(data), 2)
	// The profile is written in gzip-compressed protobuf format.
	assert.Equal(t, []byte{0x1f, 0x8b}, data[:2])
}
