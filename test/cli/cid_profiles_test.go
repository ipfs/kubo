package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cidProfileExpectations defines expected behaviors for a UnixFS import profile.
// This allows DRY testing of multiple profiles with the same test logic.
type cidProfileExpectations struct {
	// Profile identification
	Name        string   // canonical profile name from IPIP-499
	ProfileArgs []string // args to pass to ipfs init (empty for default behavior)

	// CID format expectations
	CIDVersion int    // 0 or 1
	HashFunc   string // e.g., "sha2-256"
	RawLeaves  bool   // true = raw codec for small files, false = dag-pb wrapped

	// File chunking expectations
	ChunkSize    string // e.g., "1MiB" or "256KiB"
	FileMaxLinks int    // max links before DAG rebalancing

	// HAMT directory sharding expectations.
	// Threshold behavior: boxo converts to HAMT when size > HAMTThreshold (not >=).
	// This means a directory exactly at the threshold stays as a basic (flat) directory.
	HAMTFanout         int    // max links per HAMT shard bucket (256)
	HAMTThreshold      int    // sharding threshold in bytes (262144 = 256 KiB)
	HAMTSizeEstimation string // "block" (protobuf size) or "links" (legacy name+cid)

	// Test vector parameters for threshold boundary tests.
	// - DirBasic: size == threshold (stays basic)
	// - DirHAMT: size > threshold (converts to HAMT)
	// For block estimation, last filename length is adjusted to hit exact thresholds.
	DirBasicNameLen     int // filename length for basic directory (files 0 to N-2)
	DirBasicLastNameLen int // filename length for last file (0 = same as DirBasicNameLen)
	DirBasicFiles       int // file count for basic directory (at exact threshold)
	DirHAMTNameLen      int // filename length for HAMT directory (files 0 to N-2)
	DirHAMTLastNameLen  int // filename length for last file (0 = same as DirHAMTNameLen)
	DirHAMTFiles        int // total file count for HAMT directory (over threshold)

	// Expected deterministic CIDs for test vectors
	SmallFileCID        string // CID for single byte "x"
	FileAtMaxLinksCID   string // CID for file at max links
	FileOverMaxLinksCID string // CID for file triggering rebalance
	DirBasicCID         string // CID for basic directory (at exact threshold, stays flat)
	DirHAMTCID          string // CID for HAMT directory (over threshold, sharded)
}

// unixfsV02015 is the legacy profile for backward-compatible CID generation.
// Alias: legacy-cid-v0
var unixfsV02015 = cidProfileExpectations{
	Name:        "unixfs-v0-2015",
	ProfileArgs: []string{"--profile=unixfs-v0-2015"},

	CIDVersion: 0,
	HashFunc:   "sha2-256",
	RawLeaves:  false,

	ChunkSize:    "256KiB",
	FileMaxLinks: 174,

	HAMTFanout:         256,
	HAMTThreshold:      262144, // 256 KiB
	HAMTSizeEstimation: "links",
	DirBasicNameLen:    30,   // 4096 * (30 + 34) = 262144 exactly at threshold
	DirBasicFiles:      4096, // 4096 * 64 = 262144 (stays basic with >)
	DirHAMTNameLen:     31,   // 4033 * (31 + 34) = 262145 exactly +1 over threshold
	DirHAMTLastNameLen: 0,    // 0 = same as DirHAMTNameLen (uniform filenames)
	DirHAMTFiles:       4033, // 4033 * 65 = 262145 (becomes HAMT)

	SmallFileCID:        "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD", // "hello world" dag-pb wrapped
	FileAtMaxLinksCID:   "QmUbBALi174SnogsUzLpYbD4xPiBSFANF4iztWCsHbMKh2", // 44544KiB with seed "v0-seed"
	FileOverMaxLinksCID: "QmepeWtdmS1hHXx1oZXsPUv6bMrfRRKfZcoPPU4eEfjnbf", // 44800KiB with seed "v0-seed"
	DirBasicCID:         "QmX5GtRk3TSSEHtdrykgqm4eqMEn3n2XhfkFAis5fjyZmN", // 4096 files at threshold
	DirHAMTCID:          "QmeMiJzmhpJAUgynAcxTQYek5PPKgdv3qEvFsdV3XpVnvP", // 4033 files +1 over threshold
}

// unixfsV12025 is the recommended profile for cross-implementation CID determinism.
var unixfsV12025 = cidProfileExpectations{
	Name:        "unixfs-v1-2025",
	ProfileArgs: []string{"--profile=unixfs-v1-2025"},

	CIDVersion: 1,
	HashFunc:   "sha2-256",
	RawLeaves:  true,

	ChunkSize:    "1MiB",
	FileMaxLinks: 1024,

	HAMTFanout:         256,
	HAMTThreshold:      262144, // 256 KiB
	HAMTSizeEstimation: "block",
	// Block size = numFiles * linkSize + 4 bytes overhead
	// LinkSerializedSize(11, 36, 1) = 55, LinkSerializedSize(21, 36, 1) = 65, LinkSerializedSize(22, 36, 1) = 66
	DirBasicNameLen:     11,   // 4765 files * 55 bytes
	DirBasicLastNameLen: 21,   // last file: 65 bytes; total: 4765*55 + 65 + 4 = 262144 (at threshold)
	DirBasicFiles:       4766, // stays basic with > comparison
	DirHAMTNameLen:      11,   // 4765 files * 55 bytes
	DirHAMTLastNameLen:  22,   // last file: 66 bytes; total: 4765*55 + 66 + 4 = 262145 (+1 over threshold)
	DirHAMTFiles:        4766, // becomes HAMT

	SmallFileCID:        "bafkreifzjut3te2nhyekklss27nh3k72ysco7y32koao5eei66wof36n5e", // "hello world" raw leaf
	FileAtMaxLinksCID:   "bafybeihmf37wcuvtx4hpu7he5zl5qaf2ineo2lqlfrapokkm5zzw7zyhvm", // 1024MiB with seed "v1-2025-seed"
	FileOverMaxLinksCID: "bafybeihmzokxxjqwxjcryerhp5ezpcog2wcawfryb2xm64xiakgm4a5jue", // 1025MiB with seed "v1-2025-seed"
	DirBasicCID:         "bafybeic3h7rwruealwxkacabdy45jivq2crwz6bufb5ljwupn36gicplx4", // 4766 files at 262144 bytes (threshold)
	DirHAMTCID:          "bafybeiegvuterwurhdtkikfhbxcldohmxp566vpjdofhzmnhv6o4freidu", // 4766 files at 262145 bytes (+1 over)
}

// defaultProfile points to the profile that matches Kubo's implicit default behavior.
// Today this is unixfs-v0-2015. When Kubo changes defaults, update this pointer.
var defaultProfile = unixfsV02015

const (
	cidV0Length = 34 // CIDv0 sha2-256
	cidV1Length = 36 // CIDv1 sha2-256
)

// TestCIDProfiles generates deterministic test vectors for CID profile verification.
// Set CID_PROFILES_CAR_OUTPUT environment variable to export CAR files.
// Example: CID_PROFILES_CAR_OUTPUT=/tmp/cid-profiles go test -run TestCIDProfiles -v
func TestCIDProfiles(t *testing.T) {
	t.Parallel()

	carOutputDir := os.Getenv("CID_PROFILES_CAR_OUTPUT")
	exportCARs := carOutputDir != ""
	if exportCARs {
		if err := os.MkdirAll(carOutputDir, 0o755); err != nil {
			t.Fatalf("failed to create CAR output directory: %v", err)
		}
		t.Logf("CAR export enabled, writing to: %s", carOutputDir)
	}

	// Test both IPIP-499 profiles
	for _, profile := range []cidProfileExpectations{unixfsV02015, unixfsV12025} {
		t.Run(profile.Name, func(t *testing.T) {
			t.Parallel()
			runProfileTests(t, profile, carOutputDir, exportCARs)
		})
	}

	// Test default behavior (no profile specified)
	t.Run("default", func(t *testing.T) {
		t.Parallel()
		// Default behavior should match defaultProfile (currently unixfs-v0-2015)
		defaultExp := defaultProfile
		defaultExp.Name = "default"
		defaultExp.ProfileArgs = nil // no profile args = default behavior
		runProfileTests(t, defaultExp, carOutputDir, exportCARs)
	})
}

// runProfileTests runs all test vectors for a given profile.
func runProfileTests(t *testing.T, exp cidProfileExpectations, carOutputDir string, exportCARs bool) {
	cidLen := cidV0Length
	if exp.CIDVersion == 1 {
		cidLen = cidV1Length
	}

	t.Run("small-file", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// Use "hello world" for determinism - matches CIDs in add_test.go
		cidStr := node.IPFSAddStr("hello world")

		// Verify CID version
		verifyCIDVersion(t, node, cidStr, exp.CIDVersion)

		// Verify hash function
		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		// Verify raw leaves vs wrapped
		verifyRawLeaves(t, node, cidStr, exp.RawLeaves)

		// Verify deterministic CID if expected
		if exp.SmallFileCID != "" {
			require.Equal(t, exp.SmallFileCID, cidStr, "expected deterministic CID for small file")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_small-file.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	t.Run("file-at-max-links", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// Calculate file size: maxLinks * chunkSize
		fileSize := fileAtMaxLinksSize(exp)
		// Seed matches add_test.go for deterministic CIDs
		seed := seedForProfile(exp)
		cidStr := node.IPFSAddDeterministic(fileSize, seed)

		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.Equal(t, exp.FileMaxLinks, len(root.Links),
			"expected exactly %d links at max", exp.FileMaxLinks)

		// Verify hash function on root
		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		// Verify deterministic CID if expected
		if exp.FileAtMaxLinksCID != "" {
			require.Equal(t, exp.FileAtMaxLinksCID, cidStr, "expected deterministic CID for file at max links")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_file-at-max-links.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	t.Run("file-over-max-links-rebalanced", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// One more chunk triggers rebalancing
		fileSize := fileOverMaxLinksSize(exp)
		// Seed matches add_test.go for deterministic CIDs
		seed := seedForProfile(exp)
		cidStr := node.IPFSAddDeterministic(fileSize, seed)

		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.Equal(t, 2, len(root.Links), "expected 2 links after DAG rebalancing")

		// Verify hash function on root
		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		// Verify deterministic CID if expected
		if exp.FileOverMaxLinksCID != "" {
			require.Equal(t, exp.FileOverMaxLinksCID, cidStr, "expected deterministic CID for rebalanced file")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_file-over-max-links.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	t.Run("dir-basic", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// Use consistent seed for deterministic CIDs
		seed := hamtSeedForProfile(exp)
		randDir, err := os.MkdirTemp(node.Dir, seed)
		require.NoError(t, err)

		// Create basic (flat) directory exactly at threshold.
		// With > comparison, directory at exact threshold stays basic.
		basicLastNameLen := exp.DirBasicLastNameLen
		if basicLastNameLen == 0 {
			basicLastNameLen = exp.DirBasicNameLen
		}
		if exp.HAMTSizeEstimation == "block" {
			err = createDirectoryForHAMTBlockEstimation(randDir, exp.DirBasicFiles, exp.DirBasicNameLen, basicLastNameLen, seed)
		} else {
			err = createDirectoryForHAMTLinksEstimation(randDir, exp.DirBasicFiles, exp.DirBasicNameLen, basicLastNameLen, seed)
		}
		require.NoError(t, err)

		cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

		// Verify it's a basic directory by checking UnixFS type
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.TDirectory, fsType, "expected basic directory (type=1) at exact threshold")

		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.Equal(t, exp.DirBasicFiles, len(root.Links),
			"expected basic directory with %d links", exp.DirBasicFiles)

		// Verify hash function
		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		// Verify size is exactly at threshold
		if exp.HAMTSizeEstimation == "block" {
			// Block estimation: verify actual serialized block size
			blockSize := getBlockSize(t, node, cidStr)
			require.Equal(t, exp.HAMTThreshold, blockSize,
				"expected basic directory block size to be exactly at threshold (%d), got %d", exp.HAMTThreshold, blockSize)
		}
		if exp.HAMTSizeEstimation == "links" {
			// Links estimation: verify sum of (name_len + cid_len) for all links
			linksSize := 0
			for _, link := range root.Links {
				linksSize += len(link.Name) + cidLen
			}
			require.Equal(t, exp.HAMTThreshold, linksSize,
				"expected basic directory links size to be exactly at threshold (%d), got %d", exp.HAMTThreshold, linksSize)
		}

		// Verify deterministic CID
		if exp.DirBasicCID != "" {
			require.Equal(t, exp.DirBasicCID, cidStr, "expected deterministic CID for basic directory")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_dir-basic.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s (%d files) -> %s", cidStr, exp.DirBasicFiles, carPath)
		}
	})

	t.Run("dir-hamt", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// Use consistent seed for deterministic CIDs
		seed := hamtSeedForProfile(exp)
		randDir, err := os.MkdirTemp(node.Dir, seed)
		require.NoError(t, err)

		// Create HAMT (sharded) directory exactly +1 byte over threshold.
		// With > comparison, directory over threshold becomes HAMT.
		lastNameLen := exp.DirHAMTLastNameLen
		if lastNameLen == 0 {
			lastNameLen = exp.DirHAMTNameLen
		}
		if exp.HAMTSizeEstimation == "block" {
			err = createDirectoryForHAMTBlockEstimation(randDir, exp.DirHAMTFiles, exp.DirHAMTNameLen, lastNameLen, seed)
		} else {
			err = createDirectoryForHAMTLinksEstimation(randDir, exp.DirHAMTFiles, exp.DirHAMTNameLen, lastNameLen, seed)
		}
		require.NoError(t, err)

		cidStr := node.IPFS("add", "-r", "-Q", randDir).Stdout.Trimmed()

		// Verify it's a HAMT directory by checking UnixFS type
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "expected HAMT directory (type=5) when over threshold")

		// HAMT root has at most fanout links (actual count depends on hash distribution)
		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.LessOrEqual(t, len(root.Links), exp.HAMTFanout,
			"expected HAMT directory root to have <= %d links", exp.HAMTFanout)

		// Verify hash function
		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		// Verify deterministic CID
		if exp.DirHAMTCID != "" {
			require.Equal(t, exp.DirHAMTCID, cidStr, "expected deterministic CID for HAMT directory")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_dir-hamt.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s (%d files, HAMT root links: %d) -> %s",
				cidStr, exp.DirHAMTFiles, len(root.Links), carPath)
		}
	})
}

// verifyCIDVersion checks that the CID has the expected version.
func verifyCIDVersion(t *testing.T, _ *harness.Node, cidStr string, expectedVersion int) {
	t.Helper()
	if expectedVersion == 0 {
		require.True(t, strings.HasPrefix(cidStr, "Qm"),
			"expected CIDv0 (starts with Qm), got: %s", cidStr)
	} else {
		require.True(t, strings.HasPrefix(cidStr, "b"),
			"expected CIDv1 (base32, starts with b), got: %s", cidStr)
	}
}

// verifyHashFunction checks that the CID uses the expected hash function.
func verifyHashFunction(t *testing.T, node *harness.Node, cidStr, expectedHash string) {
	t.Helper()
	// Use ipfs cid format to get hash function info
	// Format string %h gives the hash function name
	res := node.IPFS("cid", "format", "-f", "%h", cidStr)
	hashFunc := strings.TrimSpace(res.Stdout.String())
	require.Equal(t, expectedHash, hashFunc,
		"expected hash function %s, got %s for CID %s", expectedHash, hashFunc, cidStr)
}

// verifyRawLeaves checks whether the CID represents a raw leaf or dag-pb wrapped block.
// For CIDv1: raw leaves have codec 0x55 (raw), wrapped have codec 0x70 (dag-pb).
// For CIDv0: always dag-pb (no raw leaves possible).
func verifyRawLeaves(t *testing.T, node *harness.Node, cidStr string, expectRaw bool) {
	t.Helper()
	// Use ipfs cid format to get codec info
	// Format string %c gives the codec name
	res := node.IPFS("cid", "format", "-f", "%c", cidStr)
	codec := strings.TrimSpace(res.Stdout.String())

	if expectRaw {
		require.Equal(t, "raw", codec,
			"expected raw codec for raw leaves, got %s for CID %s", codec, cidStr)
	} else {
		require.Equal(t, "dag-pb", codec,
			"expected dag-pb codec for wrapped leaves, got %s for CID %s", codec, cidStr)
	}
}

// getBlockSize returns the size of a block in bytes using ipfs block stat.
func getBlockSize(t *testing.T, node *harness.Node, cidStr string) int {
	t.Helper()
	res := node.IPFS("block", "stat", "--enc=json", cidStr)
	var stat struct {
		Size int `json:"Size"`
	}
	require.NoError(t, json.Unmarshal(res.Stdout.Bytes(), &stat))
	return stat.Size
}

// fileAtMaxLinksSize returns the file size that produces exactly FileMaxLinks chunks.
func fileAtMaxLinksSize(exp cidProfileExpectations) string {
	switch exp.ChunkSize {
	case "1MiB":
		return strings.Replace(exp.ChunkSize, "1MiB", "", 1) +
			string(rune('0'+exp.FileMaxLinks/1000)) +
			string(rune('0'+(exp.FileMaxLinks%1000)/100)) +
			string(rune('0'+(exp.FileMaxLinks%100)/10)) +
			string(rune('0'+exp.FileMaxLinks%10)) + "MiB"
	case "256KiB":
		// 174 * 256 KiB = 44544 KiB
		totalKiB := exp.FileMaxLinks * 256
		return intToStr(totalKiB) + "KiB"
	default:
		panic("unknown chunk size: " + exp.ChunkSize)
	}
}

// fileOverMaxLinksSize returns the file size that triggers DAG rebalancing.
func fileOverMaxLinksSize(exp cidProfileExpectations) string {
	switch exp.ChunkSize {
	case "1MiB":
		return intToStr(exp.FileMaxLinks+1) + "MiB"
	case "256KiB":
		// (174 + 1) * 256 KiB = 44800 KiB
		totalKiB := (exp.FileMaxLinks + 1) * 256
		return intToStr(totalKiB) + "KiB"
	default:
		panic("unknown chunk size: " + exp.ChunkSize)
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// seedForProfile returns the deterministic seed used in add_test.go for file tests.
func seedForProfile(exp cidProfileExpectations) string {
	switch exp.Name {
	case "unixfs-v0-2015", "default":
		return "v0-seed"
	case "unixfs-v1-2025":
		return "v1-2025-seed"
	default:
		return exp.Name + "-seed"
	}
}

// hamtSeedForProfile returns the deterministic seed for HAMT directory tests.
// Uses the same seed for both under/at threshold tests to ensure consistency.
func hamtSeedForProfile(exp cidProfileExpectations) string {
	switch exp.Name {
	case "unixfs-v0-2015", "default":
		return "hamt-unixfs-v0-2015"
	case "unixfs-v1-2025":
		return "hamt-unixfs-v1-2025"
	default:
		return "hamt-" + exp.Name
	}
}

// TestDefaultMatchesExpectedProfile verifies that default ipfs add behavior
// matches the expected profile (currently unixfs-v0-2015).
func TestDefaultMatchesExpectedProfile(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init()
	node.StartDaemon()
	defer node.StopDaemon()

	// Small file test
	cidDefault := node.IPFSAddStr("x")

	// Same file with explicit profile
	nodeWithProfile := harness.NewT(t).NewNode().Init(defaultProfile.ProfileArgs...)
	nodeWithProfile.StartDaemon()
	defer nodeWithProfile.StopDaemon()

	cidWithProfile := nodeWithProfile.IPFSAddStr("x")

	require.Equal(t, cidWithProfile, cidDefault,
		"default behavior should match %s profile", defaultProfile.Name)
}

// TestProtobufHelpers verifies the protobuf size calculation helpers.
func TestProtobufHelpers(t *testing.T) {
	t.Parallel()

	t.Run("VarintLen", func(t *testing.T) {
		// Varint encoding: 7 bits per byte, MSB indicates continuation
		cases := []struct {
			value    uint64
			expected int
		}{
			{0, 1},
			{127, 1},         // 0x7F - max 1-byte varint
			{128, 2},         // 0x80 - min 2-byte varint
			{16383, 2},       // 0x3FFF - max 2-byte varint
			{16384, 3},       // 0x4000 - min 3-byte varint
			{2097151, 3},     // 0x1FFFFF - max 3-byte varint
			{2097152, 4},     // 0x200000 - min 4-byte varint
			{268435455, 4},   // 0xFFFFFFF - max 4-byte varint
			{268435456, 5},   // 0x10000000 - min 5-byte varint
			{34359738367, 5}, // 0x7FFFFFFFF - max 5-byte varint
		}

		for _, tc := range cases {
			got := testutils.VarintLen(tc.value)
			require.Equal(t, tc.expected, got, "VarintLen(%d)", tc.value)
		}
	})

	t.Run("LinkSerializedSize", func(t *testing.T) {
		// Test typical cases for directory links
		cases := []struct {
			nameLen  int
			cidLen   int
			tsize    uint64
			expected int
		}{
			// 255-char name, CIDv0 (34 bytes), tsize=0
			// Inner: 1+1+34 + 1+2+255 + 1+1 = 296
			// Outer: 1 + 2 + 296 = 299
			{255, 34, 0, 299},
			// 255-char name, CIDv1 (36 bytes), tsize=0
			// Inner: 1+1+36 + 1+2+255 + 1+1 = 298
			// Outer: 1 + 2 + 298 = 301
			{255, 36, 0, 301},
			// Short name (10 chars), CIDv1, tsize=0
			// Inner: 1+1+36 + 1+1+10 + 1+1 = 52
			// Outer: 1 + 1 + 52 = 54
			{10, 36, 0, 54},
			// 255-char name, CIDv1, large tsize
			// Inner: 1+1+36 + 1+2+255 + 1+5 = 302 (tsize uses 5-byte varint)
			// Outer: 1 + 2 + 302 = 305
			{255, 36, 34359738367, 305},
		}

		for _, tc := range cases {
			got := testutils.LinkSerializedSize(tc.nameLen, tc.cidLen, tc.tsize)
			require.Equal(t, tc.expected, got, "LinkSerializedSize(%d, %d, %d)", tc.nameLen, tc.cidLen, tc.tsize)
		}
	})

	t.Run("EstimateFilesForBlockThreshold", func(t *testing.T) {
		threshold := 262144
		nameLen := 255
		cidLen := 36
		var tsize uint64 = 0

		numFiles := testutils.EstimateFilesForBlockThreshold(threshold, nameLen, cidLen, tsize)
		require.Equal(t, 870, numFiles, "expected 870 files for threshold 262144")

		numFilesUnder := testutils.EstimateFilesForBlockThreshold(threshold-1, nameLen, cidLen, tsize)
		require.Equal(t, 870, numFilesUnder, "expected 870 files for threshold 262143")

		numFilesOver := testutils.EstimateFilesForBlockThreshold(262185, nameLen, cidLen, tsize)
		require.Equal(t, 871, numFilesOver, "expected 871 files for threshold 262185")
	})
}
