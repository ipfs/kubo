package path

import (
	"errors"
	"unicode/utf8"
)

var ErrPathSegmentEmpty = errors.New("path segment is empty")
var ErrPathSegmentTooLong = errors.New("path segment is too long")
var ErrInvalidUTF8 = errors.New("path is not valid UTF-8")
var ErrRestrictedCharacter = errors.New("path contains a restricted character")
var ErrRestrictedString = errors.New("path is a restricted string")

const (
	MaximumPathSegmentLength = 255 // matching Linux NAME_MAX
)

func ValidateSegmentBytes(segment []byte) error {
	// check the path segment's length first
	if len(segment) == 0 {
		return ErrPathSegmentEmpty
	}

	// check if it's "." or "..", both of which are not permitted
	if (len(segment) == 1 && segment[0] == 0x2e) || (len(segment) == 2 && segment[0] == 0x2e && segment[1] == 0x2e) {
		return ErrRestrictedString
	}

	// succesively decode UTF-8 codepoints
	codepoints := 0
	for i := 0; i < len(segment); {
		decoded, decodeLen := utf8.DecodeRune(segment[i:])

		// did we decode a valid UTF-8 codepoint?
		if decoded == utf8.RuneError && decodeLen <= 1 {
			// no; segment is invalid UTF-8
			return ErrInvalidUTF8
		} else {
			// yes, we decoded one codepoint occupying decodeLen bytes
			i += decodeLen
			codepoints += 1
		}

		// check decoded rune against character exclusions
		if (decoded >= 0 && decoded <= 0x1f) || decoded == 0x7f {
			// control character
			return ErrRestrictedCharacter
		} else if decoded == '/' {
			// slash
			return ErrRestrictedCharacter
		}

		// this character is permitted
		// do we have too many codepoints?
		if codepoints > MaximumPathSegmentLength {
			return ErrPathSegmentTooLong
		}
	}

	// no violations found
	return nil
}

func ValidateSegment(segment string) error {
	return ValidateSegmentBytes([]byte(segment))
}
