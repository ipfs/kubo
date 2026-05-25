package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProvideStrategy(t *testing.T) {
	t.Run("valid strategies", func(t *testing.T) {
		tests := []struct {
			input  string
			expect ProvideStrategy
		}{
			{"all", ProvideStrategyAll},
			{"pinned", ProvideStrategyPinned},
			{"roots", ProvideStrategyRoots},
			{"mfs", ProvideStrategyMFS},
			{"pinned+mfs", ProvideStrategyPinned | ProvideStrategyMFS},
			{"pinned+roots", ProvideStrategyPinned | ProvideStrategyRoots},
			{"pinned+mfs+roots", ProvideStrategyPinned | ProvideStrategyMFS | ProvideStrategyRoots},
			{"", ProvideStrategyAll},                                   // empty string = default = all
			{"flat", ProvideStrategyAll},                               // deprecated, maps to "all"
			{"flat+all", ProvideStrategyAll},                           // redundant but valid
			{"all+all", ProvideStrategyAll},                            // redundant but valid
			{"mfs+pinned", ProvideStrategyMFS | ProvideStrategyPinned}, // order doesn't matter
			// +unique and +entities modifiers
			{"pinned+unique", ProvideStrategyPinned | ProvideStrategyUnique},
			{"pinned+entities", ProvideStrategyPinned | ProvideStrategyEntities | ProvideStrategyUnique},
			{"pinned+unique+entities", ProvideStrategyPinned | ProvideStrategyUnique | ProvideStrategyEntities},
			{"mfs+unique", ProvideStrategyMFS | ProvideStrategyUnique},
			{"mfs+entities", ProvideStrategyMFS | ProvideStrategyEntities | ProvideStrategyUnique},
			{"pinned+mfs+unique", ProvideStrategyPinned | ProvideStrategyMFS | ProvideStrategyUnique},
			{"pinned+mfs+entities", ProvideStrategyPinned | ProvideStrategyMFS | ProvideStrategyEntities | ProvideStrategyUnique},
		}

		for _, tt := range tests {
			result, err := ParseProvideStrategy(tt.input)
			require.NoError(t, err, "ParseProvideStrategy(%q)", tt.input)
			assert.Equal(t, tt.expect, result, "ParseProvideStrategy(%q)", tt.input)
		}
	})

	t.Run("unknown token (including typos)", func(t *testing.T) {
		tests := []struct {
			input string
			err   string
		}{
			{"invalid", `unknown provide strategy token: "invalid"`},
			{"uniuqe", `unknown provide strategy token: "uniuqe"`},        // typo of "unique"
			{"entites", `unknown provide strategy token: "entites"`},      // cspell:disable-line -- intentional typo of "entities"
			{"pinned+uniuqe", `unknown provide strategy token: "uniuqe"`}, // typo in combo
		}

		for _, tt := range tests {
			_, err := ParseProvideStrategy(tt.input)
			require.Error(t, err, "ParseProvideStrategy(%q) should fail", tt.input)
			assert.Contains(t, err.Error(), tt.err)
		}
	})

	t.Run("empty token from delimiter", func(t *testing.T) {
		tests := []string{
			"pinned+",     // trailing +
			"+pinned",     // leading +
			"pinned++mfs", // double +
		}

		for _, input := range tests {
			_, err := ParseProvideStrategy(input)
			require.Error(t, err, "ParseProvideStrategy(%q) should fail", input)
			assert.Contains(t, err.Error(), "empty token")
		}
	})

	t.Run("all cannot be combined with other strategies", func(t *testing.T) {
		tests := []string{
			"all+pinned",
			"all+mfs",
			"all+roots",
			"flat+pinned",
			"all+pinned+mfs",
		}

		for _, input := range tests {
			_, err := ParseProvideStrategy(input)
			require.Error(t, err, "ParseProvideStrategy(%q) should fail", input)
			assert.Contains(t, err.Error(), "cannot be combined")
		}
	})

	t.Run("+unique/+entities require base strategy", func(t *testing.T) {
		tests := []string{
			"unique",              // modifier alone
			"entities",            // modifier alone
			"unique+entities",     // modifiers without base
			"roots+unique",        // roots is incompatible
			"roots+entities",      // roots is incompatible
			"roots+pinned+unique", // roots mixed with pinned+unique
		}

		for _, input := range tests {
			_, err := ParseProvideStrategy(input)
			require.Error(t, err, "ParseProvideStrategy(%q) should fail", input)
		}
	})
}

func TestMustParseProvideStrategy(t *testing.T) {
	t.Run("valid input returns strategy", func(t *testing.T) {
		assert.Equal(t, ProvideStrategyAll, MustParseProvideStrategy("all"))
		assert.Equal(t, ProvideStrategyPinned|ProvideStrategyMFS, MustParseProvideStrategy("pinned+mfs"))
	})

	t.Run("invalid input panics", func(t *testing.T) {
		assert.Panics(t, func() { MustParseProvideStrategy("bogus") })
		assert.Panics(t, func() { MustParseProvideStrategy("all+pinned") })
	})
}

func TestValidateProvideConfig_Strategy(t *testing.T) {
	t.Run("valid strategies", func(t *testing.T) {
		for _, s := range []string{
			"all", "pinned", "roots", "mfs", "pinned+mfs",
			"pinned+unique", "pinned+entities", "pinned+mfs+entities",
		} {
			cfg := &Provide{Strategy: NewOptionalString(s)}
			require.NoError(t, ValidateProvideConfig(cfg), "strategy=%q", s)
		}
	})

	t.Run("default (nil) strategy is valid", func(t *testing.T) {
		cfg := &Provide{}
		require.NoError(t, ValidateProvideConfig(cfg))
	})

	t.Run("invalid strategy", func(t *testing.T) {
		cfg := &Provide{Strategy: NewOptionalString("bogus")}
		err := ValidateProvideConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Provide.Strategy")
	})

	t.Run("all combined with others", func(t *testing.T) {
		cfg := &Provide{Strategy: NewOptionalString("all+pinned")}
		err := ValidateProvideConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be combined")
	})
}

func TestValidateProvideConfig_Interval(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		enabled  Flag
		wantErr  bool
		errMsg   string
	}{
		{"valid default (22h)", 22 * time.Hour, Default, false, ""},
		{"valid max (48h)", 48 * time.Hour, Default, false, ""},
		{"valid small (1h)", 1 * time.Hour, Default, false, ""},
		{"valid zero with explicit Enabled=true", 0, True, false, ""},
		{"valid zero with explicit Enabled=false", 0, False, false, ""},
		{"invalid zero without explicit Provide.Enabled", 0, Default, true, "set Provide.Enabled explicitly"},
		{"invalid over limit (49h)", 49 * time.Hour, Default, true, "must be less than or equal to DHT provider record validity"},
		{"invalid over limit (72h)", 72 * time.Hour, Default, true, "must be less than or equal to DHT provider record validity"},
		{"invalid negative", -1 * time.Hour, Default, true, "must be non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Provide{
				Enabled: tt.enabled,
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

func TestValidateProvideConfig_BloomFPRate(t *testing.T) {
	tests := []struct {
		name    string
		fpRate  int64
		wantErr bool
		errMsg  string
	}{
		{"valid default value", DefaultProvideBloomFPRate, false, ""},
		{"valid minimum (1M)", MinProvideBloomFPRate, false, ""},
		{"valid high (10M)", 10_000_000, false, ""},
		{"valid very high (100M)", 100_000_000, false, ""},
		{"invalid below minimum (999_999)", 999_999, true, "must be >="},
		{"invalid small (10_000)", 10_000, true, "must be >="},
		{"invalid one", 1, true, "must be >="},
		{"invalid zero", 0, true, "must be >="},
		{"invalid negative", -1, true, "must be >="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Provide{
				BloomFPRate: NewOptionalInteger(tt.fpRate),
			}

			err := ValidateProvideConfig(cfg)

			if tt.wantErr {
				require.Error(t, err, "expected error for fpRate=%d", tt.fpRate)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message mismatch")
				}
			} else {
				require.NoError(t, err, "unexpected error for fpRate=%d", tt.fpRate)
			}
		})
	}

	t.Run("default (nil) BloomFPRate is valid", func(t *testing.T) {
		cfg := &Provide{}
		require.NoError(t, ValidateProvideConfig(cfg))
	})
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
