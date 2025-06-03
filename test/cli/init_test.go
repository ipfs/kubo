package cli

import (
	"fmt"
	"os"
	fp "path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
	pb "github.com/libp2p/go-libp2p/core/crypto/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validatePeerID(t *testing.T, peerID peer.ID, expErr error, expAlgo pb.KeyType) {
	assert.NoError(t, peerID.Validate())
	pub, err := peerID.ExtractPublicKey()
	assert.ErrorIs(t, expErr, err)
	if expAlgo != 0 {
		assert.Equal(t, expAlgo, pub.Type())
	}
}

func testInitAlgo(t *testing.T, initFlags []string, expOutputName string, expPeerIDPubKeyErr error, expPeerIDPubKeyType pb.KeyType) {
	t.Run("init", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode()
		initRes := node.IPFS(StrCat("init", initFlags)...)

		lines := []string{
			fmt.Sprintf("generating %s keypair...done", expOutputName),
			fmt.Sprintf("peer identity: %s", node.PeerID().String()),
			fmt.Sprintf("initializing IPFS node at %s\n", node.Dir),
		}
		expectedInitOutput := strings.Join(lines, "\n")
		assert.Equal(t, expectedInitOutput, initRes.Stdout.String())

		assert.DirExists(t, node.Dir)
		assert.FileExists(t, fp.Join(node.Dir, "config"))
		assert.DirExists(t, fp.Join(node.Dir, "datastore"))
		assert.DirExists(t, fp.Join(node.Dir, "blocks"))
		assert.NoFileExists(t, fp.Join(node.Dir, "._check_writeable"))

		_, err := os.ReadDir(node.Dir)
		assert.NoError(t, err, "ipfs dir should be listable")

		validatePeerID(t, node.PeerID(), expPeerIDPubKeyErr, expPeerIDPubKeyType)

		res := node.IPFS("config", "Mounts.IPFS")
		assert.Equal(t, "/ipfs", res.Stdout.Trimmed())

		catRes := node.RunIPFS("cat", fmt.Sprintf("/ipfs/%s/readme", CIDWelcomeDocs))
		assert.NotEqual(t, 0, catRes.ExitErr.ExitCode(), "welcome readme doesn't exist")
	})

	t.Run("init without empty repo", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode()
		initRes := node.IPFS(StrCat("init", "--empty-repo=false", initFlags)...)

		validatePeerID(t, node.PeerID(), expPeerIDPubKeyErr, expPeerIDPubKeyType)

		lines := []string{
			fmt.Sprintf("generating %s keypair...done", expOutputName),
			fmt.Sprintf("peer identity: %s", node.PeerID().String()),
			fmt.Sprintf("initializing IPFS node at %s", node.Dir),
			"to get started, enter:",
			fmt.Sprintf("\n\tipfs cat /ipfs/%s/readme\n\n", CIDWelcomeDocs),
		}
		expectedEmptyInitOutput := strings.Join(lines, "\n")
		assert.Equal(t, expectedEmptyInitOutput, initRes.Stdout.String())

		node.IPFS("cat", fmt.Sprintf("/ipfs/%s/readme", CIDWelcomeDocs))

		idRes := node.IPFS("id", "-f", "<aver>")
		version := node.IPFS("version", "-n").Stdout.Trimmed()
		assert.Contains(t, idRes.Stdout.String(), version)
	})
}

func TestInit(t *testing.T) {
	t.Parallel()

	t.Run("init fails if the repo dir has no perms", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode()
		badDir := fp.Join(node.Dir, ".badipfs")
		err := os.Mkdir(badDir, 0o000)
		require.NoError(t, err)

		res := node.RunIPFS("init", "--repo-dir", badDir)
		assert.NotEqual(t, 0, res.Cmd.ProcessState.ExitCode())
		assert.Contains(t, res.Stderr.String(), "permission denied")
	})

	t.Run("init with ed25519", func(t *testing.T) {
		t.Parallel()
		testInitAlgo(t, []string{"--algorithm=ed25519"}, "ED25519", nil, pb.KeyType_Ed25519)
	})

	t.Run("init with rsa", func(t *testing.T) {
		t.Parallel()
		testInitAlgo(t, []string{"--bits=2048", "--algorithm=rsa"}, "2048-bit RSA", peer.ErrNoPublicKey, 0)
	})

	t.Run("init with default algorithm", func(t *testing.T) {
		t.Parallel()
		testInitAlgo(t, []string{}, "ED25519", nil, pb.KeyType_Ed25519)
	})

	t.Run("ipfs init --profile with invalid profile fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode()
		res := node.RunIPFS("init", "--profile=invalid_profile")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Equal(t, "Error: invalid configuration profile: invalid_profile", res.Stderr.Trimmed())
	})

	t.Run("ipfs init --profile with valid profile succeeds", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode()
		node.IPFS("init", "--profile=server")
	})

	t.Run("ipfs config looks good", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init("--profile=server")

		lines := node.IPFS("config", "Swarm.AddrFilters").Stdout.Lines()
		assert.Len(t, lines, 18)

		out := node.IPFS("config", "Bootstrap").Stdout.Trimmed()
		assert.Equal(t, "[]", out)

		out = node.IPFS("config", "Addresses.API").Stdout.Trimmed()
		assert.Equal(t, "/ip4/127.0.0.1/tcp/0", out)
	})

	t.Run("ipfs init from existing config succeeds", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2)
		node1 := nodes[0]
		node2 := nodes[1]

		node1.Init("--profile=server")

		node2.IPFS("init", fp.Join(node1.Dir, "config"))
		out := node2.IPFS("config", "Addresses.API").Stdout.Trimmed()
		assert.Equal(t, "/ip4/127.0.0.1/tcp/0", out)
	})

	t.Run("ipfs init should not run while daemon is running", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		res := node.RunIPFS("init")
		assert.NotEqual(t, 0, res.ExitErr.ExitCode())
		assert.Contains(t, res.Stderr.String(), "Error: ipfs daemon is running. please stop it to run this command")
	})
}
