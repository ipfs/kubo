package bitswap

import (
	"math"
	"math/rand"
)

type StrategyFunc func(debtRatio) bool

func StandardStrategy(db debtRatio) bool {
	return rand.Float64() <= probabilitySend(db.Value())
}

func YesManStrategy(db debtRatio) bool {
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
