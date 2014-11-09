package random

import (
	"bytes"
	randcrypto "crypto/rand"
	"io"
	randmath "math/rand"
)

func WriteRandomBytes(count int64, w io.Writer) error {
	r := &io.LimitedReader{R: randcrypto.Reader, N: count}
	_, err := io.Copy(w, r)
	return err
}

func WritePseudoRandomBytes(count int64, w io.Writer, seed int64) error {
	randmath.Seed(seed)

	// Configurable buffer size
	bufsize := int64(1024 * 1024 * 4)
	b := make([]byte, bufsize)

	for count > 0 {
		if bufsize > count {
			bufsize = count
			b = b[:bufsize]
		}

		var n int64
		for i := int64(0); i < bufsize; i++ {
			n = randmath.Int63()
			for j := 0; j < 8 && i < bufsize; j++ {
				b[i] = byte(n & 0xff)
				n >>= 8
				i++
			}
		}
		count = count - bufsize

		r := bytes.NewReader(b)
		_, err := io.Copy(w, r)
		if err != nil {
			return err
		}
	}
	return nil
}
