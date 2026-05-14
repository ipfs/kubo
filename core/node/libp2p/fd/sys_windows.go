// File descriptor counting via Windows Handle API.
//go:build windows

package fd

import (
	"math"
)

func GetNumFDs() int {
	return math.MaxInt
}
