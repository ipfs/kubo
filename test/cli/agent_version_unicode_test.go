package cli

import (
	"strings"
	"testing"

	"github.com/ipfs/kubo/core/commands/cmdutils"
	"github.com/stretchr/testify/assert"
)

func TestCleanAndTrimUnicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic ASCII",
			input:    "kubo/1.0.0",
			expected: "kubo/1.0.0",
		},
		{
			name:     "Polish characters preserved",
			input:    "test-ąęćłńóśźż",
			expected: "test-ąęćłńóśźż",
		},
		{
			name:     "Chinese characters preserved",
			input:    "版本-中文测试",
			expected: "版本-中文测试",
		},
		{
			name:     "Arabic text preserved",
			input:    "اختبار-العربية",
			expected: "اختبار-العربية",
		},
		{
			name:     "Emojis preserved",
			input:    "version-1.0-🚀-🎉",
			expected: "version-1.0-🚀-🎉",
		},
		{
			name:     "Complex Unicode with combining marks preserved",
			input:    "h̸̢̢̢̢̢̢̢̢̢̢e̵̵̵̵̵̵̵̵̵̵l̷̷̷̷̷̷̷̷̷̷l̶̶̶̶̶̶̶̶̶̶o̴̴̴̴̴̴̴̴̴̴",
			expected: "h̸̢̢̢̢̢̢̢̢̢̢e̵̵̵̵̵̵̵̵̵̵l̷̷̷̷̷̷̷̷̷̷l̶̶̶̶̶̶̶̶̶̶o̴̴̴̴̴̴̴̴̴̴", // Preserved as-is (only 50 runes)
		},
		{
			name:     "Long text with combining marks truncated at 128",
			input:    strings.Repeat("ẽ̸̢̛̖̬͈͉͖͇͈̭̥́̓̌̾͊̊̂̄̍̅̂͌́", 10),                                                                                   // Very long text (260 runes)
			expected: "ẽ̸̢̛̖̬͈͉͖͇͈̭̥́̓̌̾͊̊̂̄̍̅̂͌́ẽ̸̢̛̖̬͈͉͖͇͈̭̥́̓̌̾͊̊̂̄̍̅̂͌́ẽ̸̢̛̖̬͈͉͖͇͈̭̥́̓̌̾͊̊̂̄̍̅̂͌́ẽ̸̢̛̖̬͈͉͖͇͈̭̥́̓̌̾͊̊̂̄̍̅̂͌́ẽ̸̢̛̖̬͈͉͖͇͈̭̥́̓̌̾͊̊̂̄̍̅̂", // Truncated at 128 runes
		},
		{
			name:     "Zero-width characters replaced with U+FFFD",
			input:    "test\u200Bzero\u200Cwidth\u200D\uFEFFchars",
			expected: "test�zero�width��chars",
		},
		{
			name:     "RTL/LTR override replaced with U+FFFD",
			input:    "test\u202Drtl\u202Eltr\u202Aoverride",
			expected: "test�rtl�ltr�override",
		},
		{
			name:     "Bidi isolates replaced with U+FFFD",
			input:    "test\u2066bidi\u2067isolate\u2068text\u2069end",
			expected: "test�bidi�isolate�text�end",
		},
		{
			name:     "Control characters replaced with U+FFFD",
			input:    "test\x00null\x1Fescape\x7Fdelete",
			expected: "test�null�escape�delete",
		},
		{
			name:     "Combining marks preserved",
			input:    "e\u0301\u0302\u0303\u0304\u0305", // e with 5 combining marks
			expected: "e\u0301\u0302\u0303\u0304\u0305", // All preserved
		},
		{
			name:     "No truncation at 70 characters",
			input:    "123456789012345678901234567890123456789012345678901234567890123456789",
			expected: "123456789012345678901234567890123456789012345678901234567890123456789",
		},
		{
			name:     "No truncation with Unicode - 70 rockets preserved",
			input:    strings.Repeat("🚀", 70),
			expected: strings.Repeat("🚀", 70),
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace with control chars",
			input:    "   \t\n   ",
			expected: "\uFFFD\uFFFD", // Tab and newline become U+FFFD, spaces trimmed
		},
		{
			name:     "Leading and trailing whitespace",
			input:    "  test  ",
			expected: "test",
		},
		{
			name:     "Complex mix - invisible chars replaced with U+FFFD, Unicode preserved",
			input:    "kubo/1.0-🚀\u200B h̸̢̏̔ḛ̶̽̀s̵t\u202E-ąęł-中文",
			expected: "kubo/1.0-🚀� h̸̢̏̔ḛ̶̽̀s̵t�-ąęł-中文",
		},
		{
			name:     "Emoji with skin tone preserved",
			input:    "👍🏽", // Thumbs up with skin tone modifier
			expected: "👍🏽", // Preserved as-is
		},
		{
			name:     "Mixed scripts preserved",
			input:    "Hello-你好-مرحبا-Здравствуйте",
			expected: "Hello-你好-مرحبا-Здравствуйте",
		},
		{
			name:     "Format characters replaced with U+FFFD",
			input:    "test\u00ADsoft\u2060word\u206Fnom\u200Ebreak",
			expected: "test�soft�word�nom�break", // Soft hyphen, word joiner, etc replaced
		},
		{
			name:     "Complex Unicode text with many combining marks (91 runes, no truncation)",
			input:    "ț̸̢͙̞̖̏̔ȩ̶̰͓̪͎̱̠̥̳͔̽̀̃̿̌̾̀͗̕̕͜s̵̢̛̖̬͈͉͖͇͈̭̥̃́̓̌̾͊̊̂̄̍̅̂͌́ͅţ̴̯̹̪͖͓̘̊́̑̄̋̈́͐̈́̔̇̄̂́̎̓͛͠ͅ test",
			expected: "ț̸̢͙̞̖̏̔ȩ̶̰͓̪͎̱̠̥̳͔̽̀̃̿̌̾̀͗̕̕͜s̵̢̛̖̬͈͉͖͇͈̭̥̃́̓̌̾͊̊̂̄̍̅̂͌́ͅţ̴̯̹̪͖͓̘̊́̑̄̋̈́͐̈́̔̇̄̂́̎̓͛͠ͅ test", // Not truncated (91 < 128)
		},
		{
			name:     "Truncation at 128 characters",
			input:    strings.Repeat("a", 150),
			expected: strings.Repeat("a", 128),
		},
		{
			name:     "Truncation with Unicode at 128",
			input:    strings.Repeat("🚀", 150),
			expected: strings.Repeat("🚀", 128),
		},
		{
			name:     "Private use characters preserved (per spec)",
			input:    "test\uE000\uF8FF", // Private use area characters
			expected: "test\uE000\uF8FF", // Should be preserved
		},
		{
			name:     "U+FFFD replacement for multiple categories",
			input:    "a\x00b\u200Cc\u202Ed",   // control, format chars
			expected: "a\uFFFDb\uFFFDc\uFFFDd", // All replaced with U+FFFD
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmdutils.CleanAndTrim(tt.input)
			assert.Equal(t, tt.expected, result, "CleanAndTrim(%q) = %q, want %q", tt.input, result, tt.expected)
		})
	}
}

func TestCleanAndTrimIdempotent(t *testing.T) {
	// Test that applying CleanAndTrim twice gives the same result
	inputs := []string{
		"test-ąęćłńóśźż",
		"版本-中文测试",
		"version-1.0-🚀-🎉",
		"h̸e̵l̷l̶o̴ w̸o̵r̷l̶d̴",
		"test\u200Bzero\u200Cwidth",
	}

	for _, input := range inputs {
		once := cmdutils.CleanAndTrim(input)
		twice := cmdutils.CleanAndTrim(once)
		assert.Equal(t, once, twice, "CleanAndTrim should be idempotent for %q", input)
	}
}

func TestCleanAndTrimSecurity(t *testing.T) {
	// Test that all invisible/dangerous characters are removed
	tests := []struct {
		name  string
		input string
		check func(string) bool
	}{
		{
			name:  "No zero-width spaces",
			input: "test\u200B\u200C\u200Dtest",
			check: func(s string) bool {
				return !strings.Contains(s, "\u200B") && !strings.Contains(s, "\u200C") && !strings.Contains(s, "\u200D")
			},
		},
		{
			name:  "No bidi overrides",
			input: "test\u202A\u202B\u202C\u202D\u202Etest",
			check: func(s string) bool {
				for _, r := range []rune{0x202A, 0x202B, 0x202C, 0x202D, 0x202E} {
					if strings.ContainsRune(s, r) {
						return false
					}
				}
				return true
			},
		},
		{
			name:  "No control characters",
			input: "test\x00\x01\x02\x1F\x7Ftest",
			check: func(s string) bool {
				for _, r := range s {
					if r < 0x20 || r == 0x7F {
						return false
					}
				}
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmdutils.CleanAndTrim(tt.input)
			assert.True(t, tt.check(result), "Security check failed for %q -> %q", tt.input, result)
		})
	}
}
