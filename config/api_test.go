package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertAuthSecret(t *testing.T) {
	for _, testCase := range []struct {
		input  string
		output string
	}{
		{"", ""},
		{"someToken", "Bearer someToken"},
		{"bearer:someToken", "Bearer someToken"},
		{"basic:user:pass", "Basic dXNlcjpwYXNz"},
		{"basic:dXNlcjpwYXNz", "Basic dXNlcjpwYXNz"},
	} {
		assert.Equal(t, testCase.output, ConvertAuthSecret(testCase.input))
	}
}
