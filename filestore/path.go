// +build !windows

package filestore

import (
	"bytes"
	"os"
	"syscall"
)

// Safely cleans a unix style path

// Unlike filepath.Clean it does not remove any "/../" as removing
// those correctly involves resolving symblic links

func CleanPath(pathStr string) string {
	if pathStr == "" {
		return ""
	}
	path := []byte(pathStr)
	buf := new(bytes.Buffer)
	buf.Grow(len(path))
	buf.WriteByte(path[0])
	for i := 1; i < len(path); i++ {
		if path[i] == '/' && path[i-1] == '/' {
			// skip
		} else if path[i] == '.' && path[i-1] == '/' && i+1 < len(path) && path[i+1] == '/' {
			// skip 2 bytes
			i++
		} else {
			buf.WriteByte(path[i])
		}
	}
	res := buf.String()
	if pathStr == res {
		return pathStr
	} else {
		return res
	}
}

func SystemWd() (string, error) {
	return syscall.Getwd()
}

func EnvWd() (string, error) {
	dot, err := os.Stat(".")
	if err != nil {
		return "", err
	}
	dir := os.Getenv("PWD")
	if len(dir) > 0 && dir[0] == '/' {
		d, err := os.Stat(dir)
		if err == nil && os.SameFile(dot, d) {
			return dir, nil
		}
	}
	return SystemWd()
}

func AbsPath(dir string, file string) (string,error) {
	if file[0] == '/' {
		return CleanPath(file), nil
	}
	return CleanPath(dir + "/" + file), nil
}
