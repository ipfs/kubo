// Package types contains common types useful to the inflect package.
package types

// UncountablesType is an array of strings
type UncountablesType []string

// Contains returns a bool if the str is found in the UncountablesType.
func (self UncountablesType) Contains(str string) bool {
	for _, word := range self {
		if word == str {
			return true
		}
	}

	return false
}
