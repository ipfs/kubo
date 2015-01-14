package component

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jbenet/go-ipfs/repo/fsrepo/assert"
)

// swap arg order
func testRepoPath(t *testing.T, path ...string) string {
	name, err := ioutil.TempDir("", filepath.Join(path...))
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestOpenMoreThanOnceInSameProcess(t *testing.T) {
	t.Parallel()
	path := testRepoPath(t)
	dsc1 := DatastoreComponent{path: path}
	dsc2 := DatastoreComponent{path: path}
	assert.Nil(dsc1.Open(), t, "first repo should open successfully")
	assert.Nil(dsc2.Open(), t, "second repo should open successfully")

	assert.Nil(dsc1.Close(), t)
	assert.Nil(dsc2.Close(), t)
}
