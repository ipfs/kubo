// Stub returning zero on platforms without /proc or Handle APIs.
//go:build !linux && !darwin && !windows

package fd

func GetNumFDs() int {
	return 0
}
