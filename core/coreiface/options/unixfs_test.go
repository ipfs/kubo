package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaxHAMTFanoutValidation(t *testing.T) {
	valid := []int{8, 16, 32, 64, 128, 256, 512, 1024}
	for _, v := range valid {
		_, _, err := UnixfsAddOptions(Unixfs.MaxHAMTFanout(v))
		require.NoError(t, err, "fanout %d should be valid", v)
	}

	invalid := []int{-1, 0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 12, 24, 48, 100, 2048, 4096, 999999}
	for _, v := range invalid {
		_, _, err := UnixfsAddOptions(Unixfs.MaxHAMTFanout(v))
		require.Error(t, err, "fanout %d should be invalid", v)
		require.Contains(t, err.Error(), "HAMT fanout must be")
	}
}
