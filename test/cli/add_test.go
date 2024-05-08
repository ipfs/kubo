package cli

import (
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

func TestAdd(t *testing.T) {
	t.Parallel()

	var (
		shortString            = "hello world"
		shortStringCidV0       = "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD"
		shortStringCidV1       = "bafkreifzjut3te2nhyekklss27nh3k72ysco7y32koao5eei66wof36n5e"
		shortStringCidV1Sha512 = "bafkrgqbqt3gerhas23vuzrapkdeqf4vu2dwxp3srdj6hvg6nhsug2tgyn6mj3u23yx7utftq3i2ckw2fwdh5qmhid5qf3t35yvkc5e5ottlw6"
	)

	t.Run("output cid version: default (CIDv0)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("output cid version: follows configuration (CIDv0)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(0)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("output cid version: follows configuration (hash function)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.HashFunction = *config.NewOptionalString("sha2-512")
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1Sha512, cidStr)
	})

	t.Run("output cid version: follows configuration (CIDv1)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1, cidStr)
	})

	t.Run("output cid version: flag overrides configuration", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString, "--cid-version", "0")
		require.Equal(t, shortStringCidV0, cidStr)
	})
}
