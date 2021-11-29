package assets

import (
	"bytes"
	"io/ioutil"
	"sync"
	"testing"
)

// TestEmbeddedDocs makes sure we don't forget to regenerate after documentation change
func TestEmbeddedDocs(t *testing.T) {
	testNFiles(initDocPaths, 5, t, "documents")
}

func testNFiles(fs []string, wantCnt int, t *testing.T, ftype string) {
	if len(fs) < wantCnt {
		t.Fatalf("expected %d %s. got %d", wantCnt, ftype, len(fs))
	}

	var wg sync.WaitGroup
	for _, f := range fs {
		wg.Add(1)
		// compare asset
		go func(f string) {
			defer wg.Done()
			testOneFile(f, t)
		}(f)
	}
	wg.Wait()
}

func testOneFile(f string, t *testing.T) {
	// load data from filesystem (git)
	vcsData, err := ioutil.ReadFile(f)
	if err != nil {
		t.Errorf("asset %s: could not read vcs file: %s", f, err)
		return
	}

	// load data from emdedded source
	embdData, err := Asset(f)
	if err != nil {
		t.Errorf("asset %s: could not read vcs file: %s", f, err)
		return
	}

	if !bytes.Equal(vcsData, embdData) {
		t.Errorf("asset %s: vcs and embedded data isn't equal", f)
		return
	}

	t.Logf("checked %s", f)
}
