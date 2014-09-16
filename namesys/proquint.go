package namesys

import (
	"errors"

	proquint "github.com/bren2010/proquint"
)

type ProquintResolver struct{}

func (r *ProquintResolver) Matches(name string) bool {
	ok, err := proquint.IsProquint(name)
	return err == nil && ok
}

func (r *ProquintResolver) Resolve(name string) (string, error) {
	ok := r.Matches(name)
	if !ok {
		return "", errors.New("not a valid proquint string")
	}
	return string(proquint.Decode(name)), nil
}
