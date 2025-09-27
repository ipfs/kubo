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
