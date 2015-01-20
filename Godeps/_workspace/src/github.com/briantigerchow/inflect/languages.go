// Package inflect provides an inflector.
package inflect

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/briantigerchow/inflect/languages"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/briantigerchow/inflect/types"
)

var (
	// Language to use when converting a word from it's plural to
	// singular forms and vice versa.
	Language = "en"

	// Languages avaiable for converting a word from
	// it's plural to singular forms and vice versa.
	Languages = map[string]*types.LanguageType{
		"en": languages.English,
	}
)
