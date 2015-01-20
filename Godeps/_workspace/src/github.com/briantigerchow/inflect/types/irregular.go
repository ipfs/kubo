// Package types contains common types useful to the inflect package.
package types

import "strings"

// IrregularType provides a structure for irregular words that do not follow standard rules.
type IrregularType struct {
	Singular string // The singular form of the irregular word.
	Plural   string // The plural form of the irregular word.
}

// IrregularsType defines a slice of pointers to IrregularType.
type IrregularsType []*IrregularType

// IsIrregular returns an IrregularType and bool if the IrregularsType slice contains the word.
func (self IrregularsType) IsIrregular(str string) (*IrregularType, bool) {
	str = strings.ToLower(str)
	for _, irregular := range self {
		if strings.ToLower(irregular.Singular) == str || strings.ToLower(irregular.Plural) == str {
			return irregular, true
		}
	}

	return nil, false
}

// Irregular if a factory method to a new IrregularType.
func Irregular(singular, plural string) (irregular *IrregularType) {
	irregular = new(IrregularType)
	irregular.Singular = singular
	irregular.Plural = plural

	return
}
