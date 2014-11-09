package main

import (
	"bytes"
	randcrypto "crypto/rand"
	"fmt"
	"io"
	randmath "math/rand"
	"os"
	"strconv"
)

func main() {
	l := len(os.Args)
	if l != 2 && l != 3 {
		usageError()
	}

	count, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		usageError()
	}

	if l == 2 {
		err = writeRandomBytes(count, os.Stdout)
	} else {
		seed, err2 := strconv.ParseInt(os.Args[2], 10, 64)
		if err2 != nil {
			usageError()
		}
		err = writePseudoRandomBytes(count, os.Stdout, seed)
	}

	if err != nil {
		die(err)
	}
}

func usageError() {
	fmt.Fprintf(os.Stderr, "Usage: %s <count> [<seed>]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "If <seed> is given, output <count> pseudo random bytes made from <seed> (from Go's math/rand)\n")
	fmt.Fprintf(os.Stderr, "Otherwise, output <count> random bytes (from Go's crypto/rand)\n")
	os.Exit(-1)
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v", err)
	os.Exit(-1)
}

func writeRandomBytes(count int64, w io.Writer) error {
	r := &io.LimitedReader{R: randcrypto.Reader, N: count}
	_, err := io.Copy(w, r)
	return err
}

func writePseudoRandomBytes(count int64, w io.Writer, seed int64) error {
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
