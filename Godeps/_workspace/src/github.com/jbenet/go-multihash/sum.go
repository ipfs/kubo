package multihash

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	sha3 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.crypto/sha3"
)

func Sum(data []byte, code int, length int) (Multihash, error) {
	m := Multihash{}
	err := error(nil)
	if !ValidCode(code) {
		return m, fmt.Errorf("invalid multihash code %d", code)
	}

	var d []byte
	switch code {
	case SHA1:
		d = sumSHA1(data)
	case SHA2_256:
		d = sumSHA256(data)
	case SHA2_512:
		d = sumSHA512(data)
	case SHA3:
		d, err = sumSHA3(data)
	default:
		return m, fmt.Errorf("Function not implemented. Complain to lib maintainer.")
	}

	if err != nil {
		return m, err
	}

	if length < 0 {
		var ok bool
		length, ok = DefaultLengths[code]
		if !ok {
			return m, fmt.Errorf("no default length for code %d", code)
		}
	}

	return Encode(d[0:length], code)
}

func sumSHA1(data []byte) []byte {
	a := sha1.Sum(data)
	return a[0:20]
}

func sumSHA256(data []byte) []byte {
	a := sha256.Sum256(data)
	return a[0:32]
}

func sumSHA512(data []byte) []byte {
	a := sha512.Sum512(data)
	return a[0:64]
}

func sumSHA3(data []byte) ([]byte, error) {
	h := sha3.NewKeccak512()
	if _, err := h.Write(data); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
