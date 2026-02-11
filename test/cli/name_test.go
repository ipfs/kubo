// Tests for `ipfs name` CLI commands.
// - TestName: tests name publish, resolve, and inspect
// - TestNameGetPut: tests name get and put for raw IPNS record handling

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/kubo/core/commands/name"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	const (
		fixturePath = "fixtures/TestName.car"
		fixtureCid  = "bafybeidg3uxibfrt7uqh7zd5yaodetik7wjwi4u7rwv2ndbgj6ec7lsv2a"
		dagCid      = "bafyreidgts62p4rtg3rtmptmbv2dt46zjzin275fr763oku3wfod3quzay"
	)

	makeDaemon := func(t *testing.T, initArgs []string) *harness.Node {
		node := harness.NewT(t).NewNode().Init(append([]string{"--profile=test"}, initArgs...)...)
		r, err := os.Open(fixturePath)
		require.Nil(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		require.NoError(t, err)
		return node
	}

	testPublishingWithSelf := func(keyType string) {
		t.Run("Publishing with self (keyType = "+keyType+")", func(t *testing.T) {
			t.Parallel()

			args := []string{}
			if keyType != "default" {
				args = append(args, "-a="+keyType)
			}

			node := makeDaemon(t, args)
			name := ipns.NameFromPeer(node.PeerID())

			t.Run("Publishing a CID", func(t *testing.T) {
				publishPath := "/ipfs/" + fixtureCid

				res := node.IPFS("name", "publish", "--allow-offline", publishPath)
				require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())

				res = node.IPFS("name", "resolve", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())
			})

			t.Run("Publishing a CID with -Q option", func(t *testing.T) {
				publishPath := "/ipfs/" + fixtureCid

				res := node.IPFS("name", "publish", "--allow-offline", "-Q", publishPath)
				require.Equal(t, name.String()+"\n", res.Stdout.String())

				res = node.IPFS("name", "resolve", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())
			})

			t.Run("Publishing a CID+subpath", func(t *testing.T) {
				publishPath := "/ipfs/" + fixtureCid + "/hello"

				res := node.IPFS("name", "publish", "--allow-offline", publishPath)
				require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())

				res = node.IPFS("name", "resolve", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())
			})

			t.Run("Publishing nothing fails", func(t *testing.T) {
				res := node.RunIPFS("name", "publish")
				require.Error(t, res.Err)
				require.Equal(t, 1, res.ExitCode())
				require.Contains(t, res.Stderr.String(), `argument "ipfs-path" is required`)
			})

			t.Run("Publishing with IPLD works", func(t *testing.T) {
				publishPath := "/ipld/" + dagCid + "/thing"
				res := node.IPFS("name", "publish", "--allow-offline", publishPath)
				require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())

				res = node.IPFS("name", "resolve", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())
			})

			publishPath := "/ipfs/" + fixtureCid
			res := node.IPFS("name", "publish", "--allow-offline", publishPath)
			require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())

			t.Run("Resolving self offline succeeds (daemon off)", func(t *testing.T) {
				res = node.IPFS("name", "resolve", "--offline", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())

				// Test without cache.
				res = node.IPFS("name", "resolve", "--offline", "-n", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())
			})

			node.StartDaemon()
			defer node.StopDaemon()

			t.Run("Resolving self offline succeeds (daemon on)", func(t *testing.T) {
				res = node.IPFS("name", "resolve", "--offline", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())

				// Test without cache.
				res = node.IPFS("name", "resolve", "--offline", "-n", "/ipns/"+name.String())
				require.Equal(t, publishPath+"\n", res.Stdout.String())
			})
		})
	}

	testPublishingWithSelf("default")
	testPublishingWithSelf("rsa")
	testPublishingWithSelf("ed25519")

	testPublishWithKey := func(name string, keyArgs ...string) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			node := makeDaemon(t, nil)

			keyGenArgs := []string{"key", "gen"}
			keyGenArgs = append(keyGenArgs, keyArgs...)
			keyGenArgs = append(keyGenArgs, "key")

			res := node.IPFS(keyGenArgs...)
			key := strings.TrimSpace(res.Stdout.String())

			publishPath := "/ipfs/" + fixtureCid
			name, err := ipns.NameFromString(key)
			require.NoError(t, err)

			res = node.IPFS("name", "publish", "--allow-offline", "--key="+key, publishPath)
			require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())
		})
	}

	testPublishWithKey("Publishing with RSA (with b58mh) Key", "--ipns-base=b58mh", "--type=rsa", "--size=2048")
	testPublishWithKey("Publishing with ED25519 (with b58mh) Key", "--ipns-base=b58mh", "--type=ed25519")
	testPublishWithKey("Publishing with ED25519 (with base36) Key", "--ipns-base=base36", "--type=ed25519")

	t.Run("Fails to publish in offline mode", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t, nil).StartDaemon("--offline")
		defer node.StopDaemon()
		res := node.RunIPFS("name", "publish", "/ipfs/"+fixtureCid)
		require.Error(t, res.Err)
		require.Equal(t, 1, res.ExitCode())
		require.Contains(t, res.Stderr.String(), "can't publish while offline: pass `--allow-offline` to override or `--allow-delegated` if Ipns.DelegatedPublishers are set up")
	})

	t.Run("Publish V2-only record", func(t *testing.T) {
		t.Parallel()

		node := makeDaemon(t, nil).StartDaemon()
		defer node.StopDaemon()
		ipnsName := ipns.NameFromPeer(node.PeerID()).String()
		ipnsPath := ipns.NamespacePrefix + ipnsName
		publishPath := "/ipfs/" + fixtureCid

		res := node.IPFS("name", "publish", "--ttl=30m", "--v1compat=false", publishPath)
		require.Equal(t, fmt.Sprintf("Published to %s: %s\n", ipnsName, publishPath), res.Stdout.String())

		res = node.IPFS("name", "resolve", ipnsPath)
		require.Equal(t, publishPath+"\n", res.Stdout.String())

		res = node.IPFS("routing", "get", ipnsPath)
		record := res.Stdout.Bytes()

		res = node.PipeToIPFS(bytes.NewReader(record), "name", "inspect")
		out := res.Stdout.String()
		require.Contains(t, out, "This record was not verified.")
		require.Contains(t, out, publishPath)
		require.Contains(t, out, "30m")

		res = node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--verify="+ipnsPath)
		out = res.Stdout.String()
		require.Contains(t, out, "Valid: true")
		require.Contains(t, out, "Signature Type: V2")
		require.Contains(t, out, fmt.Sprintf("Protobuf Size:  %d", len(record)))
	})

	t.Run("Publish with TTL and inspect record", func(t *testing.T) {
		t.Parallel()

		node := makeDaemon(t, nil).StartDaemon()
		t.Cleanup(func() { node.StopDaemon() })
		ipnsPath := ipns.NamespacePrefix + ipns.NameFromPeer(node.PeerID()).String()
		publishPath := "/ipfs/" + fixtureCid

		_ = node.IPFS("name", "publish", "--ttl=30m", publishPath)
		res := node.IPFS("routing", "get", ipnsPath)
		record := res.Stdout.Bytes()

		t.Run("Inspect record shows correct TTL and that it is not validated", func(t *testing.T) {
			t.Parallel()
			res = node.PipeToIPFS(bytes.NewReader(record), "name", "inspect")
			out := res.Stdout.String()
			require.Contains(t, out, "This record was not verified.")
			require.Contains(t, out, publishPath)
			require.Contains(t, out, "30m")
			require.Contains(t, out, "Signature Type: V1+V2")
			require.Contains(t, out, fmt.Sprintf("Protobuf Size:  %d", len(record)))
		})

		t.Run("Inspect record shows valid with correct name", func(t *testing.T) {
			t.Parallel()
			res := node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--enc=json", "--verify="+ipnsPath)
			val := name.IpnsInspectResult{}
			err := json.Unmarshal(res.Stdout.Bytes(), &val)
			require.NoError(t, err)
			require.True(t, val.Validation.Valid)
		})

		t.Run("Inspect record shows invalid with wrong name", func(t *testing.T) {
			t.Parallel()
			res := node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--enc=json", "--verify=12D3KooWRirYjmmQATx2kgHBfky6DADsLP7ex1t7BRxJ6nqLs9WH")
			val := name.IpnsInspectResult{}
			err := json.Unmarshal(res.Stdout.Bytes(), &val)
			require.NoError(t, err)
			require.False(t, val.Validation.Valid)
		})
	})

	t.Run("Inspect with verification using wrong RSA key errors", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t, nil).StartDaemon()
		defer node.StopDaemon()

		// Prepare RSA Key 1
		res := node.IPFS("key", "gen", "--type=rsa", "--size=4096", "key1")
		key1 := strings.TrimSpace(res.Stdout.String())
		name1, err := ipns.NameFromString(key1)
		require.NoError(t, err)

		// Prepare RSA Key 2
		res = node.IPFS("key", "gen", "--type=rsa", "--size=4096", "key2")
		key2 := strings.TrimSpace(res.Stdout.String())
		name2, err := ipns.NameFromString(key2)
		require.NoError(t, err)

		// Publish using Key 1
		publishPath := "/ipfs/" + fixtureCid
		res = node.IPFS("name", "publish", "--allow-offline", "--key="+key1, publishPath)
		require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name1.String(), publishPath), res.Stdout.String())

		// Get IPNS Record
		res = node.IPFS("routing", "get", ipns.NamespacePrefix+name1.String())
		record := res.Stdout.Bytes()

		// Validate with correct key succeeds
		res = node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--verify="+name1.String(), "--enc=json")
		val := name.IpnsInspectResult{}
		err = json.Unmarshal(res.Stdout.Bytes(), &val)
		require.NoError(t, err)
		require.True(t, val.Validation.Valid)

		// Validate with wrong key fails
		res = node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--verify="+name2.String(), "--enc=json")
		val = name.IpnsInspectResult{}
		err = json.Unmarshal(res.Stdout.Bytes(), &val)
		require.NoError(t, err)
		require.False(t, val.Validation.Valid)
	})

	t.Run("Publishing with custom sequence number", func(t *testing.T) {
		t.Parallel()

		node := makeDaemon(t, nil)
		publishPath := "/ipfs/" + fixtureCid
		name := ipns.NameFromPeer(node.PeerID())

		t.Run("Publish with sequence=0 is not allowed", func(t *testing.T) {
			// Sequence=0 is never valid, even on a fresh node
			res := node.RunIPFS("name", "publish", "--allow-offline", "--ttl=0", "--sequence=0", publishPath)
			require.NotEqual(t, 0, res.ExitCode(), "Expected publish with sequence=0 to fail")
			require.Contains(t, res.Stderr.String(), "sequence number must be greater than the current record sequence")
		})

		t.Run("Publish with sequence=1 on fresh node", func(t *testing.T) {
			// Sequence=1 is the minimum valid sequence number for first publish
			res := node.IPFS("name", "publish", "--allow-offline", "--ttl=0", "--sequence=1", publishPath)
			require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())
		})

		t.Run("Publish with sequence=42", func(t *testing.T) {
			res := node.IPFS("name", "publish", "--allow-offline", "--ttl=0", "--sequence=42", publishPath)
			require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())
		})

		t.Run("Publish with large sequence number", func(t *testing.T) {
			res := node.IPFS("name", "publish", "--allow-offline", "--ttl=0", "--sequence=18446744073709551615", publishPath) // Max uint64
			require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath), res.Stdout.String())
		})
	})

	t.Run("Sequence number monotonic check", func(t *testing.T) {
		t.Parallel()

		node := makeDaemon(t, nil).StartDaemon()
		defer node.StopDaemon()
		publishPath1 := "/ipfs/" + fixtureCid
		publishPath2 := "/ipfs/" + dagCid // Different content
		name := ipns.NameFromPeer(node.PeerID())

		// First, publish with a high sequence number (1000)
		res := node.IPFS("name", "publish", "--ttl=0", "--sequence=1000", publishPath1)
		require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath1), res.Stdout.String())

		// Verify the record was published successfully
		res = node.IPFS("name", "resolve", name.String())
		require.Contains(t, res.Stdout.String(), publishPath1)

		// Now try to publish different content with a LOWER sequence number (500)
		// This should fail due to monotonic sequence check
		res = node.RunIPFS("name", "publish", "--ttl=0", "--sequence=500", publishPath2)
		require.NotEqual(t, 0, res.ExitCode(), "Expected publish with lower sequence to fail")
		require.Contains(t, res.Stderr.String(), "sequence number", "Expected error about sequence number")

		// Verify the original content is still published (not overwritten)
		res = node.IPFS("name", "resolve", name.String())
		require.Contains(t, res.Stdout.String(), publishPath1, "Original content should still be published")
		require.NotContains(t, res.Stdout.String(), publishPath2, "New content should not have been published")

		// Publishing with a HIGHER sequence number should succeed
		res = node.IPFS("name", "publish", "--ttl=0", "--sequence=2000", publishPath2)
		require.Equal(t, fmt.Sprintf("Published to %s: %s\n", name.String(), publishPath2), res.Stdout.String())

		// Verify the new content is now published
		res = node.IPFS("name", "resolve", name.String())
		require.Contains(t, res.Stdout.String(), publishPath2, "New content should now be published")
	})
}

func TestNameGetPut(t *testing.T) {
	t.Parallel()

	const (
		fixturePath = "fixtures/TestName.car"
		fixtureCid  = "bafybeidg3uxibfrt7uqh7zd5yaodetik7wjwi4u7rwv2ndbgj6ec7lsv2a"
	)

	makeDaemon := func(t *testing.T, daemonArgs ...string) *harness.Node {
		node := harness.NewT(t).NewNode().Init("--profile=test")
		r, err := os.Open(fixturePath)
		require.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		require.NoError(t, err)
		return node.StartDaemon(daemonArgs...)
	}

	// makeKey creates a unique IPNS key for a test and returns the IPNS name
	makeKey := func(t *testing.T, node *harness.Node, keyName string) ipns.Name {
		res := node.IPFS("key", "gen", "--type=ed25519", keyName)
		keyID := strings.TrimSpace(res.Stdout.String())
		name, err := ipns.NameFromString(keyID)
		require.NoError(t, err)
		return name
	}

	// makeExternalRecord creates an IPNS record on an ephemeral node that is
	// shut down before returning. This ensures the test node has no local
	// knowledge of the record, properly testing put/get functionality.
	// We use short --lifetime so if IPNS records from tests get published on
	// the public DHT, they won't waste storage for long.
	makeExternalRecord := func(t *testing.T, h *harness.Harness, publishPath string, publishArgs ...string) (ipns.Name, []byte) {
		node := h.NewNode().Init("--profile=test")

		r, err := os.Open(fixturePath)
		require.NoError(t, err)
		defer r.Close()
		err = node.IPFSDagImport(r, fixtureCid)
		require.NoError(t, err)

		node.StartDaemon()

		res := node.IPFS("key", "gen", "--type=ed25519", "ephemeral-key")
		keyID := strings.TrimSpace(res.Stdout.String())
		ipnsName, err := ipns.NameFromString(keyID)
		require.NoError(t, err)

		args := []string{"name", "publish", "--key=ephemeral-key", "--lifetime=5m"}
		args = append(args, publishArgs...)
		args = append(args, publishPath)
		node.IPFS(args...)

		res = node.IPFS("name", "get", ipnsName.String())
		record := res.Stdout.Bytes()
		require.NotEmpty(t, record)

		node.StopDaemon()

		return ipnsName, record
	}

	t.Run("name get retrieves IPNS record", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		publishPath := "/ipfs/" + fixtureCid
		ipnsName := makeKey(t, node, "testkey")

		// publish a record first
		node.IPFS("name", "publish", "--key=testkey", "--lifetime=5m", publishPath)

		// retrieve the record using name get
		res := node.IPFS("name", "get", ipnsName.String())
		record := res.Stdout.Bytes()
		require.NotEmpty(t, record, "expected non-empty IPNS record")

		// verify the record is valid by inspecting it
		res = node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--verify="+ipnsName.String())
		require.Contains(t, res.Stdout.String(), "Valid: true")
		require.Contains(t, res.Stdout.String(), publishPath)
	})

	t.Run("name get accepts /ipns/ prefix", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		publishPath := "/ipfs/" + fixtureCid
		ipnsName := makeKey(t, node, "testkey")

		node.IPFS("name", "publish", "--key=testkey", "--lifetime=5m", publishPath)

		// retrieve with /ipns/ prefix
		res := node.IPFS("name", "get", "/ipns/"+ipnsName.String())
		record := res.Stdout.Bytes()
		require.NotEmpty(t, record)

		// verify the record
		res = node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--verify="+ipnsName.String())
		require.Contains(t, res.Stdout.String(), "Valid: true")
	})

	t.Run("name get fails for non-existent name", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		// try to get a record for a random peer ID that doesn't exist
		res := node.RunIPFS("name", "get", "12D3KooWRirYjmmQATx2kgHBfky6DADsLP7ex1t7BRxJ6nqLs9WH")
		require.Error(t, res.Err)
		require.NotEqual(t, 0, res.ExitCode())
	})

	t.Run("name get fails for invalid name format", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		res := node.RunIPFS("name", "get", "not-a-valid-ipns-name")
		require.Error(t, res.Err)
		require.NotEqual(t, 0, res.ExitCode())
	})

	t.Run("name put accepts /ipns/ prefix", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		publishPath := "/ipfs/" + fixtureCid
		ipnsName := makeKey(t, node, "testkey")

		node.IPFS("name", "publish", "--key=testkey", "--lifetime=5m", publishPath)

		res := node.IPFS("name", "get", ipnsName.String())
		record := res.Stdout.Bytes()

		// put with /ipns/ prefix
		res = node.PipeToIPFS(bytes.NewReader(record), "name", "put", "--force", "/ipns/"+ipnsName.String())
		require.NoError(t, res.Err)
	})

	t.Run("name put fails for invalid name format", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		// create a dummy file
		recordFile := filepath.Join(node.Dir, "dummy.bin")
		err := os.WriteFile(recordFile, []byte("dummy"), 0644)
		require.NoError(t, err)

		res := node.RunIPFS("name", "put", "not-a-valid-ipns-name", recordFile)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "invalid IPNS name")
	})

	t.Run("name put rejects oversized record", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		ipnsName := makeKey(t, node, "testkey")

		// create a file larger than 10 KiB
		oversizedRecord := make([]byte, 11*1024)
		recordFile := filepath.Join(node.Dir, "oversized.bin")
		err := os.WriteFile(recordFile, oversizedRecord, 0644)
		require.NoError(t, err)

		res := node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "exceeds maximum size")
	})

	t.Run("name put --force skips size check", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		ipnsName := makeKey(t, node, "testkey")

		// create a file larger than 10 KiB
		oversizedRecord := make([]byte, 11*1024)
		recordFile := filepath.Join(node.Dir, "oversized.bin")
		err := os.WriteFile(recordFile, oversizedRecord, 0644)
		require.NoError(t, err)

		// with --force, size check is skipped (but routing will likely reject it)
		res := node.RunIPFS("name", "put", "--force", ipnsName.String(), recordFile)
		// the command itself should not fail on size, but routing may reject
		// we just verify it doesn't fail with "exceeds maximum size"
		if res.Err != nil {
			require.NotContains(t, res.Stderr.String(), "exceeds maximum size")
		}
	})

	t.Run("name put stores IPNS record", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node (shut down before test node starts)
		ipnsName, record := makeExternalRecord(t, h, publishPath)

		// start test node (has no local knowledge of the record)
		node := makeDaemon(t)
		defer node.StopDaemon()

		// put the record (should succeed since no existing record)
		recordFile := filepath.Join(node.Dir, "record.bin")
		err := os.WriteFile(recordFile, record, 0644)
		require.NoError(t, err)

		res := node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.NoError(t, res.Err)

		// verify the record was stored by getting it back
		res = node.IPFS("name", "get", ipnsName.String())
		retrievedRecord := res.Stdout.Bytes()
		require.Equal(t, record, retrievedRecord, "stored record should match original")
	})

	t.Run("name put with --force overwrites existing record", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node
		ipnsName, record := makeExternalRecord(t, h, publishPath)

		// start test node
		node := makeDaemon(t)
		defer node.StopDaemon()

		// first put the record normally
		recordFile := filepath.Join(node.Dir, "record.bin")
		err := os.WriteFile(recordFile, record, 0644)
		require.NoError(t, err)

		res := node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.NoError(t, res.Err)

		// now try to put the same record again (should fail - same sequence)
		res = node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "existing record has sequence")

		// put the record with --force (should succeed)
		res = node.RunIPFS("name", "put", "--force", ipnsName.String(), recordFile)
		require.NoError(t, res.Err)
	})

	t.Run("name put validates signature against name", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node
		_, record := makeExternalRecord(t, h, publishPath)

		// start test node
		node := makeDaemon(t)
		defer node.StopDaemon()

		// write the record to a file
		recordFile := filepath.Join(node.Dir, "record.bin")
		err := os.WriteFile(recordFile, record, 0644)
		require.NoError(t, err)

		// try to put with a wrong name (should fail validation)
		wrongName := "12D3KooWRirYjmmQATx2kgHBfky6DADsLP7ex1t7BRxJ6nqLs9WH"
		res := node.RunIPFS("name", "put", wrongName, recordFile)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "record validation failed")
	})

	t.Run("name put with --force skips command validation", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node
		ipnsName, record := makeExternalRecord(t, h, publishPath)

		// start test node
		node := makeDaemon(t)
		defer node.StopDaemon()

		// with --force the command skips its own validation (signature, sequence check)
		// and passes the record directly to the routing layer
		res := node.PipeToIPFS(bytes.NewReader(record), "name", "put", "--force", ipnsName.String())
		require.NoError(t, res.Err)
	})

	t.Run("name put rejects empty record", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		ipnsName := makeKey(t, node, "testkey")

		// create an empty file
		recordFile := filepath.Join(node.Dir, "empty.bin")
		err := os.WriteFile(recordFile, []byte{}, 0644)
		require.NoError(t, err)

		res := node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "record is empty")
	})

	t.Run("name put rejects invalid record", func(t *testing.T) {
		t.Parallel()
		node := makeDaemon(t)
		defer node.StopDaemon()

		ipnsName := makeKey(t, node, "testkey")

		// create a file with garbage data
		recordFile := filepath.Join(node.Dir, "garbage.bin")
		err := os.WriteFile(recordFile, []byte("not a valid ipns record"), 0644)
		require.NoError(t, err)

		res := node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "invalid IPNS record")
	})

	t.Run("name put accepts stdin", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node
		ipnsName, record := makeExternalRecord(t, h, publishPath)

		// start test node (has no local knowledge of the record)
		node := makeDaemon(t)
		defer node.StopDaemon()

		// put via stdin (no --force needed since no existing record)
		res := node.PipeToIPFS(bytes.NewReader(record), "name", "put", ipnsName.String())
		require.NoError(t, res.Err)
	})

	t.Run("name put fails when offline without --allow-offline", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node
		ipnsName, record := makeExternalRecord(t, h, publishPath)

		// write the record to a file
		recordFile := filepath.Join(h.Dir, "record.bin")
		err := os.WriteFile(recordFile, record, 0644)
		require.NoError(t, err)

		// start test node in offline mode
		node := h.NewNode().Init("--profile=test")
		node.StartDaemon("--offline")
		defer node.StopDaemon()

		// try to put without --allow-offline (should fail)
		res := node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.Error(t, res.Err)
		// error can come from our command or from the routing layer
		stderr := res.Stderr.String()
		require.True(t, strings.Contains(stderr, "offline") || strings.Contains(stderr, "online mode"),
			"expected offline-related error, got: %s", stderr)
	})

	t.Run("name put succeeds with --allow-offline", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node
		ipnsName, record := makeExternalRecord(t, h, publishPath)

		// write the record to a file
		recordFile := filepath.Join(h.Dir, "record.bin")
		err := os.WriteFile(recordFile, record, 0644)
		require.NoError(t, err)

		// start test node in offline mode
		node := h.NewNode().Init("--profile=test")
		node.StartDaemon("--offline")
		defer node.StopDaemon()

		// put with --allow-offline (should succeed, no --force needed since no existing record)
		res := node.RunIPFS("name", "put", "--allow-offline", ipnsName.String(), recordFile)
		require.NoError(t, res.Err)
	})

	t.Run("name get/put round trip preserves record bytes", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create a record on an ephemeral node
		ipnsName, originalRecord := makeExternalRecord(t, h, publishPath)

		// start test node (has no local knowledge of the record)
		node := makeDaemon(t)
		defer node.StopDaemon()

		// put the record
		res := node.PipeToIPFS(bytes.NewReader(originalRecord), "name", "put", ipnsName.String())
		require.NoError(t, res.Err)

		// get the record back
		res = node.IPFS("name", "get", ipnsName.String())
		retrievedRecord := res.Stdout.Bytes()

		// the records should be byte-for-byte identical
		require.Equal(t, originalRecord, retrievedRecord, "record bytes should be preserved after get/put round trip")
	})

	t.Run("name put --force allows storing lower sequence record", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create an ephemeral node to generate two records with different sequences
		ephNode := h.NewNode().Init("--profile=test")

		r, err := os.Open(fixturePath)
		require.NoError(t, err)
		err = ephNode.IPFSDagImport(r, fixtureCid)
		r.Close()
		require.NoError(t, err)

		ephNode.StartDaemon()

		res := ephNode.IPFS("key", "gen", "--type=ed25519", "ephemeral-key")
		keyID := strings.TrimSpace(res.Stdout.String())
		ipnsName, err := ipns.NameFromString(keyID)
		require.NoError(t, err)

		// publish record with sequence 100
		ephNode.IPFS("name", "publish", "--key=ephemeral-key", "--lifetime=5m", "--sequence=100", publishPath)
		res = ephNode.IPFS("name", "get", ipnsName.String())
		record100 := res.Stdout.Bytes()

		// publish record with sequence 200
		ephNode.IPFS("name", "publish", "--key=ephemeral-key", "--lifetime=5m", "--sequence=200", publishPath)
		res = ephNode.IPFS("name", "get", ipnsName.String())
		record200 := res.Stdout.Bytes()

		ephNode.StopDaemon()

		// start test node (has no local knowledge of the records)
		node := makeDaemon(t)
		defer node.StopDaemon()

		// helper to get sequence from record
		getSequence := func(record []byte) uint64 {
			res := node.PipeToIPFS(bytes.NewReader(record), "name", "inspect", "--enc=json")
			var result name.IpnsInspectResult
			err := json.Unmarshal(res.Stdout.Bytes(), &result)
			require.NoError(t, err)
			require.NotNil(t, result.Entry.Sequence)
			return *result.Entry.Sequence
		}

		// verify we have the right records
		require.Equal(t, uint64(100), getSequence(record100))
		require.Equal(t, uint64(200), getSequence(record200))

		// put record with sequence 200 first
		res = node.PipeToIPFS(bytes.NewReader(record200), "name", "put", ipnsName.String())
		require.NoError(t, res.Err)

		// verify current record has sequence 200
		res = node.IPFS("name", "get", ipnsName.String())
		require.Equal(t, uint64(200), getSequence(res.Stdout.Bytes()))

		// now put the lower sequence record (100) with --force
		// this should succeed (--force bypasses our sequence check)
		res = node.PipeToIPFS(bytes.NewReader(record100), "name", "put", "--force", ipnsName.String())
		require.NoError(t, res.Err, "putting lower sequence record with --force should succeed")

		// note: when we get the record, IPNS resolution returns the "best" record
		// (highest sequence), so we'll get the sequence 200 record back
		// this is expected IPNS behavior - the put succeeded, but get returns the best record
		res = node.IPFS("name", "get", ipnsName.String())
		retrievedSeq := getSequence(res.Stdout.Bytes())
		require.Equal(t, uint64(200), retrievedSeq, "IPNS get returns the best (highest sequence) record")
	})

	t.Run("name put sequence conflict detection", func(t *testing.T) {
		t.Parallel()
		h := harness.NewT(t)
		publishPath := "/ipfs/" + fixtureCid

		// create an ephemeral node to generate two records with different sequences
		ephNode := h.NewNode().Init("--profile=test")

		r, err := os.Open(fixturePath)
		require.NoError(t, err)
		err = ephNode.IPFSDagImport(r, fixtureCid)
		r.Close()
		require.NoError(t, err)

		ephNode.StartDaemon()

		res := ephNode.IPFS("key", "gen", "--type=ed25519", "ephemeral-key")
		keyID := strings.TrimSpace(res.Stdout.String())
		ipnsName, err := ipns.NameFromString(keyID)
		require.NoError(t, err)

		// publish record with sequence 100
		ephNode.IPFS("name", "publish", "--key=ephemeral-key", "--lifetime=5m", "--sequence=100", publishPath)
		res = ephNode.IPFS("name", "get", ipnsName.String())
		record100 := res.Stdout.Bytes()

		// publish record with sequence 200
		ephNode.IPFS("name", "publish", "--key=ephemeral-key", "--lifetime=5m", "--sequence=200", publishPath)
		res = ephNode.IPFS("name", "get", ipnsName.String())
		record200 := res.Stdout.Bytes()

		ephNode.StopDaemon()

		// start test node (has no local knowledge of the records)
		node := makeDaemon(t)
		defer node.StopDaemon()

		// put record with sequence 200 first
		res = node.PipeToIPFS(bytes.NewReader(record200), "name", "put", ipnsName.String())
		require.NoError(t, res.Err)

		// try to put record with sequence 100 (lower than current 200)
		recordFile := filepath.Join(node.Dir, "record100.bin")
		err = os.WriteFile(recordFile, record100, 0644)
		require.NoError(t, err)

		res = node.RunIPFS("name", "put", ipnsName.String(), recordFile)
		require.Error(t, res.Err)
		require.Contains(t, res.Stderr.String(), "existing record has sequence 200 >= new record sequence 100")
	})
}
