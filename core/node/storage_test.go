package node

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/mount"
	dssync "github.com/ipfs/go-datastore/sync"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	"github.com/stretchr/testify/require"
)

func oldDHTValueKey(routingKey string) string {
	return "/" + oldDHTValueBase32.EncodeToString([]byte(routingKey))
}

func TestIsStaleDHTValueRecordKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		stale bool
	}{
		{"old pk record", oldDHTValueKey("/pk/some-peer-id"), true},
		{"old ipns record", oldDHTValueKey("/ipns/some-name"), true},
		{"old record of unknown namespace", oldDHTValueKey("/v/other"), false},
		{"new-format pk record", "/dht/pk/" + oldDHTValueBase32.EncodeToString([]byte("/pk/some-peer-id")), false},
		{"new-format ipns record", "/dht/ipns/" + oldDHTValueBase32.EncodeToString([]byte("/ipns/some-name")), false},
		{"ipns publisher record", "/ipns/" + oldDHTValueBase32.EncodeToString([]byte{0x12, 0x20, 0xAA}), false},
		{"block key", "/blocks/CIQAAA", false},
		{"mfs root", "/local/filesroot", false},
		{"non-base32 root key", "/version!", false},
		{"base32 root key outside namespaces", "/" + oldDHTValueBase32.EncodeToString([]byte("hello")), false},
		{"root", "/", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equalf(t, tt.stale, isStaleDHTValueRecordKey(tt.key), "key %q", tt.key)
		})
	}
}

func TestPurgeStaleDHTValueRecords(t *testing.T) {
	t.Parallel()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	ctx := t.Context()

	stale := []string{
		oldDHTValueKey("/pk/peer-1"),
		oldDHTValueKey("/pk/peer-2"),
		oldDHTValueKey("/ipns/name-1"),
	}
	kept := []string{
		"/dht/pk/" + oldDHTValueBase32.EncodeToString([]byte("/pk/peer-1")),
		"/dht/ipns/" + oldDHTValueBase32.EncodeToString([]byte("/ipns/name-1")),
		"/ipns/" + oldDHTValueBase32.EncodeToString([]byte{0x12, 0x20, 0xAA}),
		"/blocks/CIQAAA",
		"/local/filesroot",
		"/providers/CIQBBB/CIQCCC",
	}
	for _, k := range append(append([]string{}, stale...), kept...) {
		require.NoError(t, ds.Put(ctx, datastore.NewKey(k), []byte("v")))
	}

	purgeStaleDHTValueRecords(ctx, ds)

	for _, k := range stale {
		has, err := ds.Has(ctx, datastore.NewKey(k))
		require.NoError(t, err)
		require.Falsef(t, has, "stale key %q should have been purged", k)
	}
	for _, k := range kept {
		has, err := ds.Has(ctx, datastore.NewKey(k))
		require.NoError(t, err)
		require.Truef(t, has, "key %q should have been kept", k)
	}

	has, err := ds.Has(ctx, dhtValuePurgeDoneKey)
	require.NoError(t, err)
	require.True(t, has, "purge marker should be set after a completed purge")

	// A later run must be a no-op: the marker short-circuits the scan, so a
	// key in the old format written afterwards survives.
	reseeded := datastore.NewKey(stale[0])
	require.NoError(t, ds.Put(ctx, reseeded, []byte("v")))
	purgeStaleDHTValueRecords(ctx, ds)
	has, err = ds.Has(ctx, reseeded)
	require.NoError(t, err)
	require.True(t, has, "marker should prevent a second scan")
}

// TestPurgeStaleDHTValueRecordsMountedRepo runs the purge against the same
// datastore shape a default repo uses: flatfs mounted at /blocks and leveldb
// at /. This exercises the Prefix "/" query across the mount, which the
// MapDatastore-based tests cannot.
func TestPurgeStaleDHTValueRecordsMountedRepo(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	blocks, err := flatfs.CreateOrOpen(filepath.Join(tmp, "blocks"), flatfs.NextToLast(2), false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocks.Close() })
	root, err := levelds.NewDatastore(filepath.Join(tmp, "datastore"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	ds := mount.New([]mount.Mount{
		{Prefix: datastore.NewKey("/blocks"), Datastore: blocks},
		{Prefix: datastore.NewKey("/"), Datastore: root},
	})
	ctx := t.Context()

	stale := []string{
		oldDHTValueKey("/pk/peer-1"),
		oldDHTValueKey("/ipns/name-1"),
	}
	kept := []string{
		"/blocks/CIQAAA",
		"/dht/pk/" + oldDHTValueBase32.EncodeToString([]byte("/pk/peer-1")),
		"/ipns/" + oldDHTValueBase32.EncodeToString([]byte{0x12, 0x20, 0xAA}),
		"/local/filesroot",
	}
	for _, k := range append(append([]string{}, stale...), kept...) {
		require.NoError(t, ds.Put(ctx, datastore.NewKey(k), []byte("v")))
	}

	purgeStaleDHTValueRecords(ctx, ds)

	for _, k := range stale {
		has, err := ds.Has(ctx, datastore.NewKey(k))
		require.NoError(t, err)
		require.Falsef(t, has, "stale key %q should have been purged", k)
	}
	for _, k := range kept {
		has, err := ds.Has(ctx, datastore.NewKey(k))
		require.NoError(t, err)
		require.Truef(t, has, "key %q should have been kept", k)
	}
	has, err := ds.Has(ctx, dhtValuePurgeDoneKey)
	require.NoError(t, err)
	require.True(t, has, "purge marker should be set after a completed purge")
}

func TestPurgeStaleDHTValueRecordsBatching(t *testing.T) {
	t.Parallel()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	ctx := t.Context()

	total := purgeBatchSize + 5
	for i := range total {
		key := oldDHTValueKey(fmt.Sprintf("/pk/peer-%d", i))
		require.NoError(t, ds.Put(ctx, datastore.NewKey(key), []byte("v")))
	}

	count, err := deleteStaleDHTValueRecords(ctx, ds)
	require.NoError(t, err)
	require.Equal(t, total, count)

	for i := range total {
		key := oldDHTValueKey(fmt.Sprintf("/pk/peer-%d", i))
		has, err := ds.Has(ctx, datastore.NewKey(key))
		require.NoError(t, err)
		require.Falsef(t, has, "stale key %d should have been purged", i)
	}
}
