package strategy

import (
	"math"
	"math/rand"
)

type strategyFunc func(*ledger) bool

// TODO avoid using rand.Float64 method. it uses a singleton lock and may cause
// performance issues. Instead, instantiate a rand struct and use that to call
// Float64()
func standardStrategy(l *ledger) bool {
	return rand.Float64() <= probabilitySend(l.Accounting.Value())
}

func yesManStrategy(l *ledger) bool {
	return true
}

func probabilitySend(ratio float64) float64 {
	x := 1 + math.Exp(6-3*ratio)
	y := 1 / x
	return 1 - y
}

type debtRatio struct {
	BytesSent uint64
	BytesRecv uint64
}

func (dr *debtRatio) Value() float64 {
	return float64(dr.BytesSent) / float64(dr.BytesRecv+1)
}
