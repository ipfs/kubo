package mount

import "testing"

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

		// Clamp: values above MaxBlksize are capped so tools can't be
		// tricked into allocating multi-GiB buffers per read.
		{"at cap", "size-16777216", MaxBlksize},
		{"above cap clamped", "size-33554432", MaxBlksize},
		{"uint32 max clamped", "size-4294967295", MaxBlksize},
		{"beyond uint32 clamped", "size-99999999999", MaxBlksize},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BlksizeFromChunker(tc.chunker); got != tc.want {
				t.Fatalf("BlksizeFromChunker(%q) = %d, want %d", tc.chunker, got, tc.want)
			}
		})
	}
}
