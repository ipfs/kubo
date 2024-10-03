//go:build !linux && !darwin && !windows

package fd

func GetNumFDs() int {
	return 0
}
