package assets

import (
	"bytes"
	"io/ioutil"
	"sync"
	"testing"
)

// TestEmbeddedDocs makes sure we don't forget to regenerate after documentation change
func TestEmbeddedDocs(t *testing.T) {
	const wantCnt = 6
	if len(initDocPaths) < wantCnt {
		t.Fatalf("expected %d documents got %d", wantCnt, len(initDocPaths))
	}

	var wg sync.WaitGroup
	for _, f := range initDocPaths {
		wg.Add(1)
		// compare asset
		go func(f string) {
			defer wg.Done()
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
				t.Errorf("asset %s: vcs and embedded data isnt equal", f)
				return
			}

			t.Logf("checked %s", f)
		}(f)
	}
	wg.Wait()
}

func TestGatewayAssets(t *testing.T) {
	const wantCnt = 2
	if len(initGwAssets) < wantCnt {
		t.Fatalf("expected %d assets. got %d", wantCnt, len(initDocPaths))
	}

	for _, f := range initGwAssets {
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
			t.Errorf("asset %s: vcs and embedded data isnt equal", f)
			return
		}

		t.Logf("checked %s", f)
	}

}
