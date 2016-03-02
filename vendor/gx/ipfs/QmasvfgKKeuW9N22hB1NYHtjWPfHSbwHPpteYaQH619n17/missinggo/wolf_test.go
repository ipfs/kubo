package missinggo

import (
	"testing"

	"gx/ipfs/QmZwjfAKWe7vWZ8f48u7AGA1xYfzR1iCD9A2XSCYFRBWot/testify/require"
)

func cryHeard() bool {
	return CryHeard()
}

func TestCrySameLocation(t *testing.T) {
	require.True(t, cryHeard())
	require.True(t, cryHeard())
	require.False(t, cryHeard())
	require.True(t, cryHeard())
	require.False(t, cryHeard())
	require.False(t, cryHeard())
	require.False(t, cryHeard())
	require.True(t, cryHeard())
}

func TestCryDifferentLocations(t *testing.T) {
	require.True(t, CryHeard())
	require.True(t, CryHeard())
	require.True(t, CryHeard())
	require.True(t, CryHeard())
	require.True(t, CryHeard())
}
