package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProvideStrategy(t *testing.T) {
	tests := []struct {
		input  string
		expect ProvideStrategy
	}{
		{"all", ProvideStrategyAll},
		{"pinned", ProvideStrategyPinned},
		{"mfs", ProvideStrategyMFS},
		{"pinned+mfs", ProvideStrategyPinned | ProvideStrategyMFS},
		{"invalid", 0},
		{"all+invalid", ProvideStrategyAll},
		{"", ProvideStrategyAll},
		{"flat", ProvideStrategyAll}, // deprecated, maps to "all"
		{"flat+all", ProvideStrategyAll},
	}

	for _, tt := range tests {
		result := ParseProvideStrategy(tt.input)
		if result != tt.expect {
			t.Errorf("ParseProvideStrategy(%q) = %d, want %d", tt.input, result, tt.expect)
		}
	}
}

func TestValidateProvideConfig_Interval(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		wantErr  bool
		errMsg   string
	}{
		{"valid default (22h)", 22 * time.Hour, false, ""},
		{"valid max (48h)", 48 * time.Hour, false, ""},
		{"valid small (1h)", 1 * time.Hour, false, ""},
		{"valid zero (disabled)", 0, false, ""},
		{"invalid over limit (49h)", 49 * time.Hour, true, "must be less than or equal to DHT provider record validity"},
		{"invalid over limit (72h)", 72 * time.Hour, true, "must be less than or equal to DHT provider record validity"},
		{"invalid negative", -1 * time.Hour, true, "must be non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Provide{
				DHT: ProvideDHT{
					Interval: NewOptionalDuration(tt.interval),
				},
			}

			err := ValidateProvideConfig(cfg)

			if tt.wantErr {
				require.Error(t, err, "expected error for interval=%v", tt.interval)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message mismatch")
				}
			} else {
				require.NoError(t, err, "unexpected error for interval=%v", tt.interval)
			}
		})
	}
}

func TestValidateProvideConfig_MaxWorkers(t *testing.T) {
	tests := []struct {
		name       string
		maxWorkers int64
		wantErr    bool
		errMsg     string
	}{
		{"valid default", 16, false, ""},
		{"valid high", 100, false, ""},
		{"valid low", 1, false, ""},
		{"invalid zero", 0, true, "must be positive"},
		{"invalid negative", -1, true, "must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Provide{
				DHT: ProvideDHT{
					MaxWorkers: NewOptionalInteger(tt.maxWorkers),
				},
			}

			err := ValidateProvideConfig(cfg)

			if tt.wantErr {
				require.Error(t, err, "expected error for maxWorkers=%d", tt.maxWorkers)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message mismatch")
				}
			} else {
				require.NoError(t, err, "unexpected error for maxWorkers=%d", tt.maxWorkers)
			}
		})
	}
}

func TestShouldProvideForStrategy(t *testing.T) {
	t.Run("all strategy always provides", func(t *testing.T) {
		// ProvideStrategyAll should return true regardless of flags
		testCases := []struct{ pinned, pinnedRoot, mfs bool }{
			{false, false, false},
			{true, true, true},
			{true, false, false},
		}

		for _, tc := range testCases {
			assert.True(t, ShouldProvideForStrategy(
				ProvideStrategyAll, tc.pinned, tc.pinnedRoot, tc.mfs))
		}
	})

	t.Run("single strategies match only their flag", func(t *testing.T) {
		tests := []struct {
			name                    string
			strategy                ProvideStrategy
			pinned, pinnedRoot, mfs bool
			want                    bool
		}{
			{"pinned: matches when pinned=true", ProvideStrategyPinned, true, false, false, true},
			{"pinned: ignores other flags", ProvideStrategyPinned, false, true, true, false},

			{"roots: matches when pinnedRoot=true", ProvideStrategyRoots, false, true, false, true},
			{"roots: ignores other flags", ProvideStrategyRoots, true, false, true, false},

			{"mfs: matches when mfs=true", ProvideStrategyMFS, false, false, true, true},
			{"mfs: ignores other flags", ProvideStrategyMFS, true, true, false, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := ShouldProvideForStrategy(tt.strategy, tt.pinned, tt.pinnedRoot, tt.mfs)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("combined strategies use OR logic (else-if bug fix)", func(t *testing.T) {
		// CRITICAL: Tests the fix where bitflag combinations (pinned+mfs) didn't work
		// because of else-if instead of separate if statements
		tests := []struct {
			name                    string
			strategy                ProvideStrategy
			pinned, pinnedRoot, mfs bool
			want                    bool
		}{
			// pinned|mfs: provide if EITHER matches
			{"pinned|mfs when pinned", ProvideStrategyPinned | ProvideStrategyMFS, true, false, false, true},
			{"pinned|mfs when mfs", ProvideStrategyPinned | ProvideStrategyMFS, false, false, true, true},
			{"pinned|mfs when both", ProvideStrategyPinned | ProvideStrategyMFS, true, false, true, true},
			{"pinned|mfs when neither", ProvideStrategyPinned | ProvideStrategyMFS, false, false, false, false},

			// roots|mfs
			{"roots|mfs when root", ProvideStrategyRoots | ProvideStrategyMFS, false, true, false, true},
			{"roots|mfs when mfs", ProvideStrategyRoots | ProvideStrategyMFS, false, false, true, true},
			{"roots|mfs when neither", ProvideStrategyRoots | ProvideStrategyMFS, false, false, false, false},

			// pinned|roots
			{"pinned|roots when pinned", ProvideStrategyPinned | ProvideStrategyRoots, true, false, false, true},
			{"pinned|roots when root", ProvideStrategyPinned | ProvideStrategyRoots, false, true, false, true},
			{"pinned|roots when neither", ProvideStrategyPinned | ProvideStrategyRoots, false, false, false, false},

			// triple combination
			{"all-three when any matches", ProvideStrategyPinned | ProvideStrategyRoots | ProvideStrategyMFS, false, false, true, true},
			{"all-three when none match", ProvideStrategyPinned | ProvideStrategyRoots | ProvideStrategyMFS, false, false, false, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := ShouldProvideForStrategy(tt.strategy, tt.pinned, tt.pinnedRoot, tt.mfs)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("zero strategy never provides", func(t *testing.T) {
		assert.False(t, ShouldProvideForStrategy(ProvideStrategy(0), false, false, false))
		assert.False(t, ShouldProvideForStrategy(ProvideStrategy(0), true, true, true))
	})
}
