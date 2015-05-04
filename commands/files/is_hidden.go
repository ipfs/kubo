// +build !windows

package files

import (
	"path/filepath"
	"strings"
)

func IsHidden(f File) bool {

	fName := filepath.Base(f.FileName())

	if strings.HasPrefix(fName, ".") {
		return true, nil
	}

	return false, nil
}
