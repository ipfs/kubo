package config

import (
	"strings"
	"testing"

	mh "github.com/multiformats/go-multihash"
)

func TestValidateImportConfig_HAMTFanout(t *testing.T) {
	tests := []struct {
		name    string
		fanout  int64
		wantErr bool
		errMsg  string
	}{
		// Valid values - powers of 2, multiples of 8, and <= 1024
		{name: "valid 8", fanout: 8, wantErr: false},
		{name: "valid 16", fanout: 16, wantErr: false},
		{name: "valid 32", fanout: 32, wantErr: false},
		{name: "valid 64", fanout: 64, wantErr: false},
		{name: "valid 128", fanout: 128, wantErr: false},
		{name: "valid 256", fanout: 256, wantErr: false},
		{name: "valid 512", fanout: 512, wantErr: false},
		{name: "valid 1024", fanout: 1024, wantErr: false},

		// Invalid values - not powers of 2
		{name: "invalid 7", fanout: 7, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid 15", fanout: 15, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid 100", fanout: 100, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid 257", fanout: 257, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid 1000", fanout: 1000, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},

		// Invalid values - powers of 2 but not multiples of 8
		{name: "invalid 1", fanout: 1, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid 2", fanout: 2, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid 4", fanout: 4, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},

		// Invalid values - exceeds 1024
		{name: "invalid 2048", fanout: 2048, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid 4096", fanout: 4096, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},

		// Invalid values - negative or zero
		{name: "invalid 0", fanout: 0, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid -8", fanout: -8, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
		{name: "invalid -256", fanout: -256, wantErr: true, errMsg: "must be a positive power of 2, multiple of 8, and not exceed 1024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Import{
				UnixFSHAMTDirectoryMaxFanout: *NewOptionalInteger(tt.fanout),
			}

			err := ValidateImportConfig(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateImportConfig() expected error for fanout=%d, got nil", tt.fanout)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImportConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImportConfig() unexpected error for fanout=%d: %v", tt.fanout, err)
				}
			}
		})
	}
}

func TestValidateImportConfig_CidVersion(t *testing.T) {
	tests := []struct {
		name    string
		cidVer  int64
		wantErr bool
		errMsg  string
	}{
		{name: "valid 0", cidVer: 0, wantErr: false},
		{name: "valid 1", cidVer: 1, wantErr: false},
		{name: "invalid 2", cidVer: 2, wantErr: true, errMsg: "must be 0 or 1"},
		{name: "invalid -1", cidVer: -1, wantErr: true, errMsg: "must be 0 or 1"},
		{name: "invalid 100", cidVer: 100, wantErr: true, errMsg: "must be 0 or 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Import{
				CidVersion: *NewOptionalInteger(tt.cidVer),
			}

			err := ValidateImportConfig(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateImportConfig() expected error for cidVer=%d, got nil", tt.cidVer)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImportConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImportConfig() unexpected error for cidVer=%d: %v", tt.cidVer, err)
				}
			}
		})
	}
}

func TestValidateImportConfig_UnixFSFileMaxLinks(t *testing.T) {
	tests := []struct {
		name     string
		maxLinks int64
		wantErr  bool
		errMsg   string
	}{
		{name: "valid 1", maxLinks: 1, wantErr: false},
		{name: "valid 174", maxLinks: 174, wantErr: false},
		{name: "valid 1000", maxLinks: 1000, wantErr: false},
		{name: "invalid 0", maxLinks: 0, wantErr: true, errMsg: "must be positive"},
		{name: "invalid -1", maxLinks: -1, wantErr: true, errMsg: "must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Import{
				UnixFSFileMaxLinks: *NewOptionalInteger(tt.maxLinks),
			}

			err := ValidateImportConfig(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateImportConfig() expected error for maxLinks=%d, got nil", tt.maxLinks)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImportConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImportConfig() unexpected error for maxLinks=%d: %v", tt.maxLinks, err)
				}
			}
		})
	}
}

func TestValidateImportConfig_UnixFSDirectoryMaxLinks(t *testing.T) {
	tests := []struct {
		name     string
		maxLinks int64
		wantErr  bool
		errMsg   string
	}{
		{name: "valid 0", maxLinks: 0, wantErr: false}, // 0 means no limit
		{name: "valid 1", maxLinks: 1, wantErr: false},
		{name: "valid 1000", maxLinks: 1000, wantErr: false},
		{name: "invalid -1", maxLinks: -1, wantErr: true, errMsg: "must be non-negative"},
		{name: "invalid -100", maxLinks: -100, wantErr: true, errMsg: "must be non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Import{
				UnixFSDirectoryMaxLinks: *NewOptionalInteger(tt.maxLinks),
			}

			err := ValidateImportConfig(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateImportConfig() expected error for maxLinks=%d, got nil", tt.maxLinks)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImportConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImportConfig() unexpected error for maxLinks=%d: %v", tt.maxLinks, err)
				}
			}
		})
	}
}

func TestValidateImportConfig_BatchMax(t *testing.T) {
	tests := []struct {
		name     string
		maxNodes int64
		maxSize  int64
		wantErr  bool
		errMsg   string
	}{
		{name: "valid nodes 1", maxNodes: 1, maxSize: -999, wantErr: false},
		{name: "valid nodes 128", maxNodes: 128, maxSize: -999, wantErr: false},
		{name: "valid size 1", maxNodes: -999, maxSize: 1, wantErr: false},
		{name: "valid size 20MB", maxNodes: -999, maxSize: 20 << 20, wantErr: false},
		{name: "invalid nodes 0", maxNodes: 0, maxSize: -999, wantErr: true, errMsg: "BatchMaxNodes must be positive"},
		{name: "invalid nodes -1", maxNodes: -1, maxSize: -999, wantErr: true, errMsg: "BatchMaxNodes must be positive"},
		{name: "invalid size 0", maxNodes: -999, maxSize: 0, wantErr: true, errMsg: "BatchMaxSize must be positive"},
		{name: "invalid size -1", maxNodes: -999, maxSize: -1, wantErr: true, errMsg: "BatchMaxSize must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Import{}
			if tt.maxNodes != -999 {
				cfg.BatchMaxNodes = *NewOptionalInteger(tt.maxNodes)
			}
			if tt.maxSize != -999 {
				cfg.BatchMaxSize = *NewOptionalInteger(tt.maxSize)
			}

			err := ValidateImportConfig(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateImportConfig() expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImportConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImportConfig() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateImportConfig_UnixFSChunker(t *testing.T) {
	tests := []struct {
		name    string
		chunker string
		wantErr bool
		errMsg  string
	}{
		{name: "valid size-262144", chunker: "size-262144", wantErr: false},
		{name: "valid size-1", chunker: "size-1", wantErr: false},
		{name: "valid size-1048576", chunker: "size-1048576", wantErr: false},
		{name: "valid rabin", chunker: "rabin-128-256-512", wantErr: false},
		{name: "valid rabin min", chunker: "rabin-16-32-64", wantErr: false},
		{name: "valid buzhash", chunker: "buzhash", wantErr: false},
		{name: "invalid size-", chunker: "size-", wantErr: true, errMsg: "invalid format"},
		{name: "invalid size-abc", chunker: "size-abc", wantErr: true, errMsg: "invalid format"},
		{name: "invalid rabin-", chunker: "rabin-", wantErr: true, errMsg: "invalid format"},
		{name: "invalid rabin-128", chunker: "rabin-128", wantErr: true, errMsg: "invalid format"},
		{name: "invalid rabin-128-256", chunker: "rabin-128-256", wantErr: true, errMsg: "invalid format"},
		{name: "invalid rabin-a-b-c", chunker: "rabin-a-b-c", wantErr: true, errMsg: "invalid format"},
		{name: "invalid unknown", chunker: "unknown", wantErr: true, errMsg: "invalid format"},
		{name: "invalid empty", chunker: "", wantErr: true, errMsg: "invalid format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Import{
				UnixFSChunker: *NewOptionalString(tt.chunker),
			}

			err := ValidateImportConfig(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateImportConfig() expected error for chunker=%s, got nil", tt.chunker)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImportConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImportConfig() unexpected error for chunker=%s: %v", tt.chunker, err)
				}
			}
		})
	}
}

func TestValidateImportConfig_HashFunction(t *testing.T) {
	tests := []struct {
		name     string
		hashFunc string
		wantErr  bool
		errMsg   string
	}{
		{name: "valid sha2-256", hashFunc: "sha2-256", wantErr: false},
		{name: "valid sha2-512", hashFunc: "sha2-512", wantErr: false},
		{name: "valid sha3-256", hashFunc: "sha3-256", wantErr: false},
		{name: "valid blake2b-256", hashFunc: "blake2b-256", wantErr: false},
		{name: "valid blake3", hashFunc: "blake3", wantErr: false},
		{name: "invalid unknown", hashFunc: "unknown-hash", wantErr: true, errMsg: "unrecognized"},
		{name: "invalid empty", hashFunc: "", wantErr: true, errMsg: "unrecognized"},
	}

	// Check for hashes that exist but are not allowed
	// MD5 should exist but not be allowed
	if code, ok := mh.Names["md5"]; ok {
		tests = append(tests, struct {
			name     string
			hashFunc string
			wantErr  bool
			errMsg   string
		}{name: "md5 not allowed", hashFunc: "md5", wantErr: true, errMsg: "not allowed"})
		_ = code // use the variable
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Import{
				HashFunction: *NewOptionalString(tt.hashFunc),
			}

			err := ValidateImportConfig(cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateImportConfig() expected error for hashFunc=%s, got nil", tt.hashFunc)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImportConfig() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImportConfig() unexpected error for hashFunc=%s: %v", tt.hashFunc, err)
				}
			}
		})
	}
}

func TestValidateImportConfig_DefaultValue(t *testing.T) {
	// Test that default (unset) value doesn't trigger validation
	cfg := &Import{}

	err := ValidateImportConfig(cfg)
	if err != nil {
		t.Errorf("ValidateImportConfig() unexpected error for default config: %v", err)
	}
}

func TestIsValidChunker(t *testing.T) {
	tests := []struct {
		chunker string
		want    bool
	}{
		{"buzhash", true},
		{"size-262144", true},
		{"size-1", true},
		{"size-0", false}, // 0 is not valid - must be positive
		{"size-9999999", true},
		{"rabin-128-256-512", true},
		{"rabin-16-32-64", true},
		{"rabin-1-2-3", true},
		{"rabin-512-256-128", false}, // Invalid ordering: min > avg > max
		{"rabin-256-128-512", false}, // Invalid ordering: min > avg
		{"rabin-128-512-256", false}, // Invalid ordering: avg > max

		{"", false},
		{"size-", false},
		{"size-abc", false},
		{"size--1", false},
		{"rabin-", false},
		{"rabin-128", false},
		{"rabin-128-256", false},
		{"rabin-128-256-512-1024", false},
		{"rabin-a-b-c", false},
		{"unknown", false},
		{"buzzhash", false}, // typo
	}

	for _, tt := range tests {
		t.Run(tt.chunker, func(t *testing.T) {
			if got := isValidChunker(tt.chunker); got != tt.want {
				t.Errorf("isValidChunker(%q) = %v, want %v", tt.chunker, got, tt.want)
			}
		})
	}
}

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int64
		want bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{5, false},
		{6, false},
		{7, false},
		{8, true},
		{16, true},
		{32, true},
		{64, true},
		{100, false},
		{128, true},
		{256, true},
		{512, true},
		{1024, true},
		{2048, true},
		{-1, false},
		{-8, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := isPowerOfTwo(tt.n); got != tt.want {
				t.Errorf("isPowerOfTwo(%d) = %v, want %v", tt.n, got, tt.want)
			}
		})
	}
}
