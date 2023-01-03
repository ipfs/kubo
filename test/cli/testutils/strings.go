package testutils

import (
	"bufio"
	"fmt"
	"strings"
)

// StrCat takes a bunch of strings or string slices
// and concats them all together into one string slice.
// If an arg is not one of those types, this panics.
// If an arg is an empty string, it is dropped.
func StrCat(args ...interface{}) []string {
	res := make([]string, 0)
	for _, a := range args {
		if s, ok := a.(string); ok {
			if s != "" {
				res = append(res, s)
			}
			continue
		}
		if ss, ok := a.([]string); ok {
			for _, s := range ss {
				if s != "" {
					res = append(res, s)
				}
			}
			continue
		}
		panic(fmt.Sprintf("arg '%v' must be a string or string slice, but is '%T'", a, a))
	}
	return res
}

// PreviewStr returns a preview of s, which is a prefix for logging that avoids dumping a huge string to logs.
func PreviewStr(s string) string {
	suffix := "..."
	previewLength := 10
	if len(s) < previewLength {
		previewLength = len(s)
		suffix = ""
	}
	return s[0:previewLength] + suffix
}

func SplitLines(s string) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
