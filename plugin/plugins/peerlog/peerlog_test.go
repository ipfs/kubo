package peerlog

import "testing"

func TestExtractEnabled(t *testing.T) {
	for _, c := range []struct {
		name     string
		config   any
		expected bool
	}{
		{
			name:     "nil config returns false",
			config:   nil,
			expected: false,
		},
		{
			name:     "returns false when config is not a string map",
			config:   1,
			expected: false,
		},
		{
			name:     "returns false when config has no Enabled field",
			config:   map[string]any{},
			expected: false,
		},
		{
			name:     "returns false when config has a null Enabled field",
			config:   map[string]any{"Enabled": nil},
			expected: false,
		},
		{
			name:     "returns false when config has a non-boolean Enabled field",
			config:   map[string]any{"Enabled": 1},
			expected: false,
		},
		{
			name:     "returns the value of the Enabled field",
			config:   map[string]any{"Enabled": true},
			expected: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			isEnabled := extractEnabled(c.config)
			if isEnabled != c.expected {
				t.Fatalf("expected %v, got %v", c.expected, isEnabled)
			}
		})
	}
}
