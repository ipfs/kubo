// Package types contains common types useful to the inflect package.
package types

import (
	"regexp"
)

// RuleType provides a structure for pluralization/singularizations rules
// of a language.
type RuleType struct {
	Regexp   *regexp.Regexp // The regular expression the rule must match.
	Replacer string         // The replacement to use if the RuleType's Regexp is matched.
}

// RulesType defines a slice of pointers to RuleType.
type RulesType []*RuleType

// Rule if a factory method to a new RuleType.
func Rule(matcher, replacer string) (rule *RuleType) {
	rule = new(RuleType)
	rule.Regexp = regexp.MustCompile(matcher)
	rule.Replacer = replacer

	return
}
