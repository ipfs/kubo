package libp2p

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPercentage(t *testing.T) {
	require.True(t, abovePercentage(10, 100, 10))
	require.True(t, abovePercentage(100, 100, 99))
}
