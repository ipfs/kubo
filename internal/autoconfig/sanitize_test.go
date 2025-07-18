package autoconfig

import "testing"

func TestSanitizeForPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"sub.example.com", "sub.example.com"},
		{"example..com", "example_com"},
		{"example...com", "example_com"},
		{"example....com", "example_com"},
		{"..example.com", "_example.com"},
		{"example.com..", "example.com_"},
		{"example/com", "example_com"},
		{"example:8080", "example_8080"},
		{"example.com:8080", "example.com_8080"},
		{"sub..example..com", "sub_example_com"},
		{"valid-name_123.com", "valid-name_123.com"},
		{"123.456.789.012", "123.456.789.012"},
		{"a..b..c", "a_b_c"},
	}

	for _, test := range tests {
		result := sanitizeForPath(test.input)
		if result != test.expected {
			t.Errorf("sanitizeForPath(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
