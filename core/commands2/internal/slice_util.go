package internal

import (
	"io"

	u "github.com/jbenet/go-ipfs/util"
)

func CastToReaders(slice []interface{}) ([]io.Reader, error) {
	readers := make([]io.Reader, 0)
	for _, arg := range slice {
		reader, ok := arg.(io.Reader)
		if !ok {
			return nil, u.ErrCast()
		}
		readers = append(readers, reader)
	}
	return readers, nil
}

func CastToStrings(slice []interface{}) ([]string, error) {
	strs := make([]string, 0)
	for _, maybe := range slice {
		str, ok := maybe.(string)
		if !ok {
			return nil, u.ErrCast()
		}
		strs = append(strs, str)
	}
	return strs, nil
}
