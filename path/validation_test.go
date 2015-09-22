package path

import (
	"strings"
	"testing"
)

// these test vectors are chosen as examples of IPFS path validation rules
// these vectors are not themselves the definition of a valid path

// these strings are not valid path segments, but are valid given a prefix or suffix, or when used as part of a larger path
var invalidPathSegments = []string{
	// empty string
	"",

	// special meaning in POSIX, and cleaned away by path.ParsePath()
	".",
	"..",
}

// these substrings are not valid in a path segment, but *are* valid in paths
var invalidPathSegmentSubstrings = []string{
	// slash is not permitted in path segments, since it delimits path segments
	// it is (obviously) valid in paths
	"/",
}

// these substrings are never valid in a path anywhere, even when prefixed/suffixed with valid UTF-8
var invalidPathSubstrings = []string{
	// ASCII/Unicode C0 control characters  are not permitted
	"\u0000", "\u0001", "\u0002", "\u0003", "\u0004", "\u0005", "\u0006", "\u0007",
	"\u0008", "\u0009", "\u000a", "\u000b", "\u000c", "\u000d", "\u000e", "\u000f",
	"\u0010", "\u0011", "\u0012", "\u0013", "\u0014", "\u0015", "\u0016", "\u0017",
	"\u0018", "\u0019", "\u001a", "\u001b", "\u001c", "\u001d", "\u001e", "\u001f",
	"\u007f",

	// bytes which are illegal in UTF-8 are not permitted
	"\xfe", "\xff",
	"\xfe\xff", // the UTF-16 BOM
	"\xff\xfe", // test a reversed UTF-16 BOM because why not

	// malformed UTF-8 sequences are not permitted
	"\x80",
	// TODO: more

	// overlong UTF-8 sequences are not permitted
	"\xc0\x80",
	"\xe0\x80\x80",
	"\xf0\x80\x80\x80",

	// 4-byte UTF-8 sequences above plane 16 are not permitted
	"\xf4\x90\x80\x80", // U+110000
	"\xf7\xbf\xbf\xbf", // U+1FFFFF

	// 5- and 6-byte UTF-8 sequences are not permitted
	"\xf8\x88\x80\x80\x80",     // U+200000
	"\xfb\xbf\xbf\xbf\xbf",     // U+3FFFFF
	"\xfc\x84\x80\x80\x80\x80", // U+400000
	"\xfd\xbf\xbf\xbf\xbf\xbf", // U+7FFFFF

	// surrogate halves/pairs are not permitted
	"\xed\xa0\x80", // U+D800
	"\xed\xaf\xbf", // U+DBFF
	"\xed\xb0\x80", // U+DC00
	"\xed\xbf\xbf", // U+DFFF

	"\xed\xa0\x80\xed\xb0\x80", // U+D800 U+DC00

	// path segments past NAME_MAX bytes:
	strings.Repeat("a", 256),

	// path segments past NAME_MAX characters:
	strings.Repeat("á", 256),
}

// these strings are valid path segments
var validPathSegments = []string{
	// path segments just under NAME_MAX bytes:
	strings.Repeat("a", 255),

	// path segments just under NAME_MAX characters:
	strings.Repeat("á", 255),
}

// these strings *are* valid path segments, even when prefixed/suffixed
var validPathSubstrings = []string{
	// basic strings are fine
	"true",
	"1",
	"ipfs",
	"I P F S",
	"IPFS   ",
	"autorun.inf",
	".DS_Store",
	"Program Files",
	"Progra~1",

	// dots are fine, as long as it's not "." and ".." verbatim
	"foo.",
	".foo",
	"..bar",
	"...",

	// Unicode strings are fine
	// (Go source files -- and by extension Go string literals -- are UTF-8, just like IPFS paths)
	`currencies $€£¥₹₪`,
	`…sigh…`,
	`“smart quotes”`,
	`emoji 😘`,
	`ἄλφα βῆτα`,
	`АВГД`,
	`ᎠᎡᎢᎣ`,
	`z̮ͮ̉ͪͥ͛ͦ̌a̾̈̐̎̿̈l̙̺̮ͭͭ̄̂ͤ̈ͅg̶̺̊͋͒͒ͧő̫̹̟͛̓ͭ`, // "zalgo"

	// 2-byte, 3-byte, and 4-byte UTF-8 sequences are okay up to U+10FFFF
	"\xc2\x80",
	"\xe0\xa0\x80",
	"\xf0\x90\x80\x80",
	"\xf4\x8f\xbf\xbf",

	// things that are fine in IPFS that might be less fine in other contexts
	"\\",      // just a backslash
	"foo:bar", // problematic on Windows and ~OSX
	" ",       // look ma, all whitespace
	"COM1",    // problematic on Windows
	"C:\\",    // very very problematic on Windows

	// all Unicode codepoints are permitted except those that are specifically excluded, so these things are fine too
	"\u00a0",     // non-breaking space
	"\u0156",     // unicode C1 string terminator
	"\u0200",     // non-breaking space
	"\u0300",     // combining diacritical mark without anything on which to combine
	"\u200d",     // zero width joiner (invisible)
	"\u202e",     // right-to-left override; https://xkcd.com/1137/
	"\ufdd0",     // not a character; https://xkcd.com/380/
	"\ufdef",     // not a character
	"\ufeff",     // byte order mark
	"\uf8ff",     // private use character
	"\ufffd",     // Unicode replacement character
	"\ufffe",     // not a character, and sometimes mistakable for a reversed BOM
	"\uffff",     // not a character, and sometimes mistakable for an EOF
	".gi\u200ct", // CVE-2014-9390
}

// given a string slice, return a bigger string slice containing prefixes and suffixes
func pathSegmentVariations(segments []string) []string {
	out := make([]string, 0, len(segments)*4)

	for _, segment := range segments {
		out = append(out, segment)
		out = append(out, "foo"+segment)
		out = append(out, segment+"bar")
		out = append(out, "baz"+segment+"quxx")
	}

	return out
}

func TestValidPathSegments(t *testing.T) {
	pathSegments := append(
		validPathSegments,
		pathSegmentVariations(validPathSubstrings)...,
	)

	for _, segment := range pathSegments {
		if err := ValidateSegment(segment); err != nil {
			t.Fatalf("expected string segment %q to be valid, got %q", segment, err)
		}

		if err := ValidateSegmentBytes([]byte(segment)); err != nil {
			t.Fatalf("expected []byte segment %q to be valid, got %q", segment, err)
		}

		if _, err := ParsePath("/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/" + segment); err != nil {
			t.Fatalf("expected Path based on %q to be valid, got %q", segment, err)
		}
	}
}

func TestInvalidPathSegments(t *testing.T) {
	// invalidPathSegments are only invalid as path segments, but are valid in paths
	// they *are* valid when passed through pathSegmentVariations() to gain a prefix/suffix, and
	// they *are* valid as a Path()
	for _, segment := range invalidPathSegments {
		// invalid as a path segment, whether bytes or string
		if err := ValidateSegment(segment); err == nil {
			t.Fatalf("expected string segment %q to be invalid, but it was valid", segment)
		}

		if err := ValidateSegmentBytes([]byte(segment)); err == nil {
			t.Fatalf("expected []byte segment %q to be invalid, but it was valid", segment)
		}

		// valid as a Path
		if _, err := ParsePath("/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a/" + segment); err != nil {
			t.Fatalf("expected Path based on %q to be valid, got %q", segment, err)
		}
	}
}

func TestInvalidPathSegmentSubstrings(t *testing.T) {
	// invalidPathSegmentSubstrings are invalid as path segments, whether prefixed or not, but *are* valid in paths
	for _, segment := range pathSegmentVariations(invalidPathSegmentSubstrings) {
		// invalid as a path segment, whether bytes or string
		if err := ValidateSegment(segment); err == nil {
			t.Fatalf("expected string segment %q to be invalid, but it was valid", segment)
		}

		if err := ValidateSegmentBytes([]byte(segment)); err == nil {
			t.Fatalf("expected []byte segment %q to be invalid, but it was valid", segment)
		}

		if _, err := ParsePath("/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a/" + segment); err != nil {
			t.Fatalf("expected Path based on %q to be valid, got %q", segment, err)
		}
	}
}

func TestInvalidPathSubstrings(t *testing.T) {
	// invalidPathSubstrings are invalid when appearing anywhere Path-related

	for _, segment := range pathSegmentVariations(invalidPathSubstrings) {
		if err := ValidateSegment(segment); err == nil {
			t.Fatalf("expected string segment %q to be invalid, but it was valid", segment)
		}

		if err := ValidateSegmentBytes([]byte(segment)); err == nil {
			t.Fatalf("expected []byte segment %q to be invalid, but it was valid", segment)
		}

		if _, err := ParsePath("/ipfs/QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n/a/" + segment); err == nil {
			t.Fatalf("expected Path based on %q to be invalid, but it was valid", segment)
		}
	}
}
