package cli

import (
	"testing"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdd(t *testing.T) {
	t.Parallel()

	var (
		shortString                 = "hello world"
		shortStringCidV0            = "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD"              // cidv0 - dag-pb - sha2-256
		shortStringCidV1            = "bafkreifzjut3te2nhyekklss27nh3k72ysco7y32koao5eei66wof36n5e" // cidv1 - raw - sha2-256
		shortStringCidV1NoRawLeaves = "bafybeihykld7uyxzogax6vgyvag42y7464eywpf55gxi5qpoisibh3c5wa" // cidv1 - dag-pb - sha2-256
		shortStringCidV1Sha512      = "bafkrgqbqt3gerhas23vuzrapkdeqf4vu2dwxp3srdj6hvg6nhsug2tgyn6mj3u23yx7utftq3i2ckw2fwdh5qmhid5qf3t35yvkc5e5ottlw6"
	)

	t.Run("produced cid version: implicit default (CIDv0)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("produced cid version: follows user-set configuration Import.CidVersion=0", func(t *testing.T) {
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

	t.Run("produced cid multihash: follows user-set configuration in Import.HashFunction", func(t *testing.T) {
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

	t.Run("produced cid version: follows user-set configuration Import.CidVersion=1", func(t *testing.T) {
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

	t.Run("produced cid version: command flag overrides configuration in Import.CidVersion", func(t *testing.T) {
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

	t.Run("produced unixfs raw leaves: follows user-set configuration Import.UnixFSRawLeaves", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.UpdateConfig(func(cfg *config.Config) {
			// CIDv1 defaults to  raw-leaves=true
			cfg.Import.CidVersion = *config.NewOptionalInteger(1)
			// disable manually
			cfg.Import.UnixFSRawLeaves = config.False
		})
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1NoRawLeaves, cidStr)
	})

	t.Run("ipfs init --profile=legacy-cid-v0 sets config that produces legacy CIDv0", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=legacy-cid-v0")
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV0, cidStr)
	})

	t.Run("ipfs init --profile=legacy-cid-v0 applies UnixFSChunker=size-262144 and UnixFSFileMaxLinks", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=legacy-cid-v0")
		node.StartDaemon()
		defer node.StopDaemon()
		seed := "v0-seed"

		t.Run("under UnixFSFileMaxLinks=174", func(t *testing.T) {
			// Add 44544KiB file:
			// 174 * 256KiB should fit in single DAG layer
			cidStr := node.IPFSAddFromSeed("44544KiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 174, len(root.Links))
			// expect same CID every time
			require.Equal(t, "QmUbBALi174SnogsUzLpYbD4xPiBSFANF4iztWCsHbMKh2", cidStr)
		})

		t.Run("above UnixFSFileMaxLinks=174", func(t *testing.T) {
			// add 256KiB (one more block), it should force rebalancing DAG and moving most to second layer
			cidStr := node.IPFSAddFromSeed("44800KiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 2, len(root.Links))
			// expect same CID every time
			require.Equal(t, "QmepeWtdmS1hHXx1oZXsPUv6bMrfRRKfZcoPPU4eEfjnbf", cidStr)
		})
	})

	t.Run("ipfs init --profile=legacy-cid-v1 produces CIDv1 with raw leaves", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=legacy-cid-v1")
		node.StartDaemon()
		defer node.StopDaemon()

		cidStr := node.IPFSAddStr(shortString)
		require.Equal(t, shortStringCidV1, cidStr) // raw leaf
	})

	t.Run("ipfs init --profile=legacy-cid-v1 applies UnixFSChunker=size-1048576 and UnixFSFileMaxLinks", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=legacy-cid-v1")
		node.StartDaemon()
		defer node.StopDaemon()
		seed := "v1-seed"

		t.Run("under UnixFSFileMaxLinks=174", func(t *testing.T) {
			// Add 174MiB file:
			// 174 * 1MiB should fit in single layer
			cidStr := node.IPFSAddFromSeed("174MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 174, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeigwduxcf2aawppv3isnfeshnimkyplvw3hthxjhr2bdeje4tdaicu", cidStr)
		})

		t.Run("above UnixFSFileMaxLinks=174", func(t *testing.T) {
			// add +1MiB (one more block), it should force rebalancing DAG and moving most to second layer
			cidStr := node.IPFSAddFromSeed("175MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 2, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeidhd7lo2n2v7lta5yamob3xwhbxcczmmtmhquwhjesi35jntf7mpu", cidStr)
		})
	})

	t.Run("ipfs init --profile=test-cid-v1-2025-v35 applies UnixFSChunker=size-1048576 and UnixFSFileMaxLinks", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=test-cid-v1-2025-v35")
		node.StartDaemon()
		defer node.StopDaemon()
		seed := "v1-seed-1024"

		t.Run("under UnixFSFileMaxLinks=1024", func(t *testing.T) {
			// Add 174MiB file:
			// 1024 * 1MiB should fit in single layer
			cidStr := node.IPFSAddFromSeed("1024MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 1024, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeiej5w63ir64oxgkr5htqmlerh5k2rqflurn2howimexrlkae64xru", cidStr)
		})

		t.Run("above UnixFSFileMaxLinks=1024", func(t *testing.T) {
			// add +1MiB (one more block), it should force rebalancing DAG and moving most to second layer
			cidStr := node.IPFSAddFromSeed("1025MiB", seed)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 2, len(root.Links))
			// expect same CID every time
			require.Equal(t, "bafybeieilp2qx24pe76hxrxe6bpef5meuxto3kj5dd6mhb5kplfeglskdm", cidStr)
		})
	})
}
