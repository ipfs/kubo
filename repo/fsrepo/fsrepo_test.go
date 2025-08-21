package fsrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	datastore "github.com/ipfs/go-datastore"
	config "github.com/ipfs/kubo/config"
	"github.com/stretchr/testify/require"
)

func TestInitIdempotence(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	for i := 0; i < 10; i++ {
		require.NoError(t, Init(path, &config.Config{Datastore: config.DefaultDatastoreConfig()}), "multiple calls to init should succeed")
	}
}

func Remove(repoPath string) error {
	repoPath = filepath.Clean(repoPath)
	return os.RemoveAll(repoPath)
}

func TestCanManageReposIndependently(t *testing.T) {
	t.Parallel()
	pathA := t.TempDir()
	pathB := t.TempDir()

	t.Log("initialize two repos")
	require.NoError(t, Init(pathA, &config.Config{Datastore: config.DefaultDatastoreConfig()}), "a", "should initialize successfully")
	require.NoError(t, Init(pathB, &config.Config{Datastore: config.DefaultDatastoreConfig()}), "b", "should initialize successfully")

	t.Log("ensure repos initialized")
	require.True(t, IsInitialized(pathA), "a should be initialized")
	require.True(t, IsInitialized(pathB), "b should be initialized")

	t.Log("open the two repos")
	repoA, err := Open(pathA)
	require.NoError(t, err, "a")
	repoB, err := Open(pathB)
	require.NoError(t, err, "b")

	t.Log("close and remove b while a is open")
	require.NoError(t, repoB.Close(), "close b")
	require.NoError(t, Remove(pathB), "remove b")

	t.Log("close and remove a")
	require.NoError(t, repoA.Close())
	require.NoError(t, Remove(pathA))
}

func TestDatastoreGetNotAllowedAfterClose(t *testing.T) {
	t.Parallel()
	path := t.TempDir()

	require.False(t, IsInitialized(path), "should NOT be initialized")
	require.NoError(t, Init(path, &config.Config{Datastore: config.DefaultDatastoreConfig()}), "should initialize successfully")
	r, err := Open(path)
	require.NoError(t, err, "should open successfully")

	k := "key"
	data := []byte(k)
	require.NoError(t, r.Datastore().Put(context.Background(), datastore.NewKey(k), data), "Put should be successful")

	require.NoError(t, r.Close())
	_, err = r.Datastore().Get(context.Background(), datastore.NewKey(k))
	require.Error(t, err, "after closer, Get should be fail")
}

func TestDatastorePersistsFromRepoToRepo(t *testing.T) {
	t.Parallel()
	path := t.TempDir()

	require.NoError(t, Init(path, &config.Config{Datastore: config.DefaultDatastoreConfig()}))
	r1, err := Open(path)
	require.NoError(t, err)

	k := "key"
	expected := []byte(k)
	require.NoError(t, r1.Datastore().Put(context.Background(), datastore.NewKey(k), expected), "using first repo, Put should be successful")
	require.NoError(t, r1.Close())

	r2, err := Open(path)
	require.NoError(t, err)
	actual, err := r2.Datastore().Get(context.Background(), datastore.NewKey(k))
	require.NoError(t, err, "using second repo, Get should be successful")
	require.NoError(t, r2.Close())
	require.True(t, bytes.Equal(expected, actual), "data should match")
}

func TestOpenMoreThanOnceInSameProcess(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	require.NoError(t, Init(path, &config.Config{Datastore: config.DefaultDatastoreConfig()}))

	r1, err := Open(path)
	require.NoError(t, err, "first repo should open successfully")
	r2, err := Open(path)
	require.NoError(t, err, "second repo should open successfully")
	require.Equal(t, r1, r2, "second open returns same value")

	require.NoError(t, r1.Close())
	require.NoError(t, r2.Close())
}
