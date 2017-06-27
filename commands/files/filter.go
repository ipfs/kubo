package files

import (
	"strings"

	ignore "github.com/crackcomm/go-gitignore"
)

// Filter - File filter.
type Filter struct {
	// IncludeHidden - Include hidden files.
	IncludeHidden bool
	// Rules - File filter rules.
	Rules *ignore.GitIgnore
}

// NewFilter - Creates a new file filter from .gitignore file and list of rules.
// First argument can be empty and only second one will be used.
func NewFilter(ignorefile string, rules []string, includeHidden bool) (filter *Filter, err error) {
	filter = &Filter{IncludeHidden: includeHidden}
	if ignorefile == "" {
		filter.Rules, err = ignore.CompileIgnoreLines(rules...)
	} else {
		filter.Rules, err = ignore.CompileIgnoreFileAndLines(ignorefile, rules...)
	}
	return
}

// Filter - Returns true if file should be filtered.
func (filter *Filter) Filter(fpath string) bool {
	if !filter.IncludeHidden && strings.HasPrefix(fpath, ".") {
		return true
	}
	return filter.Rules.MatchesPath(fpath)
}
