package pin

import "github.com/ipfs/go-ipfs/blocks/key"

func ignoreKeys(key.Key) {}

func copyMap(m map[key.Key]uint16) map[key.Key]uint64 {
	c := make(map[key.Key]uint64, len(m))
	for k, v := range m {
		c[k] = uint64(v)
	}
	return c
}
