package testutils

import "crypto/rand"

func RandomBytes(n int) []byte {
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return bytes
}

func RandomStr(n int) string {
	return string(RandomBytes(n))
}
