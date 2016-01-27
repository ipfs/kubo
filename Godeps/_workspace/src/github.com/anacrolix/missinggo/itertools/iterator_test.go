package itertools

import (
	"testing"

	"gx/ipfs/QmZwjfAKWe7vWZ8f48u7AGA1xYfzR1iCD9A2XSCYFRBWot/testify/require"
)

func TestIterator(t *testing.T) {
	const s = "AAAABBBCCDAABBB"
	si := StringIterator(s)
	for i := range s {
		require.True(t, si.Next())
		require.Equal(t, s[i], si.Value().(byte))
	}
	require.False(t, si.Next())
}
