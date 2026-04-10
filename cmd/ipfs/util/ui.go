// GUI detection stub. Windows has its own implementation.
//go:build !windows

package util

func InsideGUI() bool {
	return false
}
