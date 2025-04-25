package testutils

import (
	"crypto/sha256"
	"io"

	"github.com/dustin/go-humanize"
	"golang.org/x/crypto/chacha20"
)

type randomReader struct {
	cipher    *chacha20.Cipher
	remaining int64
}

func (r *randomReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	// Generate random bytes directly into the provided buffer
	r.cipher.XORKeyStream(p[:n], make([]byte, n))
	r.remaining -= n
	return int(n), nil
}

// createRandomReader produces specified number of pseudo-random bytes
// from a seed.
func DeterministicRandomReader(sizeStr string, seed string) (io.Reader, error) {
	size, err := humanize.ParseBytes(sizeStr)
	if err != nil {
		return nil, err
	}
	// Hash the seed string to a 32-byte key for ChaCha20
	key := sha256.Sum256([]byte(seed))
	// Use ChaCha20 for deterministic random bytes
	var nonce [chacha20.NonceSize]byte // Zero nonce for simplicity
	cipher, err := chacha20.NewUnauthenticatedCipher(key[:chacha20.KeySize], nonce[:])
	if err != nil {
		return nil, err
	}
	return &randomReader{cipher: cipher, remaining: int64(size)}, nil
}
