package libp2p

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/stretchr/testify/require"
)

func TestNonPublicAddrPublishing(t *testing.T) {
	for _, tc := range []struct {
		name         string
		flag         config.Flag
		wantOptCount int
		wantDisabled bool
	}{
		{"unset defers to go-libp2p", config.Default, 0, false},
		{"true announces private addrs, as a LAN-only node needs", config.True, 1, false},
		{"false keeps private addrs off identify and the DHT", config.False, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := NonPublicAddrPublishing(tc.flag)()
			require.Len(t, opts.Opts, tc.wantOptCount)

			var cfg libp2p.Config
			for _, opt := range opts.Opts {
				require.NoError(t, opt(&cfg))
			}
			require.Equal(t, tc.wantDisabled, cfg.DisableNonPublicAddrPublishing)
		})
	}
}

func TestPrioritize(t *testing.T) {
	// The option is encoded into the port number of a TCP multiaddr.
	// By extracting the port numbers obtained from the applied option, we can make sure that
	// prioritization sorted the options correctly.
	newOption := func(num int) libp2p.Option {
		return func(cfg *libp2p.Config) error {
			cfg.ListenAddrs = append(cfg.ListenAddrs, ma.StringCast(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", num)))
			return nil
		}
	}

	extractNums := func(cfg *libp2p.Config) []int {
		addrs := cfg.ListenAddrs
		nums := make([]int, 0, len(addrs))
		for _, addr := range addrs {
			_, comp := ma.SplitLast(addr)
			num, err := strconv.Atoi(comp.Value())
			require.NoError(t, err)
			nums = append(nums, num)
		}
		return nums
	}

	t.Run("using default priorities", func(t *testing.T) {
		opts := []priorityOption{
			{defaultPriority: 200, opt: newOption(200)},
			{defaultPriority: 1, opt: newOption(1)},
			{defaultPriority: 300, opt: newOption(300)},
		}
		var cfg libp2p.Config
		require.NoError(t, prioritizeOptions(opts)(&cfg))
		require.Equal(t, extractNums(&cfg), []int{1, 200, 300})
	})

	t.Run("using custom priorities", func(t *testing.T) {
		opts := []priorityOption{
			{defaultPriority: 200, priority: 1, opt: newOption(1)},
			{defaultPriority: 1, priority: 300, opt: newOption(300)},
			{defaultPriority: 300, priority: 20, opt: newOption(20)},
		}
		var cfg libp2p.Config
		require.NoError(t, prioritizeOptions(opts)(&cfg))
		require.Equal(t, extractNums(&cfg), []int{1, 20, 300})
	})
}
