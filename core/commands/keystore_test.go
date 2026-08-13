package commands

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteExportedKeyDoesNotFollowSymlinkToCharacterDevice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and character-device semantics are not applicable on Windows")
	}

	path := filepath.Join(t.TempDir(), "key")
	require.NoError(t, os.Symlink(os.DevNull, path))

	exportedKey := []byte("private key")
	require.NoError(t, writeExportedKey(path, strings.NewReader(string(exportedKey)), keyFormatLibp2pCleartextOption))

	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "atomic export should replace the symlink itself")

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, exportedKey, contents)
}

func TestWriteExportedKeyPreservesExistingFileOnReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	for _, format := range []string{keyFormatLibp2pCleartextOption, keyFormatPemCleartextOption} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "existing.key")
			existingContents := []byte("existing contents")
			require.NoError(t, os.WriteFile(path, existingContents, 0o644))

			err := writeExportedKey(path, iotest.ErrReader(readErr), format)
			require.ErrorIs(t, err, readErr)

			contents, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, existingContents, contents)

			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())

			tempFiles, err := filepath.Glob(filepath.Join(dir, ".tmp-existing.key*"))
			require.NoError(t, err)
			assert.Empty(t, tempFiles)
		})
	}
}

func TestSecp256k1PKCS8RoundTrip(t *testing.T) {
	t.Parallel()

	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err)

	der, err := marshalSecp256k1PrivateKey(priv.(*crypto.Secp256k1PrivateKey))
	require.NoError(t, err)
	require.True(t, isSecp256k1PKCS8(der))

	parsed, err := parsePKCS8PrivateKey(der)
	require.NoError(t, err)

	sk, _, err := crypto.KeyPairFromStdKey(parsed)
	require.NoError(t, err)
	assert.True(t, priv.Equals(sk))
}

func TestParseSecp256k1PrivateKeyRejectsInvalid(t *testing.T) {
	t.Parallel()

	priv, _, err := crypto.GenerateSecp256k1Key(rand.Reader)
	require.NoError(t, err)
	valid, err := marshalSecp256k1PrivateKey(priv.(*crypto.Secp256k1PrivateKey))
	require.NoError(t, err)

	rewrap := func(ec ecPrivateKey) []byte {
		var wrapper pkcs8Key
		_, err := asn1.Unmarshal(valid, &wrapper)
		require.NoError(t, err)
		ecDER, err := asn1.Marshal(ec)
		require.NoError(t, err)
		wrapper.PrivateKey = ecDER
		der, err := asn1.Marshal(wrapper)
		require.NoError(t, err)
		return der
	}

	t.Run("wrong EC version", func(t *testing.T) {
		_, err := parseSecp256k1PrivateKey(rewrap(ecPrivateKey{Version: 2, PrivateKey: make([]byte, 32)}))
		assert.ErrorContains(t, err, "version")
	})

	t.Run("oversized private key", func(t *testing.T) {
		_, err := parseSecp256k1PrivateKey(rewrap(ecPrivateKey{Version: 1, PrivateKey: make([]byte, 33)}))
		assert.ErrorContains(t, err, "length")
	})

	t.Run("zero scalar", func(t *testing.T) {
		_, err := parseSecp256k1PrivateKey(rewrap(ecPrivateKey{Version: 1, PrivateKey: make([]byte, 32)}))
		assert.ErrorContains(t, err, "valid range")
	})

	t.Run("scalar above the curve order", func(t *testing.T) {
		overflow := make([]byte, 32)
		for i := range overflow {
			overflow[i] = 0xff
		}
		_, err := parseSecp256k1PrivateKey(rewrap(ecPrivateKey{Version: 1, PrivateKey: overflow}))
		assert.ErrorContains(t, err, "valid range")
	})
}

func TestParsePKCS8PrivateKeyFallsBackToX509(t *testing.T) {
	t.Parallel()

	// ed25519 keys must keep going through crypto/x509
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	stdKey, err := crypto.PrivKeyToStdKey(priv)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(*stdKey.(*ed25519.PrivateKey))
	require.NoError(t, err)
	require.False(t, isSecp256k1PKCS8(der))

	parsed, err := parsePKCS8PrivateKey(der)
	require.NoError(t, err)
	edKey, ok := parsed.(ed25519.PrivateKey)
	require.True(t, ok)

	sk, _, err := crypto.KeyPairFromStdKey(&edKey)
	require.NoError(t, err)
	assert.True(t, priv.Equals(sk))
}
