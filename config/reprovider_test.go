package config

import "testing"

func TestParseReproviderStrategy(t *testing.T) {
	tests := []struct {
		input  string
		expect ReproviderStrategy
	}{
		{"all", ReproviderStrategyAll},
		{"pinned", ReproviderStrategyPinned},
		{"mfs", ReproviderStrategyMFS},
		{"pinned+mfs", ReproviderStrategyPinned | ReproviderStrategyMFS},
		{"invalid", 0},
		{"all+invalid", ReproviderStrategyAll},
		{"", ReproviderStrategyAll},
		{"flat", ReproviderStrategyAll}, // deprecated, maps to "all"
		{"flat+all", ReproviderStrategyAll},
	}

	for _, tt := range tests {
		result := ParseReproviderStrategy(tt.input)
		if result != tt.expect {
			t.Errorf("ParseReproviderStrategy(%q) = %d, want %d", tt.input, result, tt.expect)
		}
	}
}
