package pin

import "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"

func ignoreKeys(key.Key) {}

func copyMap(m map[key.Key]uint16) map[key.Key]uint64 {
	c := make(map[key.Key]uint64, len(m))
	for k, v := range m {
		c[k] = uint64(v)
	}
	return c
}
