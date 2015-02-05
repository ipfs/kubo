package importer

import (
	"math"
	"testing"

	humanize "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/dustin/go-humanize"
)

func TestCalculateSizes(t *testing.T) {

	//  d := ((lbs/271) ^ layer) * dbs

	increments := func(a, b int) []int {
		ints := []int{}
		for ; a <= b; a *= 2 {
			ints = append(ints, a)
		}
		return ints
	}

	layers := 7
	roughLinkSize := roughLinkSize // from importer pkg
	dataBlockSizes := increments(1<<12, 1<<18)
	linkBlockSizes := increments(1<<12, 1<<14)

	t.Logf("rough link size:  %d", roughLinkSize)
	t.Logf("data block sizes: %v", dataBlockSizes)
	t.Logf("link block sizes: %v", linkBlockSizes)
	for _, dbs := range dataBlockSizes {
		t.Logf("")
		t.Logf("with data block size: %d", dbs)
		for _, lbs := range linkBlockSizes {
			t.Logf("")
			t.Logf("\twith data block size: %d", dbs)
			t.Logf("\twith link block size: %d", lbs)

			lpb := lbs / roughLinkSize
			t.Logf("\tlinks per block: %d", lpb)

			for l := 1; l < layers; l++ {
				total := int(math.Pow(float64(lpb), float64(l))) * dbs
				htotal := humanize.Bytes(uint64(total))
				t.Logf("\t\t\tlayer %d: %s\t%d", l, htotal, total)
			}

		}
	}

}
