//go:build windows

package fd

import (
	"math"
)

func GetNumFDs() int {
	return math.MaxInt
}
