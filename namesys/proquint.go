package namesys

import (
	"errors"

	proquint "github.com/bren2010/proquint"
)

var _ = proquint.Encode

type ProquintResolver struct{}

func (r *ProquintResolver) Resolve(name string) (string, error) {
	ok, err := proquint.IsProquint(name)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("not a valid proquint string")
	}
	return string(proquint.Decode(name)), nil
}
