//go:generate git submodule update --init ./dir-index-html
//go:generate go run github.com/go-bindata/go-bindata/v3/go-bindata -mode=0644 -modtime=1403768328 -pkg=assets init-doc dir-index-html/dir-index.html dir-index-html/knownIcons.txt
//go:generate gofmt -w bindata.go
//go:generate sh -c "sed -i \"s/.*BindataVersionHash.*/BindataVersionHash=\\\"$(git hash-object bindata.go)\\\"/\" bindata_version_hash.go"
//go:generate gofmt -w bindata_version_hash.go
package assets

import (
	"fmt"
	"path/filepath"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"

	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

// initDocPaths lists the paths for the docs we want to seed during --init
var initDocPaths = []string{
	filepath.Join("init-doc", "about"),
	filepath.Join("init-doc", "readme"),
	filepath.Join("init-doc", "help"),
	filepath.Join("init-doc", "contact"),
	filepath.Join("init-doc", "security-notes"),
	filepath.Join("init-doc", "quick-start"),
	filepath.Join("init-doc", "ping"),
}

// SeedInitDocs adds the list of embedded init documentation to the passed node, pins it and returns the root key
func SeedInitDocs(nd *core.IpfsNode) (cid.Cid, error) {
	return addAssetList(nd, initDocPaths)
}

func addAssetList(nd *core.IpfsNode, l []string) (cid.Cid, error) {
	api, err := coreapi.NewCoreAPI(nd)
	if err != nil {
		return cid.Cid{}, err
	}

	dirb, err := api.Object().New(nd.Context(), options.Object.Type("unixfs-dir"))
	if err != nil {
		return cid.Cid{}, err
	}

	basePath := path.IpfsPath(dirb.Cid())

	for _, p := range l {
		d, err := Asset(p)
		if err != nil {
			return cid.Cid{}, fmt.Errorf("assets: could load Asset '%s': %s", p, err)
		}

		fp, err := api.Unixfs().Add(nd.Context(), files.NewBytesFile(d))
		if err != nil {
			return cid.Cid{}, err
		}

		fname := filepath.Base(p)

		basePath, err = api.Object().AddLink(nd.Context(), basePath, fname, fp)
		if err != nil {
			return cid.Cid{}, err
		}
	}

	if err := api.Pin().Add(nd.Context(), basePath); err != nil {
		return cid.Cid{}, err
	}

	return basePath.Cid(), nil
}
