package cli

import (
	_ "embed"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OpenSSL-generated PKCS#8 fixtures, used to prove Kubo reads and writes the
// same interoperable format. Regenerate with:
//
//	rsa:       openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048
//	ed25519:   openssl genpkey -algorithm ED25519
//	secp256k1: openssl ecparam -name secp256k1 -genkey -noout | openssl pkcs8 -topk8 -nocrypt
//	p256:      openssl ecparam -name prime256v1 -genkey -noout | openssl pkcs8 -topk8 -nocrypt
//
//go:embed testdata/openssl_rsa.pem
var opensslRSAKey []byte

//go:embed testdata/openssl_ed25519.pem
var opensslEd25519Key []byte

//go:embed testdata/openssl_secp256k1.pem
var opensslSecp256k1Key []byte

//go:embed testdata/openssl_p256.pem
var opensslP256Key []byte

// keyGenTypes are the algorithms `ipfs key gen` supports. They are also the
// only types `ipfs key import` accepts without --allow-any-key-type.
var keyGenTypes = []struct {
	typ     string
	genArgs []string // extra `key gen`/`key rotate` args (RSA needs a size)
}{
	{"rsa", []string{"--size=2048"}},
	{"ed25519", nil},
	{"secp256k1", nil},
}

// keyExportFormats are the on-disk formats export and import understand.
var keyExportFormats = []struct {
	name string
	ext  string
}{
	{"libp2p-protobuf-cleartext", "key"},
	{"pem-pkcs8-cleartext", "pem"},
}

func genKeyArgs(typ string, extra []string, name string) []string {
	args := []string{"key", "gen", "--type=" + typ}
	args = append(args, extra...)
	return append(args, name)
}

func TestKeyGen(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	for _, kt := range keyGenTypes {
		t.Run(kt.typ, func(t *testing.T) {
			gen := func(base, name string) string {
				args := []string{"key", "gen", "--type=" + kt.typ, "--ipns-base=" + base}
				args = append(args, kt.genArgs...)
				args = append(args, name)
				return node.IPFS(args...).Stdout.Trimmed()
			}

			// base36 CIDv1 PeerIDs start with "k"; the b58 multihash form
			// starts with Qm (rsa), 12D3 (ed25519), or 16Uiu (secp256k1).
			// key list renders IDs in whichever base is asked for, so check
			// each generated key against a listing in the same encoding.
			b36 := gen("base36", "b36-"+kt.typ)
			assert.True(t, strings.HasPrefix(b36, "k"), "base36 PeerID should start with k, got %q", b36)
			assert.Contains(t, node.IPFS("key", "list", "-l", "--ipns-base=base36").Stdout.String(), b36)

			b58 := gen("b58mh", "b58-"+kt.typ)
			assert.Regexp(t, `^(Qm|12D3|16Uiu)`, b58)
			assert.Contains(t, node.IPFS("key", "list", "-l", "--ipns-base=b58mh").Stdout.String(), b58)
		})
	}

	t.Run("default type is ed25519", func(t *testing.T) {
		// gen with no --type must keep using the ed25519 default
		id := node.IPFS("key", "gen", "--ipns-base=b58mh", "defaulted").Stdout.Trimmed()
		assert.Regexp(t, `^12D3`, id, "default key type should be ed25519")
	})

	t.Run("self identity is listed with its PeerID", func(t *testing.T) {
		self := node.IPFS("config", "Identity.PeerID").Stdout.Trimmed()
		list := node.IPFS("key", "list", "-l", "--ipns-base=b58mh").Stdout.String()
		assert.Regexp(t, regexp.QuoteMeta(self)+`\s+self`, list)
	})
}

func TestKeyImportExportRoundtrip(t *testing.T) {
	t.Parallel()

	for _, kt := range keyGenTypes {
		t.Run(kt.typ, func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()

			origID := node.IPFS(genKeyArgs(kt.typ, kt.genArgs, "orig")...).Stdout.Trimmed()
			require.NotEmpty(t, origID)

			for _, f := range keyExportFormats {
				t.Run(f.name, func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "key."+f.ext)
					node.IPFS("key", "export", "orig", "-f", f.name, "-o", path)

					importedID := node.IPFS("key", "import", "imported-"+f.ext, "-f", f.name, path).Stdout.Trimmed()
					assert.Equal(t, origID, importedID, "%s round-trip must preserve the key", f.name)
				})
			}
		})
	}
}

func TestKeyImportOpenSSL(t *testing.T) {
	t.Parallel()

	// PeerIDs (b58mh) are deterministic from each fixture's public key.
	fixtures := []struct {
		typ    string
		pem    []byte
		peerID string
	}{
		{"rsa", opensslRSAKey, "QmWqgbBdPWDUnqDWey5RoRbg98bEciLkoFwLg76xctpKgJ"},
		{"ed25519", opensslEd25519Key, "12D3KooWAcekXpuJjXdXUVKvtjYsa2WAY7hF5hX4csqGe4S434R6"},
		{"secp256k1", opensslSecp256k1Key, "16Uiu2HAmUnU2WFKDFtd9zxpedHkDb72z7stLDSj54uyvHpZeVMkp"},
	}

	for _, f := range fixtures {
		t.Run(f.typ, func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()
			dir := t.TempDir()
			src := filepath.Join(dir, "openssl.pem")
			require.NoError(t, os.WriteFile(src, f.pem, 0o600))

			// import: an OpenSSL-generated key loads and yields the expected PeerID
			id := node.IPFS("key", "import", "k", "-f", "pem-pkcs8-cleartext", "--ipns-base=b58mh", src).Stdout.Trimmed()
			assert.Equal(t, f.peerID, id)

			// export: Kubo writes back the exact bytes OpenSSL produced
			reexport := filepath.Join(dir, "reexport.pem")
			node.IPFS("key", "export", "k", "-f", "pem-pkcs8-cleartext", "-o", reexport)
			got, err := os.ReadFile(reexport)
			require.NoError(t, err)
			assert.Equal(t, f.pem, got, "Kubo PEM export must be byte-identical to the OpenSSL original")

			// the key is unchanged after a trip through the libp2p-protobuf format
			libp2p := filepath.Join(dir, "k.libp2p")
			node.IPFS("key", "export", "k", "-f", "libp2p-protobuf-cleartext", "-o", libp2p)
			node.IPFS("key", "rm", "k")
			node.IPFS("key", "import", "k", "-f", "libp2p-protobuf-cleartext", libp2p)
			node.IPFS("key", "export", "k", "-f", "pem-pkcs8-cleartext", "-o", reexport)
			got, err = os.ReadFile(reexport)
			require.NoError(t, err)
			assert.Equal(t, f.pem, got, "key must survive a libp2p-protobuf round-trip unchanged")
		})
	}
}

func TestKeyImportRestrictedType(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	// P-256 is a valid key OpenSSL makes but Kubo never generates, so import
	// refuses it unless the operator explicitly opts in.
	src := filepath.Join(t.TempDir(), "p256.pem")
	require.NoError(t, os.WriteFile(src, opensslP256Key, 0o600))

	res := node.RunIPFS("key", "import", "restricted", "-f", "pem-pkcs8-cleartext", src)
	assert.NotEqual(t, 0, res.ExitCode())
	assert.Contains(t, res.Stderr.String(), "only RSA, Ed25519, or Secp256k1")

	// with the opt-in flag the same key loads and yields its deterministic PeerID
	id := node.IPFS("key", "import", "restricted", "--allow-any-key-type", "-f", "pem-pkcs8-cleartext", "--ipns-base=b58mh", src).Stdout.Trimmed()
	assert.Equal(t, "QmQEEuDFa6ieFFaf3z6zeuRG5HU7vHWiY6AwkGimJ4Zre6", id)
}

func TestKeyRename(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	id := node.IPFS("key", "gen", "--type=ed25519", "before").Stdout.Trimmed()
	assert.Equal(t, "Key "+id+" renamed to after", node.IPFS("key", "rename", "before", "after").Stdout.Trimmed())

	list := node.IPFS("key", "list", "-l").Stdout.String()
	assert.Contains(t, list, "after")
	assert.NotContains(t, list, "before")
	assert.Contains(t, list, id, "rename must preserve the key's identity")

	// 'self' is reserved: it can be neither renamed nor overwritten
	renameSelf := node.RunIPFS("key", "rename", "self", "bar")
	assert.NotEqual(t, 0, renameSelf.ExitCode())
	assert.Contains(t, renameSelf.Stderr.String(), "cannot rename key with name")

	overwriteSelf := node.RunIPFS("key", "rename", "-f", "after", "self")
	assert.NotEqual(t, 0, overwriteSelf.ExitCode())
	assert.Contains(t, overwriteSelf.Stderr.String(), "cannot overwrite key with name")
}

func TestKeyRemove(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	node.IPFS("key", "gen", "--type=ed25519", "temp")
	require.Contains(t, node.IPFS("key", "list").Stdout.String(), "temp")

	node.IPFS("key", "rm", "temp")
	assert.NotContains(t, node.IPFS("key", "list").Stdout.String(), "temp")
}

func TestKeyReservedSelf(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	// the node's own identity ('self') is off-limits to gen, export, rm, and
	// import. Asserting the specific message matters: without it these would
	// pass for the wrong reason (e.g. export self already fails with "doesn't
	// exist" because self lives in the config, not the keystore).
	gen := node.RunIPFS("key", "gen", "--type=ed25519", "self")
	assert.NotEqual(t, 0, gen.ExitCode())
	assert.Contains(t, gen.Stderr.String(), "cannot create key with name 'self'")

	export := node.RunIPFS("key", "export", "self")
	assert.NotEqual(t, 0, export.ExitCode())
	assert.Contains(t, export.Stderr.String(), "cannot export key with name 'self'")

	rm := node.RunIPFS("key", "rm", "self")
	assert.NotEqual(t, 0, rm.ExitCode())
	assert.Contains(t, rm.Stderr.String(), "cannot remove key with name 'self'")

	path := filepath.Join(t.TempDir(), "real.key")
	node.IPFS("key", "gen", "--type=ed25519", "real")
	node.IPFS("key", "export", "real", "-o", path)
	imp := node.RunIPFS("key", "import", "self", path)
	assert.NotEqual(t, 0, imp.ExitCode())
	assert.Contains(t, imp.Stderr.String(), "cannot import key with name 'self'")
}

func TestKeyRotate(t *testing.T) {
	t.Parallel()

	// rotateBacksUpOldIdentity rotates a node to newType and asserts the old
	// identity survives, intact and usable, under the backup name.
	rotateBacksUpOldIdentity := func(t *testing.T, node *harness.Node, newType string, extra []string) {
		oldID := node.IPFS("config", "Identity.PeerID").Stdout.Trimmed()

		args := append([]string{"key", "rotate", "-o", "backup", "-t", newType}, extra...)
		node.IPFS(args...)

		newID := node.IPFS("config", "Identity.PeerID").Stdout.Trimmed()
		assert.NotEqual(t, oldID, newID, "rotate must change the node identity")

		// the backup entry must carry the pre-rotate PeerID, not just the name
		list := node.IPFS("key", "list", "-l", "--ipns-base=b58mh").Stdout.String()
		assert.Regexp(t, regexp.QuoteMeta(oldID)+`\s+backup`, list, "backup must hold the pre-rotate identity")

		// and it must be a real, exportable private key
		node.IPFS("key", "export", "backup", "-o", filepath.Join(t.TempDir(), "backup.key"))
	}

	// rotate a default (ed25519) identity to each supported type
	for _, kt := range keyGenTypes {
		t.Run("to "+kt.typ, func(t *testing.T) {
			t.Parallel()
			rotateBacksUpOldIdentity(t, harness.NewT(t).NewNode().Init(), kt.typ, kt.genArgs)
		})
	}

	// rotate away from non-default starting identities (the old key is read
	// and re-serialized, so its type is load-bearing)
	for _, from := range []string{"rsa", "secp256k1"} {
		t.Run("from "+from, func(t *testing.T) {
			t.Parallel()
			rotateBacksUpOldIdentity(t, harness.NewT(t).NewNode().Init("-a", from), "ed25519", nil)
		})
	}

	t.Run("backup name self is rejected", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		res := node.RunIPFS("key", "rotate", "-o", "self", "-t", "ed25519")
		assert.NotEqual(t, 0, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "cannot be named 'self'")
	})

	t.Run("rejected while the daemon is running", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon()
		defer node.StopDaemon()

		// rotating the identity under a live daemon must be refused
		res := node.RunIPFS("key", "rotate", "-o", "backup", "-t", "ed25519")
		assert.NotEqual(t, 0, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "daemon is running")
	})
}

func TestKeyOnlineExportImport(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()
	origID := node.IPFS("key", "gen", "--type=secp256k1", "live").Stdout.Trimmed()

	node.StartDaemon()
	defer node.StopDaemon()

	// export is read-only and works even while the daemon holds the repo
	path := filepath.Join(t.TempDir(), "live.pem")
	node.IPFS("key", "export", "live", "-f", "pem-pkcs8-cleartext", "-o", path)

	// import is routed to the running daemon and round-trips to the same key
	importedID := node.IPFS("key", "import", "copy", "-f", "pem-pkcs8-cleartext", path).Stdout.Trimmed()
	assert.Equal(t, origID, importedID)
}

func TestKeyExportNotAllowedOverHTTP(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()
	node.IPFS("key", "gen", "--type=ed25519", "secret")
	node.StartDaemon()
	defer node.StopDaemon()

	// key export is NoRemote: private key material must never leave over the API
	resp, err := http.Post(node.APIURL()+"/api/v0/key/export?arg=secret", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.NotContains(t, string(body), "PRIVATE KEY", "private key material must not be returned over the API")
}

func TestKeyExportFilePermissions(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not applicable on Windows")
	}

	node := harness.NewT(t).NewNode().Init()

	node.IPFS("key", "gen", "--type=ed25519", "testkey")

	t.Run("libp2p-protobuf-cleartext format", func(t *testing.T) {
		exportPath := filepath.Join(t.TempDir(), "testkey.key")
		node.IPFS("key", "export", "testkey", "-o", exportPath)

		info, err := os.Stat(exportPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"exported key file should have owner-only permissions")
	})

	t.Run("pem-pkcs8-cleartext format", func(t *testing.T) {
		exportPath := filepath.Join(t.TempDir(), "testkey.pem")
		node.IPFS("key", "export", "testkey", "-o", exportPath, "-f", "pem-pkcs8-cleartext")

		info, err := os.Stat(exportPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
			"exported PEM key file should have owner-only permissions")
	})
}

func TestKeyGenFixedSize(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	// ed25519 and secp256k1 keys are always 256 bits. key gen accepts --size
	// only when it matches; a mismatched value is an error, so a user asking
	// for 2048 bits is told no rather than handed a 256-bit key.
	for _, keyType := range []string{"ed25519", "secp256k1"} {
		t.Run(keyType, func(t *testing.T) {
			matched := node.IPFS("key", "gen", "--type="+keyType, "--size=256", "matched-"+keyType).Stdout.Trimmed()
			assert.NotEmpty(t, matched)

			res := node.RunIPFS("key", "gen", "--type="+keyType, "--size=2048", "mismatched-"+keyType)
			assert.NotEqual(t, 0, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "invalid key size 2048: "+keyType+" keys are always 256 bits")
		})
	}
}
