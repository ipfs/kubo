//go:build !windows
// +build !windows

package util

func InsideGUI() bool {
	return false
}
