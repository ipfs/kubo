package mount

import (
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// TestDefaultBlksizeAnchor pins DefaultBlksize to 1 MiB so a silent
// refactor cannot drift the value FUSE mounts advertise to tools.
// See stat.go for the rationale (CID-deterministic profile alignment).
func TestDefaultBlksizeAnchor(t *testing.T) {
	if DefaultBlksize != 1024*1024 {
		t.Fatalf("DefaultBlksize = %d, want 1 MiB (%d)", DefaultBlksize, 1024*1024)
	}
}

func TestBlksizeFromChunker(t *testing.T) {
	tests := []struct {
		name    string
		chunker string
		want    uint32
	}{
		// Kubo defaults and common user choices.
		{"default chunker", "size-262144", 262144},
		{"CID-deterministic profile", "size-1048576", 1024 * 1024},
		{"small custom", "size-65536", 65536},

		// Non-size chunkers: fall back to DefaultBlksize because no
		// single preferred I/O size describes their variable output.
		{"rabin", "rabin", DefaultBlksize},
		{"rabin with params", "rabin-512-1024-2048", DefaultBlksize},
		{"buzhash", "buzhash", DefaultBlksize},

		// Defensive: malformed or empty input must not panic or return
		// a surprising value.
		{"empty", "", DefaultBlksize},
		{"size prefix only", "size-", DefaultBlksize},
		{"non-numeric size", "size-abc", DefaultBlksize},
		{"zero size", "size-0", DefaultBlksize},

		// Clamp: values above fuse.MAX_KERNEL_WRITE (the largest single FUSE
		// request the kernel delivers) are capped so tools can't be
		// tricked into allocating buffers the kernel will just split.
		{"above cap clamped", "size-2097152", fuse.MAX_KERNEL_WRITE},
		{"16 MiB clamped", "size-16777216", fuse.MAX_KERNEL_WRITE},
		{"uint32 max clamped", "size-4294967295", fuse.MAX_KERNEL_WRITE},
		{"beyond uint32 clamped", "size-99999999999", fuse.MAX_KERNEL_WRITE},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BlksizeFromChunker(tc.chunker); got != tc.want {
				t.Fatalf("BlksizeFromChunker(%q) = %d, want %d", tc.chunker, got, tc.want)
			}
		})
	}
}
