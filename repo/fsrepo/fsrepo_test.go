package fsrepo

import (
	"bytes"
	"io/ioutil"
	"testing"

	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/jbenet/go-ipfs/repo/config"
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
	path := testRepoPath("", t)
	for i := 0; i < 10; i++ {
		AssertNil(Init(path, &config.Config{}), t, "multiple calls to init should succeed")
	}
}

func TestRemove(t *testing.T) {
	path := testRepoPath("foo", t)
	AssertNil(Remove(path), t, "should be able to remove after closed")
}

func TestCannotRemoveIfOpen(t *testing.T) {
	path := testRepoPath("TestCannotRemoveIfOpen", t)
	AssertNil(Init(path, &config.Config{}), t, "should initialize successfully")
	r := At(path)
	AssertNil(r.Open(), t)
	AssertErr(Remove(path), t, "should not be able to remove while open")
	AssertNil(r.Close(), t)
	AssertNil(Remove(path), t, "should be able to remove after closed")
}

func TestCannotBeReopened(t *testing.T) {
	path := testRepoPath("", t)
	AssertNil(Init(path, &config.Config{}), t)
	r := At(path)
	AssertNil(r.Open(), t)
	AssertNil(r.Close(), t)
	AssertErr(r.Open(), t, "shouldn't be possible to re-open the repo")

	// mutable state is the enemy. Take Close() as an opportunity to reduce
	// entropy. Callers ought to start fresh with a new handle by calling `At`.
}

func TestCanManageReposIndependently(t *testing.T) {
	pathA := testRepoPath("a", t)
	pathB := testRepoPath("b", t)

	t.Log("initialize two repos")
	AssertNil(Init(pathA, &config.Config{}), t, "a", "should initialize successfully")
	AssertNil(Init(pathB, &config.Config{}), t, "b", "should initialize successfully")

	t.Log("ensure repos initialized")
	Assert(IsInitialized(pathA), t, "a should be initialized")
	Assert(IsInitialized(pathB), t, "b should be initialized")

	t.Log("open the two repos")
	repoA := At(pathA)
	repoB := At(pathB)
	AssertNil(repoA.Open(), t, "a")
	AssertNil(repoB.Open(), t, "b")

	t.Log("close and remove b while a is open")
	AssertNil(repoB.Close(), t, "close b")
	AssertNil(Remove(pathB), t, "remove b")

	t.Log("close and remove a")
	AssertNil(repoA.Close(), t)
	AssertNil(Remove(pathA), t)
}

func TestDatastoreGetNotAllowedAfterClose(t *testing.T) {
	path := testRepoPath("test", t)

	Assert(!IsInitialized(path), t, "should NOT be initialized")
	AssertNil(Init(path, &config.Config{}), t, "should initialize successfully")
	r := At(path)
	AssertNil(r.Open(), t, "should open successfully")

	k := "key"
	data := []byte(k)
	AssertNil(r.Datastore().Put(datastore.NewKey(k), data), t, "Put should be successful")

	AssertNil(r.Close(), t)
	_, err := r.Datastore().Get(datastore.NewKey(k))
	AssertErr(err, t, "after closer, Get should be fail")
}

func TestDatastorePersistsFromRepoToRepo(t *testing.T) {
	path := testRepoPath("test", t)

	AssertNil(Init(path, &config.Config{}), t)
	r1 := At(path)
	AssertNil(r1.Open(), t)

	k := "key"
	expected := []byte(k)
	AssertNil(r1.Datastore().Put(datastore.NewKey(k), expected), t, "using first repo, Put should be successful")
	AssertNil(r1.Close(), t)

	r2 := At(path)
	AssertNil(r2.Open(), t)
	v, err := r2.Datastore().Get(datastore.NewKey(k))
	AssertNil(err, t, "using second repo, Get should be successful")
	actual, ok := v.([]byte)
	Assert(ok, t, "value should be the []byte from r1's Put")
	AssertNil(r2.Close(), t)
	Assert(bytes.Compare(expected, actual) == 0, t, "data should match")
}

func AssertNil(err error, t *testing.T, msgs ...string) {
	if err != nil {
		t.Fatal(msgs, "error:", err)
	}
}

func Assert(v bool, t *testing.T, msgs ...string) {
	if !v {
		t.Fatal(msgs)
	}
}

func AssertErr(err error, t *testing.T, msgs ...string) {
	if err == nil {
		t.Fatal(msgs, "error:", err)
	}
}
