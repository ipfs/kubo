package profile

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfiler(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		goos string

		expectFiles []string
	}{
		{
			name: "happy case",
			opts: Options{
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 4,
				BlockProfileRate:     50 * time.Nanosecond,
			},
			expectFiles: []string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"ipfs",
				"cpu.pprof",
				"mutex.pprof",
				"block.pprof",
			},
		},
		{
			name: "windows",
			opts: Options{
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 4,
				BlockProfileRate:     50 * time.Nanosecond,
			},
			goos: "windows",
			expectFiles: []string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"ipfs.exe",
				"cpu.pprof",
				"mutex.pprof",
				"block.pprof",
			},
		},
		{
			name: "sampling profiling disabled",
			opts: Options{
				MutexProfileFraction: 4,
				BlockProfileRate:     50 * time.Nanosecond,
			},
			expectFiles: []string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"ipfs",
			},
		},
		{
			name: "Mutex profiling disabled",
			opts: Options{
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 0,
				BlockProfileRate:     50 * time.Nanosecond,
			},
			expectFiles: []string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"ipfs",
				"cpu.pprof",
				"block.pprof",
			},
		},
		{
			name: "block profiling disabled",
			opts: Options{
				ProfileDuration:      1 * time.Millisecond,
				MutexProfileFraction: 4,
				BlockProfileRate:     0,
			},
			expectFiles: []string{
				"goroutines.stacks",
				"goroutines.pprof",
				"version.json",
				"heap.pprof",
				"ipfs",
				"cpu.pprof",
				"mutex.pprof",
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
