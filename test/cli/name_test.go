package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
		res := node.RunIPFS("name", "publish", "/ipfs/"+fixtureCid)
		require.Error(t, res.Err)
		require.Equal(t, 1, res.ExitCode())
		require.Contains(t, res.Stderr.String(), `can't publish while offline`)
	})

	t.Run("Publish V2-only record", func(t *testing.T) {
		t.Parallel()

		node := makeDaemon(t, nil).StartDaemon()
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
