package config

import "testing"

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
