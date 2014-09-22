package strategy

import (
	"testing"
)

func TestProbabilitySendDecreasesAsRatioIncreases(t *testing.T) {
	grateful := debtRatio{BytesSent: 0, BytesRecv: 10000}
	pWhenGrateful := probabilitySend(grateful.Value())

	abused := debtRatio{BytesSent: 10000, BytesRecv: 0}
	pWhenAbused := probabilitySend(abused.Value())

	if pWhenGrateful < pWhenAbused {
		t.Fail()
	}
}
