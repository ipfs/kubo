package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/assert"
)

func TestCidCommands(t *testing.T) {
	t.Parallel()

	t.Run("base32", testCidBase32)
	t.Run("format", testCidFormat)
	t.Run("bases", testCidBases)
	t.Run("codecs", testCidCodecs)
	t.Run("hashes", testCidHashes)
}

// testCidBase32 tests 'ipfs cid base32' subcommand
// Includes regression tests for https://github.com/ipfs/kubo/issues/9007
func testCidBase32(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()

	t.Run("converts valid CIDs to base32", func(t *testing.T) {
		t.Run("CIDv0 to base32", func(t *testing.T) {
			res := node.RunIPFS("cid", "base32", "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo")
			assert.Equal(t, 0, res.ExitCode())
			assert.Equal(t, "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa\n", res.Stdout.String())
		})

		t.Run("CIDv1 base58 to base32", func(t *testing.T) {
			res := node.RunIPFS("cid", "base32", "zdj7WgefqQm5HogBQ2bckZuTYYDarRTUZi51GYCnerHD2G86j")
			assert.Equal(t, 0, res.ExitCode())
			assert.Equal(t, "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa\n", res.Stdout.String())
		})

		t.Run("already base32 CID remains unchanged", func(t *testing.T) {
			res := node.RunIPFS("cid", "base32", "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa")
			assert.Equal(t, 0, res.ExitCode())
			assert.Equal(t, "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa\n", res.Stdout.String())
		})

		t.Run("multiple valid CIDs", func(t *testing.T) {
			res := node.RunIPFS("cid", "base32",
				"QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo",
				"bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa")
			assert.Equal(t, 0, res.ExitCode())
			assert.Empty(t, res.Stderr.String())
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			assert.Equal(t, 2, len(lines))
			assert.Equal(t, "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa", lines[0])
			assert.Equal(t, "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa", lines[1])
		})
	})

	t.Run("error handling", func(t *testing.T) {
		// Regression tests for https://github.com/ipfs/kubo/issues/9007
		t.Run("returns error code 1 for single invalid CID", func(t *testing.T) {
			res := node.RunIPFS("cid", "base32", "invalid-cid")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "invalid-cid: invalid cid")
			assert.Contains(t, res.Stderr.String(), "Error: errors while displaying some entries")
		})

		t.Run("returns error code 1 for mixed valid and invalid CIDs", func(t *testing.T) {
			res := node.RunIPFS("cid", "base32", "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo", "invalid-cid")
			assert.Equal(t, 1, res.ExitCode())
			// Valid CID should be converted and printed to stdout
			assert.Contains(t, res.Stdout.String(), "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa")
			// Invalid CID error should be printed to stderr
			assert.Contains(t, res.Stderr.String(), "invalid-cid: invalid cid")
			assert.Contains(t, res.Stderr.String(), "Error: errors while displaying some entries")
		})

		t.Run("returns error code 1 for stdin with invalid CIDs", func(t *testing.T) {
			input := "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo\nbad-cid\nbafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa"
			res := node.RunPipeToIPFS(strings.NewReader(input), "cid", "base32")
			assert.Equal(t, 1, res.ExitCode())
			// Valid CIDs should be converted
			assert.Contains(t, res.Stdout.String(), "bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa")
			// Invalid CID error should be in stderr
			assert.Contains(t, res.Stderr.String(), "bad-cid: invalid cid")
		})
	})
}

// testCidFormat tests 'ipfs cid format' subcommand
// Includes regression tests for https://github.com/ipfs/kubo/issues/9007
func testCidFormat(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()

	t.Run("formats CIDs with various options", func(t *testing.T) {
		t.Run("default format preserves CID", func(t *testing.T) {
			res := node.RunIPFS("cid", "format", "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo")
			assert.Equal(t, 0, res.ExitCode())
			assert.Equal(t, "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo\n", res.Stdout.String())
		})

		t.Run("convert to CIDv1 with base58btc", func(t *testing.T) {
			res := node.RunIPFS("cid", "format", "-v", "1", "-b", "base58btc",
				"QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo")
			assert.Equal(t, 0, res.ExitCode())
			assert.Equal(t, "zdj7WgefqQm5HogBQ2bckZuTYYDarRTUZi51GYCnerHD2G86j\n", res.Stdout.String())
		})

		t.Run("convert to CIDv0", func(t *testing.T) {
			res := node.RunIPFS("cid", "format", "-v", "0",
				"bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa")
			assert.Equal(t, 0, res.ExitCode())
			assert.Equal(t, "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo\n", res.Stdout.String())
		})

		t.Run("change codec to raw", func(t *testing.T) {
			res := node.RunIPFS("cid", "format", "--mc", "raw", "-b", "base32",
				"bafybeievd6mwe6vcwnkwo3eizs3h7w3a34opszbyfxziqdxguhjw7imdve")
			assert.Equal(t, 0, res.ExitCode())
			assert.Equal(t, "bafkreievd6mwe6vcwnkwo3eizs3h7w3a34opszbyfxziqdxguhjw7imdve\n", res.Stdout.String())
		})

		t.Run("multiple valid CIDs with format options", func(t *testing.T) {
			res := node.RunIPFS("cid", "format", "-v", "1", "-b", "base58btc",
				"QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo",
				"bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa")
			assert.Equal(t, 0, res.ExitCode())
			assert.Empty(t, res.Stderr.String())
			lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
			assert.Equal(t, 2, len(lines))
			assert.Equal(t, "zdj7WgefqQm5HogBQ2bckZuTYYDarRTUZi51GYCnerHD2G86j", lines[0])
			assert.Equal(t, "zdj7WgefqQm5HogBQ2bckZuTYYDarRTUZi51GYCnerHD2G86j", lines[1])
		})
	})

	t.Run("error handling", func(t *testing.T) {
		// Regression tests for https://github.com/ipfs/kubo/issues/9007
		t.Run("returns error code 1 for single invalid CID", func(t *testing.T) {
			res := node.RunIPFS("cid", "format", "not-a-cid")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "not-a-cid: invalid cid")
			assert.Contains(t, res.Stderr.String(), "Error: errors while displaying some entries")
		})

		t.Run("returns error code 1 for mixed valid and invalid CIDs", func(t *testing.T) {
			res := node.RunIPFS("cid", "format", "not-a-cid", "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo")
			assert.Equal(t, 1, res.ExitCode())
			// Valid CID should be printed to stdout
			assert.Contains(t, res.Stdout.String(), "QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo")
			// Invalid CID error should be printed to stderr
			assert.Contains(t, res.Stderr.String(), "not-a-cid: invalid cid")
			assert.Contains(t, res.Stderr.String(), "Error: errors while displaying some entries")
		})

		t.Run("returns error code 1 for stdin with invalid CIDs", func(t *testing.T) {
			input := "invalid\nQmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo"
			res := node.RunPipeToIPFS(strings.NewReader(input), "cid", "format", "-v", "1", "-b", "base58btc")
			assert.Equal(t, 1, res.ExitCode())
			// Valid CID should be converted
			assert.Contains(t, res.Stdout.String(), "zdj7WgefqQm5HogBQ2bckZuTYYDarRTUZi51GYCnerHD2G86j")
			// Invalid CID error should be in stderr
			assert.Contains(t, res.Stderr.String(), "invalid: invalid cid")
		})
	})
}

// testCidBases tests 'ipfs cid bases' subcommand
func testCidBases(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()

	t.Run("lists available bases", func(t *testing.T) {
		// This is a regression test to ensure we don't accidentally add or remove support
		// for multibase encodings. If a new base is intentionally added or removed,
		// this test should be updated accordingly.
		expectedBases := []string{
			"identity",
			"base2",
			"base16",
			"base16upper",
			"base32",
			"base32upper",
			"base32pad",
			"base32padupper",
			"base32hex",
			"base32hexupper",
			"base32hexpad",
			"base32hexpadupper",
			"base36",
			"base36upper",
			"base58btc",
			"base58flickr",
			"base64",
			"base64pad",
			"base64url",
			"base64urlpad",
			"base256emoji",
		}

		res := node.RunIPFS("cid", "bases")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "bases", expectedBases, lines)
	})

	t.Run("with --prefix flag shows single letter prefixes", func(t *testing.T) {
		// Regression test to catch any changes to the output format or supported bases
		expectedLines := []string{
			"identity",
			"0  base2",
			"b  base32",
			"B  base32upper",
			"c  base32pad",
			"C  base32padupper",
			"f  base16",
			"F  base16upper",
			"k  base36",
			"K  base36upper",
			"m  base64",
			"M  base64pad",
			"t  base32hexpad",
			"T  base32hexpadupper",
			"u  base64url",
			"U  base64urlpad",
			"v  base32hex",
			"V  base32hexupper",
			"z  base58btc",
			"Z  base58flickr",
			"🚀  base256emoji",
		}

		res := node.RunIPFS("cid", "bases", "--prefix")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "bases --prefix output", expectedLines, lines)
	})

	t.Run("with --numeric flag shows numeric codes", func(t *testing.T) {
		// Regression test to catch any changes to the output format or supported bases
		expectedLines := []string{
			"0  identity",
			"48  base2",
			"98  base32",
			"66  base32upper",
			"99  base32pad",
			"67  base32padupper",
			"102  base16",
			"70  base16upper",
			"107  base36",
			"75  base36upper",
			"109  base64",
			"77  base64pad",
			"116  base32hexpad",
			"84  base32hexpadupper",
			"117  base64url",
			"85  base64urlpad",
			"118  base32hex",
			"86  base32hexupper",
			"122  base58btc",
			"90  base58flickr",
			"128640  base256emoji",
		}

		res := node.RunIPFS("cid", "bases", "--numeric")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "bases --numeric output", expectedLines, lines)
	})

	t.Run("with both --prefix and --numeric flags", func(t *testing.T) {
		// Regression test to catch any changes to the output format or supported bases
		expectedLines := []string{
			"0  identity",
			"0      48  base2",
			"b      98  base32",
			"B      66  base32upper",
			"c      99  base32pad",
			"C      67  base32padupper",
			"f     102  base16",
			"F      70  base16upper",
			"k     107  base36",
			"K      75  base36upper",
			"m     109  base64",
			"M      77  base64pad",
			"t     116  base32hexpad",
			"T      84  base32hexpadupper",
			"u     117  base64url",
			"U      85  base64urlpad",
			"v     118  base32hex",
			"V      86  base32hexupper",
			"z     122  base58btc",
			"Z      90  base58flickr",
			"🚀  128640  base256emoji",
		}

		res := node.RunIPFS("cid", "bases", "--prefix", "--numeric")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "bases --prefix --numeric output", expectedLines, lines)
	})
}

// testCidCodecs tests 'ipfs cid codecs' subcommand
func testCidCodecs(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()

	t.Run("lists available codecs", func(t *testing.T) {
		// This is a regression test to ensure we don't accidentally add or remove
		// IPLD codecs. If a codec is intentionally added or removed,
		// this test should be updated accordingly.
		expectedCodecs := []string{
			"cbor",
			"raw",
			"dag-pb",
			"dag-cbor",
			"libp2p-key",
			"git-raw",
			"torrent-info",
			"torrent-file",
			"blake3-hashseq",
			"leofcoin-block",
			"leofcoin-tx",
			"leofcoin-pr",
			"dag-jose",
			"dag-cose",
			"eth-block",
			"eth-block-list",
			"eth-tx-trie",
			"eth-tx",
			"eth-tx-receipt-trie",
			"eth-tx-receipt",
			"eth-state-trie",
			"eth-account-snapshot",
			"eth-storage-trie",
			"eth-receipt-log-trie",
			"eth-receipt-log",
			"bitcoin-block",
			"bitcoin-tx",
			"bitcoin-witness-commitment",
			"zcash-block",
			"zcash-tx",
			"stellar-block",
			"stellar-tx",
			"decred-block",
			"decred-tx",
			"dash-block",
			"dash-tx",
			"swarm-manifest",
			"swarm-feed",
			"beeson",
			"dag-json",
			"swhid-1-snp",
			"json",
			"rdfc-1",
			"json-jcs",
		}

		res := node.RunIPFS("cid", "codecs")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "codecs", expectedCodecs, lines)
	})

	t.Run("with --numeric flag shows codec numbers", func(t *testing.T) {
		// This is a regression test to ensure we don't accidentally add or remove
		// IPLD codecs. If a codec is intentionally added or removed,
		// this test should be updated accordingly.
		expectedLines := []string{
			"81  cbor",
			"85  raw",
			"112  dag-pb",
			"113  dag-cbor",
			"114  libp2p-key",
			"120  git-raw",
			"123  torrent-info",
			"124  torrent-file",
			"128  blake3-hashseq",
			"129  leofcoin-block",
			"130  leofcoin-tx",
			"131  leofcoin-pr",
			"133  dag-jose",
			"134  dag-cose",
			"144  eth-block",
			"145  eth-block-list",
			"146  eth-tx-trie",
			"147  eth-tx",
			"148  eth-tx-receipt-trie",
			"149  eth-tx-receipt",
			"150  eth-state-trie",
			"151  eth-account-snapshot",
			"152  eth-storage-trie",
			"153  eth-receipt-log-trie",
			"154  eth-receipt-log",
			"176  bitcoin-block",
			"177  bitcoin-tx",
			"178  bitcoin-witness-commitment",
			"192  zcash-block",
			"193  zcash-tx",
			"208  stellar-block",
			"209  stellar-tx",
			"224  decred-block",
			"225  decred-tx",
			"240  dash-block",
			"241  dash-tx",
			"250  swarm-manifest",
			"251  swarm-feed",
			"252  beeson",
			"297  dag-json",
			"496  swhid-1-snp",
			"512  json",
			"46083  rdfc-1",
			"46593  json-jcs",
		}

		res := node.RunIPFS("cid", "codecs", "--numeric")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "codecs --numeric output", expectedLines, lines)
	})

	t.Run("with --supported flag lists only supported codecs", func(t *testing.T) {
		// This is a regression test to ensure we don't accidentally change the list
		// of supported codecs. If a codec is intentionally added or removed from
		// support, this test should be updated accordingly.
		expectedSupportedCodecs := []string{
			"cbor",
			"dag-cbor",
			"dag-jose",
			"dag-json",
			"dag-pb",
			"git-raw",
			"json",
			"libp2p-key",
			"raw",
		}

		res := node.RunIPFS("cid", "codecs", "--supported")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "supported codecs", expectedSupportedCodecs, lines)
	})

	t.Run("with both --supported and --numeric flags", func(t *testing.T) {
		// Regression test to catch any changes to supported codecs or output format
		expectedLines := []string{
			"81  cbor",
			"85  raw",
			"112  dag-pb",
			"113  dag-cbor",
			"114  libp2p-key",
			"120  git-raw",
			"133  dag-jose",
			"297  dag-json",
			"512  json",
		}

		res := node.RunIPFS("cid", "codecs", "--supported", "--numeric")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "codecs --supported --numeric output", expectedLines, lines)
	})
}

// testCidHashes tests 'ipfs cid hashes' subcommand
func testCidHashes(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()

	t.Run("lists available hashes", func(t *testing.T) {
		// This is a regression test to ensure we don't accidentally add or remove
		// support for hash functions. If a hash function is intentionally added
		// or removed, this test should be updated accordingly.
		expectedHashes := []string{
			"identity",
			"sha1",
			"sha2-256",
			"sha2-512",
			"sha3-512",
			"sha3-384",
			"sha3-256",
			"sha3-224",
			"shake-256",
			"keccak-224",
			"keccak-256",
			"keccak-384",
			"keccak-512",
			"blake3",
			"dbl-sha2-256",
		}

		// Also expect all blake2b variants (160-512 in steps of 8)
		for i := 160; i <= 512; i += 8 {
			expectedHashes = append(expectedHashes, fmt.Sprintf("blake2b-%d", i))
		}

		// Also expect all blake2s variants (160-256 in steps of 8)
		for i := 160; i <= 256; i += 8 {
			expectedHashes = append(expectedHashes, fmt.Sprintf("blake2s-%d", i))
		}

		res := node.RunIPFS("cid", "hashes")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "hash functions", expectedHashes, lines)
	})

	t.Run("with --numeric flag shows hash function codes", func(t *testing.T) {
		// This is a regression test to ensure we don't accidentally add or remove
		// support for hash functions. If a hash function is intentionally added
		// or removed, this test should be updated accordingly.
		expectedLines := []string{
			"0  identity",
			"17  sha1",
			"18  sha2-256",
			"19  sha2-512",
			"20  sha3-512",
			"21  sha3-384",
			"22  sha3-256",
			"23  sha3-224",
			"25  shake-256",
			"26  keccak-224",
			"27  keccak-256",
			"28  keccak-384",
			"29  keccak-512",
			"30  blake3",
			"86  dbl-sha2-256",
		}

		// Add all blake2b variants (160-512 in steps of 8)
		for i := 160; i <= 512; i += 8 {
			expectedLines = append(expectedLines, fmt.Sprintf("%d  blake2b-%d", 45568+i/8, i))
		}

		// Add all blake2s variants (160-256 in steps of 8)
		for i := 160; i <= 256; i += 8 {
			expectedLines = append(expectedLines, fmt.Sprintf("%d  blake2s-%d", 45632+i/8, i))
		}

		res := node.RunIPFS("cid", "hashes", "--numeric")
		assert.Equal(t, 0, res.ExitCode())

		lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
		assertExactSet(t, "hashes --numeric output", expectedLines, lines)
	})
}

// assertExactSet compares expected vs actual items and reports clear errors for any differences.
// This is used as a regression test to ensure we don't accidentally add or remove support.
// Both expected and actual strings are trimmed of whitespace before comparison for maintainability.
func assertExactSet(t *testing.T, itemType string, expected []string, actual []string) {
	t.Helper()

	// Normalize by trimming whitespace
	normalizedExpected := make([]string, len(expected))
	for i, item := range expected {
		normalizedExpected[i] = strings.TrimSpace(item)
	}

	normalizedActual := make([]string, len(actual))
	for i, item := range actual {
		normalizedActual[i] = strings.TrimSpace(item)
	}

	expectedSet := make(map[string]bool)
	for _, item := range normalizedExpected {
		expectedSet[item] = true
	}

	actualSet := make(map[string]bool)
	for _, item := range normalizedActual {
		actualSet[item] = true
	}

	var missing []string
	for _, item := range normalizedExpected {
		if !actualSet[item] {
			missing = append(missing, item)
		}
	}

	var unexpected []string
	for _, item := range normalizedActual {
		if !expectedSet[item] {
			unexpected = append(unexpected, item)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing expected %s: %q", itemType, missing)
	}
	if len(unexpected) > 0 {
		t.Errorf("Unexpected %s found: %q", itemType, unexpected)
	}

	assert.Equal(t, len(expected), len(actual),
		"Expected %d %s but got %d", len(expected), itemType, len(actual))
}
