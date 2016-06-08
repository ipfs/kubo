// +build windows

package filestore

import (
	"errors"
	"syscall"
)

func CleanPath(pathStr string) string {
	return pathStr
}

func SystemWd() (string, error) {
	return ".", nil
}

func EnvWd() (string, error) {
	return ".", nil
}

func AbsPath(dir string, file string) (string, error) {
	if dir != "." {
		return "", errors.New("AbsPath: dir must be '.' on windows")
	}
	return syscall.FullPath(file)
}
