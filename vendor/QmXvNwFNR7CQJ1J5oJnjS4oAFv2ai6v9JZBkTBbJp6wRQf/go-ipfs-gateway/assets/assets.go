//go:generate go-bindata -pkg=assets ../vendor/dir-index-html-v1.0.0/...
//go:generate gofmt -w bindata.go

package assets

import (
	"fmt"
	"path/filepath"
)

var initDirIndex = []string{
	filepath.Join("..", "vendor", "dir-index-html-v1.0.0", "knownIcons.txt"),
	filepath.Join("..", "vendor", "dir-index-html-v1.0.0", "dir-index.html"),
}

func seedInitDirIndex() error {
	for _, p := range initDirIndex {
		_, err := Asset(p)
		if err != nil {
			return fmt.Errorf("assets: could load Asset '%s': %s", p, err)
		}
	}
	return nil
}
