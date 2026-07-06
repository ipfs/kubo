package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyExportFilePermissions(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not applicable on Windows")
	}

	node := harness.NewT(t).NewNode().Init()

	node.IPFS("key", "gen", "--type=ed25519", "testkey")

	t.Run("libp2p-protobuf-cleartext format", func(t *testing.T) {
		t.Parallel()
		exportPath := filepath.Join(t.TempDir(), "testkey.key")
		node.IPFS("key", "export", "testkey", "-o", exportPath)

		info, err := os.Stat(exportPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"exported key file should have owner-only permissions")
	})

	t.Run("pem-pkcs8-cleartext format", func(t *testing.T) {
		t.Parallel()
		exportPath := filepath.Join(t.TempDir(), "testkey.pem")
		node.IPFS("key", "export", "testkey", "-o", exportPath, "-f", "pem-pkcs8-cleartext")

		info, err := os.Stat(exportPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"exported PEM key file should have owner-only permissions")
	})
}

func TestKeySecp256k1(t *testing.T) {
	t.Parallel()

	// fixture produced by:
	//   openssl ecparam -name secp256k1 -genkey -noout | openssl pkcs8 -topk8 -nocrypt
	const opensslPEM = `-----BEGIN PRIVATE KEY-----
MIGEAgEAMBAGByqGSM49AgEGBSuBBAAKBG0wawIBAQQg8JXjM5UxXZAbsRh4YlXH
93qwRlt+qZ6rFXhhm27vcKShRANCAATvpVmYLz42SFb/wCogvXiWaNW/Jtjf+NmY
d417YkRfLfQovtY6OH++bZLXuZfF4cIDZ+2N3486dmbEqpbAxLLP
-----END PRIVATE KEY-----
`
	const opensslPeerID = "16Uiu2HAmUnU2WFKDFtd9zxpedHkDb72z7stLDSj54uyvHpZeVMkp"

	t.Run("key gen", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		keyID := node.IPFS("key", "gen", "--type=secp256k1", "generated").Stdout.Trimmed()
		require.NotEmpty(t, keyID)
		assert.Contains(t, node.IPFS("key", "list", "-l").Stdout.String(), keyID)
	})

	t.Run("pem export/import round-trip preserves the key", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		keyID := node.IPFS("key", "gen", "--type=secp256k1", "original").Stdout.Trimmed()
		exportPath := filepath.Join(t.TempDir(), "secp.pem")
		node.IPFS("key", "export", "original", "-o", exportPath, "-f", "pem-pkcs8-cleartext")

		importedID := node.IPFS("key", "import", "copy", "-f", "pem-pkcs8-cleartext", exportPath).Stdout.Trimmed()
		assert.Equal(t, keyID, importedID)
	})

	t.Run("import of an openssl-generated key", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		pemPath := filepath.Join(t.TempDir(), "openssl.pem")
		require.NoError(t, os.WriteFile(pemPath, []byte(opensslPEM), 0o600))

		keyID := node.IPFS("key", "import", "imported", "-f", "pem-pkcs8-cleartext", "--ipns-base=b58mh", pemPath).Stdout.Trimmed()
		assert.Equal(t, opensslPeerID, keyID)
	})

	t.Run("rotate to a secp256k1 identity", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		oldID := node.IPFS("config", "Identity.PeerID").Stdout.Trimmed()
		node.IPFS("key", "rotate", "-o", "backup", "-t", "secp256k1")

		newID := node.IPFS("config", "Identity.PeerID").Stdout.Trimmed()
		assert.NotEqual(t, oldID, newID)
		assert.Contains(t, node.IPFS("key", "list").Stdout.String(), "backup")
	})
}
