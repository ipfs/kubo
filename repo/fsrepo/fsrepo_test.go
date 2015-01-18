package fsrepo

import (
	"bytes"
	"io/ioutil"
	"testing"

	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/jbenet/go-ipfs/repo/config"
	"github.com/jbenet/go-ipfs/thirdparty/assert"
)

// swap arg order
func testRepoPath(p string, t *testing.T) string {
	name, err := ioutil.TempDir("", p)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestInitIdempotence(t *testing.T) {
	t.Parallel()
	path := testRepoPath("", t)
	for i := 0; i < 10; i++ {
		assert.Nil(Init(path, &config.Config{}), t, "multiple calls to init should succeed")
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()
	path := testRepoPath("foo", t)
	assert.Nil(Remove(path), t, "should be able to remove after closed")
}

func TestCannotRemoveIfOpen(t *testing.T) {
	t.Parallel()
	path := testRepoPath("TestCannotRemoveIfOpen", t)
	assert.Nil(Init(path, &config.Config{}), t, "should initialize successfully")
	r := At(path)
	assert.Nil(r.Open(), t)
	assert.Err(Remove(path), t, "should not be able to remove while open")
	assert.Nil(r.Close(), t)
	assert.Nil(Remove(path), t, "should be able to remove after closed")
}

func TestCannotBeReopened(t *testing.T) {
	t.Parallel()
	path := testRepoPath("", t)
	assert.Nil(Init(path, &config.Config{}), t)
	r := At(path)
	assert.Nil(r.Open(), t)
	assert.Nil(r.Close(), t)
	assert.Err(r.Open(), t, "shouldn't be possible to re-open the repo")

	// mutable state is the enemy. Take Close() as an opportunity to reduce
	// entropy. Callers ought to start fresh with a new handle by calling `At`.
}

func TestCanManageReposIndependently(t *testing.T) {
	t.Parallel()
	pathA := testRepoPath("a", t)
	pathB := testRepoPath("b", t)

	t.Log("initialize two repos")
	assert.Nil(Init(pathA, &config.Config{}), t, "a", "should initialize successfully")
	assert.Nil(Init(pathB, &config.Config{}), t, "b", "should initialize successfully")

	t.Log("ensure repos initialized")
	assert.True(IsInitialized(pathA), t, "a should be initialized")
	assert.True(IsInitialized(pathB), t, "b should be initialized")

	t.Log("open the two repos")
	repoA := At(pathA)
	repoB := At(pathB)
	assert.Nil(repoA.Open(), t, "a")
	assert.Nil(repoB.Open(), t, "b")

	t.Log("close and remove b while a is open")
	assert.Nil(repoB.Close(), t, "close b")
	assert.Nil(Remove(pathB), t, "remove b")

	t.Log("close and remove a")
	assert.Nil(repoA.Close(), t)
	assert.Nil(Remove(pathA), t)
}

func TestDatastoreGetNotAllowedAfterClose(t *testing.T) {
	t.Parallel()
	path := testRepoPath("test", t)

	assert.True(!IsInitialized(path), t, "should NOT be initialized")
	assert.Nil(Init(path, &config.Config{}), t, "should initialize successfully")
	r := At(path)
	assert.Nil(r.Open(), t, "should open successfully")

	k := "key"
	data := []byte(k)
	assert.Nil(r.Datastore().Put(datastore.NewKey(k), data), t, "Put should be successful")

	assert.Nil(r.Close(), t)
	_, err := r.Datastore().Get(datastore.NewKey(k))
	assert.Err(err, t, "after closer, Get should be fail")
}

func TestDatastorePersistsFromRepoToRepo(t *testing.T) {
	t.Parallel()
	path := testRepoPath("test", t)

	assert.Nil(Init(path, &config.Config{}), t)
	r1 := At(path)
	assert.Nil(r1.Open(), t)

	k := "key"
	expected := []byte(k)
	assert.Nil(r1.Datastore().Put(datastore.NewKey(k), expected), t, "using first repo, Put should be successful")
	assert.Nil(r1.Close(), t)

	r2 := At(path)
	assert.Nil(r2.Open(), t)
	v, err := r2.Datastore().Get(datastore.NewKey(k))
	assert.Nil(err, t, "using second repo, Get should be successful")
	actual, ok := v.([]byte)
	assert.True(ok, t, "value should be the []byte from r1's Put")
	assert.Nil(r2.Close(), t)
	assert.True(bytes.Compare(expected, actual) == 0, t, "data should match")
}

func TestOpenMoreThanOnceInSameProcess(t *testing.T) {
	t.Parallel()
	path := testRepoPath("", t)
	assert.Nil(Init(path, &config.Config{}), t)

	r1 := At(path)
	r2 := At(path)
	assert.Nil(r1.Open(), t, "first repo should open successfully")
	assert.Nil(r2.Open(), t, "second repo should open successfully")

	assert.Nil(r1.Close(), t)
	assert.Nil(r2.Close(), t)
}
