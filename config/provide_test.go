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

func TestValidateProvideConfig_BufferedOptions(t *testing.T) {
	tests := []struct {
		name          string
		batchSize     *int64
		idleWriteTime *time.Duration
		wantError     string
	}{
		// Valid cases
		{"defaults", nil, nil, ""},
		{"valid batch 512", ptr(int64(512)), nil, ""},
		{"valid batch max", ptr(int64(10000)), nil, ""},
		{"valid idle 5s", nil, ptr(5 * time.Second), ""},
		{"valid idle min", nil, ptr(1 * time.Second), ""},
		{"valid idle max", nil, ptr(5 * time.Minute), ""},
		{"valid both custom", ptr(int64(2048)), ptr(10 * time.Second), ""},

		// Invalid batch sizes
		{"invalid batch zero", ptr(int64(0)), nil, "Provide.DHT.BufferedBatchSize must be positive, got 0"},
		{"invalid batch negative", ptr(int64(-100)), nil, "Provide.DHT.BufferedBatchSize must be positive, got -100"},
		{"invalid batch too large", ptr(int64(10001)), nil, "Provide.DHT.BufferedBatchSize must be <= 10000, got 10001"},

		// Invalid idle times
		{"invalid idle too short", nil, ptr(500 * time.Millisecond), "Provide.DHT.BufferedIdleWriteTime must be >= 1s, got 500ms"},
		{"invalid idle too long", nil, ptr(6 * time.Minute), "Provide.DHT.BufferedIdleWriteTime must be <= 5m, got 6m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Provide{DHT: ProvideDHT{}}
			if tt.batchSize != nil {
				cfg.DHT.BufferedBatchSize = NewOptionalInteger(*tt.batchSize)
			}
			if tt.idleWriteTime != nil {
				cfg.DHT.BufferedIdleWriteTime = NewOptionalDuration(*tt.idleWriteTime)
			}

			err := ValidateProvideConfig(cfg)
			if tt.wantError != "" {
				require.Error(t, err, "expected error")
				assert.Equal(t, tt.wantError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
