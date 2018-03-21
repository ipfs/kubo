// +build !windows,!darwin

package tar

import "path/filepath"

func platformSanitize(pathElements []string) string {
	return filepath.Join(pathElements...)
}
