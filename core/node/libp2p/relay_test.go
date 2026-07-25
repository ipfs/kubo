package libp2p

import (
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/stretchr/testify/require"
)

func TestHolePunching(t *testing.T) {
	t.Run("explicitly enabled without a relay client fails startup", func(t *testing.T) {
		_, err := HolePunching(config.True, false)()
		require.Error(t, err)
	})

	t.Run("left at default without a relay client is silently disabled", func(t *testing.T) {
		opts, err := HolePunching(config.Default, false)()
		require.NoError(t, err)
		require.Empty(t, opts.Opts)
	})

	t.Run("explicitly disabled without a relay client is a no-op", func(t *testing.T) {
		opts, err := HolePunching(config.False, false)()
		require.NoError(t, err)
		require.Empty(t, opts.Opts)
	})

	t.Run("enabled with a relay client adds the libp2p option", func(t *testing.T) {
		opts, err := HolePunching(config.True, true)()
		require.NoError(t, err)
		require.Len(t, opts.Opts, 1)
	})
}
