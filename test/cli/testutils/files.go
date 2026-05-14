package testutils

import (
	"log"
	"os"
	"path/filepath"
)

func MustOpen(name string) *os.File {
	f, err := os.Open(name)
	if err != nil {
		log.Panicf("opening %s: %s", name, err)
	}
	return f
}

// Searches for a file in a dir, then the parent dir, etc.
// If the file is not found, an empty string is returned.
func FindUp(name, dir string) string {
	curDir := dir
	for {
		entries, err := os.ReadDir(curDir)
		if err != nil {
			panic(err)
		}
		for _, e := range entries {
			if name == e.Name() {
				return filepath.Join(curDir, name)
			}
		}
		newDir := filepath.Dir(curDir)
		if newDir == curDir {
			return ""
		}
		curDir = newDir
	}
}
