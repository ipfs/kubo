package leveldb

import (
	"bytes"
	"io"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
)

// CastAsReader does type assertions to find the type of a value and attempts
// to turn it into an io.Reader. If not possible, will return ds.ErrInvalidType
func CastAsReader(value interface{}) (io.Reader, error) {
	switch v := value.(type) {
	case io.Reader:
		return v, nil

	case []byte:
		return bytes.NewReader(v), nil

	case string:
		return bytes.NewReader([]byte(v)), nil

	default:
		return nil, ds.ErrInvalidType
	}
}

// // CastAsWriter does type assertions to find the type of a value and attempts
// // to turn it into an io.Writer. If not possible, will return ds.ErrInvalidType
// func CastAsWriter(value interface{}) (err error) {
// 	switch v := value.(type) {
// 	case io.Reader:
// 		return v, nil
//
// 	case []byte:
// 		return bytes.NewReader(v), nil
//
// 	case string:
// 		return bytes.NewReader([]byte(v)), nil
//
// 	default:
// 		return nil, ds.ErrInvalidType
// 	}
// }
