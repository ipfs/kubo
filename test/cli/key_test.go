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
