//go:build windows
// +build windows

package util

func init() {
	supportsFDManagement = false
}
