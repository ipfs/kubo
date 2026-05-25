// Windows ulimit handling via SetHandleInformation.
//go:build windows

package util

func init() {
	supportsFDManagement = false
}
