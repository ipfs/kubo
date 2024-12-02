package cli

import (
	"context"
	"path"
	"testing"

	rpcapi "github.com/ipfs/kubo/client/rpc"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestRPCUnixSocket(t *testing.T) {
	node := harness.NewT(t).NewNode().Init()

	sockDir := node.Dir
	sockAddr := path.Join("/unix", sockDir, "sock")

	node.UpdateConfig(func(cfg *config.Config) {
		//cfg.Addresses.API = append(cfg.Addresses.API, sockPath)
		cfg.Addresses.API = []string{sockAddr}
	})
	t.Log("Starting daemon with unix socket:", sockAddr)
	node.StartDaemon()

	unixMaddr, err := multiaddr.NewMultiaddr(sockAddr)
	require.NoError(t, err)

	apiClient, err := rpcapi.NewApi(unixMaddr)
	require.NoError(t, err)

	var ver struct {
		Version string
	}
	err = apiClient.Request("version").Exec(context.Background(), &ver)
	require.NoError(t, err)
	require.NotEmpty(t, ver)
	t.Log("Got version:", ver.Version)

	var res struct {
		ID string
	}
	err = apiClient.Request("id").Exec(context.Background(), &res)
	require.NoError(t, err)
	require.NotEmpty(t, res)
	t.Log("Got ID:", res.ID)

	node.StopDaemon()
}
