package tar

import (
	"path/filepath"
	"strings"
)

func platformSanitize(pathElements []string) string {
	res := filepath.Join(pathElements...)
	return strings.Replace(res, ":", "-", -1)
}
