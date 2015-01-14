package fsrepo

import (
	"io/ioutil"
	"testing"

	"github.com/jbenet/go-ipfs/repo/config"
)

func testRepoPath(p string, t *testing.T) string {
	name, err := ioutil.TempDir("", p)
	if err != nil {
		t.Fatal(err)
	}
	return name
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
