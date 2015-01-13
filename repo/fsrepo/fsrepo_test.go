package fsrepo

import (
	"os"
	"path"
	"testing"

	"github.com/jbenet/go-ipfs/repo/config"
)

// NB: These tests cannot be run in parallel

func init() {
	// ensure tests begin in clean state
	os.RemoveAll(testRepoDir)
}

const testRepoDir = "./fsrepo_test/repos"

func testRepoPath(p string) string {
	return path.Join(testRepoDir, p)
}

func TestCannotRemoveIfOpen(t *testing.T) {
	path := testRepoPath("TestCannotRemoveIfOpen")
	AssertNil(Init(path, &config.Config{}), t, "should initialize successfully")
	r := At(path)
	AssertNil(r.Open(), t)
	AssertErr(Remove(path), t, "should not be able to remove while open")
	AssertNil(r.Close(), t)
	AssertNil(Remove(path), t, "should be able to remove after closed")
}

func TestCanManageReposIndependently(t *testing.T) {
	pathA := testRepoPath("a")
	pathB := testRepoPath("b")

	t.Log("initialize two repos")
	AssertNil(Init(pathA, &config.Config{}), t, "should initialize successfully")
	AssertNil(Init(pathB, &config.Config{}), t, "should initialize successfully")

	t.Log("open the two repos")
	repoA := At(pathA)
	repoB := At(pathB)
	AssertNil(repoA.Open(), t)
	AssertNil(repoB.Open(), t)

	t.Log("close and remove b while a is open")
	AssertNil(repoB.Close(), t, "close b")
	AssertNil(Remove(pathB), t, "remove b")

	t.Log("close and remove a")
	AssertNil(repoA.Close(), t)
	AssertNil(Remove(pathA), t)
}

func AssertNil(err error, t *testing.T, msgs ...string) {
	if err != nil {
		t.Error(msgs, "error:", err)
	}
}

func AssertErr(err error, t *testing.T, msgs ...string) {
	if err == nil {
		t.Error(msgs, "error:", err)
	}
}
