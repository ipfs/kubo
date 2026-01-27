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
//
// Each profile is tested against threshold boundaries to verify:
// - CID format (version, hash function, raw leaves vs dag-pb wrapped)
// - File chunking (UnixFSChunker size threshold)
// - DAG structure (UnixFSFileMaxLinks rebalancing threshold)
// - Directory sharding (HAMTThreshold for flat vs HAMT directories)
type cidProfileExpectations struct {
	// Profile identification
	Name        string   // canonical profile name from IPIP-499
	ProfileArgs []string // args to pass to ipfs init (empty for default behavior)

	// CID format expectations
	CIDVersion int    // 0 or 1
	HashFunc   string // e.g., "sha2-256"
	RawLeaves  bool   // true = raw codec for small files, false = dag-pb wrapped

	// File chunking expectations (UnixFSChunker config)
	ChunkSize      int    // chunk size in bytes (e.g., 262144 for 256KiB, 1048576 for 1MiB)
	ChunkSizeHuman string // human-readable chunk size (e.g., "256KiB", "1MiB")
	FileMaxLinks   int    // max links before DAG rebalancing (UnixFSFileMaxLinks config)

	// HAMT directory sharding expectations (UnixFSHAMTDirectory* config).
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

	// Expected deterministic CIDs for test vectors.
	// These serve as regression tests to detect unintended changes in CID generation.

	// SmallFileCID is the deterministic CID for "hello world" string.
	// Tests basic CID format (version, codec, hash).
	SmallFileCID string

	// FileAtChunkSizeCID is the deterministic CID for a file exactly at chunk size.
	// This file fits in a single block with no links:
	// - v0-2015: dag-pb wrapped TFile node (CIDv0)
	// - v1-2025: raw leaf block (CIDv1)
	FileAtChunkSizeCID string

	// FileOverChunkSizeCID is the deterministic CID for a file 1 byte over chunk size.
	// This file requires 2 chunks, producing a root dag-pb node with 2 links:
	// - v0-2015: links point to dag-pb wrapped TFile leaf nodes
	// - v1-2025: links point to raw leaf blocks
	FileOverChunkSizeCID string

	// FileAtMaxLinksCID is the deterministic CID for a file at UnixFSFileMaxLinks threshold.
	// File size = maxLinks * chunkSize, producing a single-layer DAG with exactly maxLinks children.
	FileAtMaxLinksCID string

	// FileOverMaxLinksCID is the deterministic CID for a file 1 byte over max links threshold.
	// The +1 byte requires an additional chunk, forcing DAG rebalancing to 2 layers.
	FileOverMaxLinksCID string

	// DirBasicCID is the deterministic CID for a directory exactly at HAMTThreshold.
	// With > comparison (not >=), directory at exact threshold stays as basic (flat) directory.
	DirBasicCID string

	// DirHAMTCID is the deterministic CID for a directory 1 byte over HAMTThreshold.
	// Crossing the threshold converts the directory to a HAMT sharded structure.
	DirHAMTCID string
}

// unixfsV02015 is the legacy profile for backward-compatible CID generation.
// Alias: legacy-cid-v0
var unixfsV02015 = cidProfileExpectations{
	Name:        "unixfs-v0-2015",
	ProfileArgs: []string{"--profile=unixfs-v0-2015"},

	CIDVersion: 0,
	HashFunc:   "sha2-256",
	RawLeaves:  false,

	ChunkSize:      262144, // 256 KiB
	ChunkSizeHuman: "256KiB",
	FileMaxLinks:   174,

	HAMTFanout:         256,
	HAMTThreshold:      262144, // 256 KiB
	HAMTSizeEstimation: "links",
	DirBasicNameLen:    30,   // 4096 * (30 + 34) = 262144 exactly at threshold
	DirBasicFiles:      4096, // 4096 * 64 = 262144 (stays basic with >)
	DirHAMTNameLen:     31,   // 4033 * (31 + 34) = 262145 exactly +1 over threshold
	DirHAMTLastNameLen: 0,    // 0 = same as DirHAMTNameLen (uniform filenames)
	DirHAMTFiles:       4033, // 4033 * 65 = 262145 (becomes HAMT)

	SmallFileCID:         "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD", // "hello world" dag-pb wrapped
	FileAtChunkSizeCID:   "QmWmRj3dFDZdb6ABvbmKhEL6TmPbAfBZ1t5BxsEyJrcZhE", // 262144 bytes with seed "chunk-v0-seed"
	FileOverChunkSizeCID: "QmYyLxtzZyW22zpoVAtKANLRHpDjZtNeDjQdJrcQNWoRkJ", // 262145 bytes with seed "chunk-v0-seed"
	FileAtMaxLinksCID:    "QmUbBALi174SnogsUzLpYbD4xPiBSFANF4iztWCsHbMKh2", // 174*256KiB bytes with seed "v0-seed"
	FileOverMaxLinksCID:  "QmV81WL765sC8DXsRhE5fJv2rwhS4icHRaf3J9Zk5FdRnW", // 174*256KiB+1 bytes with seed "v0-seed"
	DirBasicCID:          "QmX5GtRk3TSSEHtdrykgqm4eqMEn3n2XhfkFAis5fjyZmN", // 4096 files at threshold
	DirHAMTCID:           "QmeMiJzmhpJAUgynAcxTQYek5PPKgdv3qEvFsdV3XpVnvP", // 4033 files +1 over threshold
}

// unixfsV12025 is the recommended profile for cross-implementation CID determinism.
var unixfsV12025 = cidProfileExpectations{
	Name:        "unixfs-v1-2025",
	ProfileArgs: []string{"--profile=unixfs-v1-2025"},

	CIDVersion: 1,
	HashFunc:   "sha2-256",
	RawLeaves:  true,

	ChunkSize:      1048576, // 1 MiB
	ChunkSizeHuman: "1MiB",
	FileMaxLinks:   1024,

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

	SmallFileCID:         "bafkreifzjut3te2nhyekklss27nh3k72ysco7y32koao5eei66wof36n5e", // "hello world" raw leaf
	FileAtChunkSizeCID:   "bafkreiacndfy443ter6qr2tmbbdhadvxxheowwf75s6zehscklu6ezxmta", // 1048576 bytes with seed "chunk-v1-seed"
	FileOverChunkSizeCID: "bafybeigmix7t42i6jacydtquhet7srwvgpizfg7gjbq7627d35mjomtu64", // 1048577 bytes with seed "chunk-v1-seed"
	FileAtMaxLinksCID:    "bafybeihmf37wcuvtx4hpu7he5zl5qaf2ineo2lqlfrapokkm5zzw7zyhvm", // 1024*1MiB bytes with seed "v1-2025-seed"
	FileOverMaxLinksCID:  "bafybeibdsi225ugbkmpbdohnxioyab6jsqrmkts3twhpvfnzp77xtzpyhe", // 1024*1MiB+1 bytes with seed "v1-2025-seed"
	DirBasicCID:          "bafybeic3h7rwruealwxkacabdy45jivq2crwz6bufb5ljwupn36gicplx4", // 4766 files at 262144 bytes (threshold)
	DirHAMTCID:           "bafybeiegvuterwurhdtkikfhbxcldohmxp566vpjdofhzmnhv6o4freidu", // 4766 files at 262145 bytes (+1 over)
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
// Tests verify threshold behaviors for:
// - Small files (CID format verification)
// - UnixFSChunker threshold (single block vs multi-block)
// - UnixFSFileMaxLinks threshold (single-layer vs rebalanced DAG)
// - HAMTThreshold (basic flat directory vs HAMT sharded)
func runProfileTests(t *testing.T, exp cidProfileExpectations, carOutputDir string, exportCARs bool) {
	cidLen := cidV0Length
	if exp.CIDVersion == 1 {
		cidLen = cidV1Length
	}

	// Test: small file produces correct CID format
	// Verifies the profile sets the expected CID version, hash function, and leaf encoding.
	t.Run("small file produces correct CID format", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// Use "hello world" for determinism
		cidStr := node.IPFSAddStr("hello world")

		// Verify CID version (v0 starts with "Qm", v1 with "b")
		verifyCIDVersion(t, node, cidStr, exp.CIDVersion)

		// Verify hash function (sha2-256 for both profiles)
		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		// Verify raw leaves vs dag-pb wrapped
		// - v0-2015: dag-pb codec (wrapped)
		// - v1-2025: raw codec (raw leaves)
		verifyRawLeaves(t, node, cidStr, exp.RawLeaves)

		// Verify deterministic CID matches expected value
		if exp.SmallFileCID != "" {
			require.Equal(t, exp.SmallFileCID, cidStr, "expected deterministic CID for small file")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_small-file.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	// Test: file at UnixFSChunker threshold (single block)
	// A file exactly at chunk size fits in one block with no links.
	// - v0-2015 (256KiB): produces dag-pb wrapped TFile node
	// - v1-2025 (1MiB): produces raw leaf block
	t.Run("file at UnixFSChunker threshold (single block)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// File exactly at chunk size = single block (no links)
		seed := chunkSeedForProfile(exp)
		cidStr := node.IPFSAddDeterministicBytes(int64(exp.ChunkSize), seed)

		// Verify block structure based on raw leaves setting
		if exp.RawLeaves {
			// v1-2025: single block is a raw leaf (no dag-pb structure)
			codec := node.IPFS("cid", "format", "-f", "%c", cidStr).Stdout.Trimmed()
			require.Equal(t, "raw", codec, "single block file is raw leaf")
		} else {
			// v0-2015: single block is a dag-pb node with no links (TFile type)
			root, err := node.InspectPBNode(cidStr)
			assert.NoError(t, err)
			require.Equal(t, 0, len(root.Links), "single block file has no links")
			fsType, err := node.UnixFSDataType(cidStr)
			require.NoError(t, err)
			require.Equal(t, ft.TFile, fsType, "single block file is dag-pb wrapped (TFile)")
		}

		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		if exp.FileAtChunkSizeCID != "" {
			require.Equal(t, exp.FileAtChunkSizeCID, cidStr, "expected deterministic CID for file at chunk size")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_file-at-chunk-size.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	// Test: file 1 byte over UnixFSChunker threshold (2 blocks)
	// A file 1 byte over chunk size requires 2 chunks.
	// Root is a dag-pb node with 2 links. Leaf encoding depends on profile:
	// - v0-2015: leaf blocks are dag-pb wrapped TFile nodes
	// - v1-2025: leaf blocks are raw codec blocks
	t.Run("file 1 byte over UnixFSChunker threshold (2 blocks)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// File +1 byte over chunk size = 2 blocks
		seed := chunkSeedForProfile(exp)
		cidStr := node.IPFSAddDeterministicBytes(int64(exp.ChunkSize)+1, seed)

		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.Equal(t, 2, len(root.Links), "file over chunk size has 2 links")

		// Verify leaf block encoding
		for _, link := range root.Links {
			if exp.RawLeaves {
				// v1-2025: leaves are raw blocks
				leafCodec := node.IPFS("cid", "format", "-f", "%c", link.Hash.Slash).Stdout.Trimmed()
				require.Equal(t, "raw", leafCodec, "leaf blocks are raw, not dag-pb")
			} else {
				// v0-2015: leaves are dag-pb wrapped (TFile type)
				leafType, err := node.UnixFSDataType(link.Hash.Slash)
				require.NoError(t, err)
				require.Equal(t, ft.TFile, leafType, "leaf blocks are dag-pb wrapped (TFile)")
			}
		}

		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		if exp.FileOverChunkSizeCID != "" {
			require.Equal(t, exp.FileOverChunkSizeCID, cidStr, "expected deterministic CID for file over chunk size")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_file-over-chunk-size.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	// Test: file at UnixFSFileMaxLinks threshold (single layer)
	// A file of exactly maxLinks * chunkSize bytes fits in a single DAG layer.
	// - v0-2015: 174 links (174 * 256KiB = ~44.6MiB)
	// - v1-2025: 1024 links (1024 * 1MiB = 1GiB)
	t.Run("file at UnixFSFileMaxLinks threshold (single layer)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// File size = maxLinks * chunkSize (exactly at threshold)
		fileSize := fileAtMaxLinksBytes(exp)
		seed := seedForProfile(exp)
		cidStr := node.IPFSAddDeterministicBytes(fileSize, seed)

		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.Equal(t, exp.FileMaxLinks, len(root.Links),
			"expected exactly %d links at max", exp.FileMaxLinks)

		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		if exp.FileAtMaxLinksCID != "" {
			require.Equal(t, exp.FileAtMaxLinksCID, cidStr, "expected deterministic CID for file at max links")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_file-at-max-links.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	// Test: file 1 byte over UnixFSFileMaxLinks threshold (rebalanced DAG)
	// Adding 1 byte requires an additional chunk, exceeding maxLinks.
	// This triggers DAG rebalancing: chunks are grouped into intermediate nodes,
	// producing a 2-layer DAG with 2 links at the root.
	t.Run("file 1 byte over UnixFSFileMaxLinks threshold (rebalanced DAG)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// +1 byte over max links threshold triggers DAG rebalancing
		fileSize := fileOverMaxLinksBytes(exp)
		seed := seedForProfile(exp)
		cidStr := node.IPFSAddDeterministicBytes(fileSize, seed)

		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.Equal(t, 2, len(root.Links), "expected 2 links after DAG rebalancing")

		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		if exp.FileOverMaxLinksCID != "" {
			require.Equal(t, exp.FileOverMaxLinksCID, cidStr, "expected deterministic CID for rebalanced file")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_file-over-max-links.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s -> %s", cidStr, carPath)
		}
	})

	// Test: directory at HAMTThreshold (basic flat dir)
	// A directory exactly at HAMTThreshold stays as a basic (flat) UnixFS directory.
	// Threshold uses > comparison (not >=), so size == threshold stays basic.
	// Size estimation method depends on profile:
	// - v0-2015 "links": size = sum(nameLen + cidLen)
	// - v1-2025 "block": size = serialized protobuf block size
	t.Run("directory at HAMTThreshold (basic flat dir)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// Use consistent seed for deterministic CIDs
		seed := hamtSeedForProfile(exp)
		randDir, err := os.MkdirTemp(node.Dir, seed)
		require.NoError(t, err)

		// Create basic (flat) directory exactly at threshold
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

		// Verify UnixFS type is TDirectory (1), not THAMTShard (5)
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.TDirectory, fsType, "expected basic directory (type=1) at exact threshold")

		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.Equal(t, exp.DirBasicFiles, len(root.Links),
			"expected basic directory with %d links", exp.DirBasicFiles)

		verifyHashFunction(t, node, cidStr, exp.HashFunc)

		// Verify size is exactly at threshold
		if exp.HAMTSizeEstimation == "block" {
			blockSize := getBlockSize(t, node, cidStr)
			require.Equal(t, exp.HAMTThreshold, blockSize,
				"expected basic directory block size to be exactly at threshold (%d), got %d", exp.HAMTThreshold, blockSize)
		}
		if exp.HAMTSizeEstimation == "links" {
			linksSize := 0
			for _, link := range root.Links {
				linksSize += len(link.Name) + cidLen
			}
			require.Equal(t, exp.HAMTThreshold, linksSize,
				"expected basic directory links size to be exactly at threshold (%d), got %d", exp.HAMTThreshold, linksSize)
		}

		if exp.DirBasicCID != "" {
			require.Equal(t, exp.DirBasicCID, cidStr, "expected deterministic CID for basic directory")
		}

		if exportCARs {
			carPath := filepath.Join(carOutputDir, exp.Name+"_dir-basic.car")
			require.NoError(t, node.IPFSDagExport(cidStr, carPath))
			t.Logf("exported: %s (%d files) -> %s", cidStr, exp.DirBasicFiles, carPath)
		}
	})

	// Test: directory 1 byte over HAMTThreshold (HAMT sharded)
	// A directory 1 byte over HAMTThreshold is converted to a HAMT sharded structure.
	// HAMT distributes entries across buckets using consistent hashing.
	// Root has at most HAMTFanout links (256), with entries distributed across buckets.
	t.Run("directory 1 byte over HAMTThreshold (HAMT sharded)", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init(exp.ProfileArgs...)
		node.StartDaemon()
		defer node.StopDaemon()

		// Use consistent seed for deterministic CIDs
		seed := hamtSeedForProfile(exp)
		randDir, err := os.MkdirTemp(node.Dir, seed)
		require.NoError(t, err)

		// Create HAMT (sharded) directory exactly +1 byte over threshold
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

		// Verify UnixFS type is THAMTShard (5), not TDirectory (1)
		fsType, err := node.UnixFSDataType(cidStr)
		require.NoError(t, err)
		require.Equal(t, ft.THAMTShard, fsType, "expected HAMT directory (type=5) when over threshold")

		// HAMT root has at most fanout links (actual count depends on hash distribution)
		root, err := node.InspectPBNode(cidStr)
		assert.NoError(t, err)
		require.LessOrEqual(t, len(root.Links), exp.HAMTFanout,
			"expected HAMT directory root to have <= %d links", exp.HAMTFanout)

		verifyHashFunction(t, node, cidStr, exp.HashFunc)

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

// fileAtMaxLinksBytes returns the file size in bytes that produces exactly FileMaxLinks chunks.
func fileAtMaxLinksBytes(exp cidProfileExpectations) int64 {
	return int64(exp.FileMaxLinks) * int64(exp.ChunkSize)
}

// fileOverMaxLinksBytes returns the file size in bytes that triggers DAG rebalancing (+1 byte over max links threshold).
func fileOverMaxLinksBytes(exp cidProfileExpectations) int64 {
	return int64(exp.FileMaxLinks)*int64(exp.ChunkSize) + 1
}

// seedForProfile returns the deterministic seed used in add_test.go for file max links tests.
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

// chunkSeedForProfile returns the deterministic seed for chunk threshold tests.
func chunkSeedForProfile(exp cidProfileExpectations) string {
	switch exp.Name {
	case "unixfs-v0-2015", "default":
		return "chunk-v0-seed"
	case "unixfs-v1-2025":
		return "chunk-v1-seed"
	default:
		return "chunk-" + exp.Name + "-seed"
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
