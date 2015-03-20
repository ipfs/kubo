package migrate

import (
	"fmt"
)

// Options are migration options. For now all flags are options.
type Options struct {
	Flags
}

// Migration represents
type Migration interface {

	// Versions is the "v-to-v" version string.
	Versions() string

	// Reversible returns whether this migration can be reverted.
	// Endeavor to make them all reversible. This is here only to warn users
	// in case this is not the case.
	Reversible() bool

	// Apply applies the migration in question.
	Apply(Options) error

	// Revert un-applies the migration in question. This should be best-effort.
	// Some migrations are definitively one-way. If so, return an error.
	Revert(Options) error
}

func SplitVersion(s string) (from int, to int) {
	_, err := fmt.Scanf(s, "%d-to-%d", &from, &to)
	if err != nil {
		panic(err.Error())
	}
	return
}
