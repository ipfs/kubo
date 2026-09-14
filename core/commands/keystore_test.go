package commands

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"net"
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

const exportedKeyContents = "private key"

func exportKey(t *testing.T, path string) error {
	t.Helper()
	return writeExportedKey(path, strings.NewReader(exportedKeyContents), keyFormatLibp2pCleartextOption)
}

// Exporting to /dev/null, a terminal or a pipe writes to the device itself,
// including when the path given is a symlink to one.
func TestWriteExportedKeyToCharacterDevice(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and character-device semantics are not applicable on Windows")
	}

	require.NoError(t, exportKey(t, os.DevNull))

	path := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(os.DevNull, path))
	require.NoError(t, exportKey(t, path))

	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "symlink to a device must survive the export")
}

// A path that points at a regular file through a symlink, such as a backup
// directory symlinked into a mounted volume, is written where the link points.
func TestWriteExportedKeyFollowsSymlinkToRegularFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink semantics are not applicable on Windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target.key")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))
	link := filepath.Join(dir, "link.key")
	require.NoError(t, os.Symlink(target, link))

	require.NoError(t, exportKey(t, link))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "the symlink must survive the export")

	contents, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, exportedKeyContents, string(contents))

	info, err = os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(exportedKeyFileMode), info.Mode().Perm())
}

// A symlink is often created before the file it points at, for example to send
// the key to a volume that is not mounted yet. The key belongs where the link
// points, and the link itself must survive.
func TestWriteExportedKeyFollowsDanglingSymlink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink semantics are not applicable on Windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "volume", "target.key")
	require.NoError(t, os.Mkdir(filepath.Dir(target), 0o755))
	link := filepath.Join(dir, "link.key")
	require.NoError(t, os.Symlink(target, link))

	require.NoError(t, exportKey(t, link))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "the symlink must survive the export")

	contents, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, exportedKeyContents, string(contents))
}

// A relative symlink points at a file next to itself, and the directory it
// lives in may be reached through a symlink of its own, such as a keys
// directory that points at another volume. The key belongs where the system
// resolves the link, not where joining the target onto the path as typed
// happens to land.
func TestWriteExportedKeyFollowsRelativeSymlinkThroughLinkedDir(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink semantics are not applicable on Windows")
	}

	dir := t.TempDir()
	volume := filepath.Join(dir, "volume")
	require.NoError(t, os.MkdirAll(filepath.Join(volume, "keys"), 0o755))
	require.NoError(t, os.Symlink("../target.key", filepath.Join(volume, "keys", "link.key")))
	require.NoError(t, os.Symlink(filepath.Join(volume, "keys"), filepath.Join(dir, "keys")))

	// Named like the link target, but one directory up from where the link
	// resolves, so a lexical join lands on it.
	bystander := filepath.Join(dir, "target.key")
	require.NoError(t, os.WriteFile(bystander, []byte("unrelated"), 0o644))

	require.NoError(t, exportKey(t, filepath.Join(dir, "keys", "link.key")))

	contents, err := os.ReadFile(filepath.Join(volume, "target.key"))
	require.NoError(t, err)
	assert.Equal(t, exportedKeyContents, string(contents))

	contents, err = os.ReadFile(bystander)
	require.NoError(t, err)
	assert.Equal(t, "unrelated", string(contents), "the export must not replace a file the link does not point at")
}

// Targets that can neither be replaced by rename nor written as a stream are
// refused instead of failing halfway through writing the key.
func TestWriteExportedKeyRefusesUnsupportedTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := exportKey(t, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), dir)

	if runtime.GOOS != "windows" {
		// Not t.TempDir(): its name is long enough to overrun the 104 byte
		// socket path limit on macOS.
		socketDir, err := os.MkdirTemp("", "s")
		require.NoError(t, err)
		defer os.RemoveAll(socketDir)

		socket := filepath.Join(socketDir, "socket")
		listener, err := net.Listen("unix", socket)
		require.NoError(t, err)
		defer listener.Close()

		err = exportKey(t, socket)
		require.Error(t, err)
		assert.Contains(t, err.Error(), socket)
	}
}

// Failures name the file the user asked for.
func TestWriteExportedKeyErrorNamesTarget(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing-dir", "key")
	err := exportKey(t, path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
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
			// os.WriteFile applies the umask, the assertion below does not.
			require.NoError(t, os.Chmod(path, 0o644))

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
