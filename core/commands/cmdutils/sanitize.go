package cmdutils

import (
	"strings"
	"unicode"
)

const maxRunes = 128

// CleanAndTrim sanitizes untrusted strings from remote peers to prevent display issues
// across web UIs, terminals, and logs. It replaces control characters, format characters,
// and surrogates with U+FFFD (�), then enforces a maximum length of 128 runes.
//
// This follows the libp2p identify specification and RFC 9839 guidance:
// replacing problematic code points is preferred over deletion as deletion
// is a known security risk.
func CleanAndTrim(str string) string {
	// Build sanitized result
	var result []rune
	for _, r := range str {
		// Replace control characters (Cc) with U+FFFD - prevents terminal escapes, CR, LF, etc.
		if unicode.Is(unicode.Cc, r) {
			result = append(result, '\uFFFD')
			continue
		}
		// Replace format characters (Cf) with U+FFFD - prevents RTL/LTR overrides, zero-width chars
		if unicode.Is(unicode.Cf, r) {
			result = append(result, '\uFFFD')
			continue
		}
		// Replace surrogate characters (Cs) with U+FFFD - invalid in UTF-8
		if unicode.Is(unicode.Cs, r) {
			result = append(result, '\uFFFD')
			continue
		}
		// Private use characters (Co) are preserved per spec
		result = append(result, r)
	}

	// Convert to string and trim whitespace
	sanitized := strings.TrimSpace(string(result))

	// Enforce maximum length (128 runes, not bytes)
	runes := []rune(sanitized)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}

	return sanitized
}
