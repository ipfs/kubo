package cli

import (
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
)

func TestDaemon(t *testing.T) {
	t.Parallel()

	t.Run("daemon starts if api is set to null", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.SetIPFSConfig("API", nil)
		node.IPFS("daemon") // can't use .StartDaemon because it do a .WaitOnAPI
	})
}
