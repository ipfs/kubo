//go:generate go-bindata -pkg=assets -prefix=$GOPATH/src/gx/ipfs/QmT1jwrqzSMjSjLG5oBd9w4P9vXPKQksWuf5ghsE3Q88ZV init-doc $GOPATH/src/gx/ipfs/QmT1jwrqzSMjSjLG5oBd9w4P9vXPKQksWuf5ghsE3Q88ZV/dir-index-html
//go:generate gofmt -w bindata.go

package assets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"

	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	options "github.com/ipfs/interface-go-ipfs-core/options"

	// this import keeps gx from thinking the dep isn't used
	_ "github.com/ipfs/dir-index-html"
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

var initDirPath = filepath.Join(os.Getenv("GOPATH"), "gx", "ipfs", "QmT1jwrqzSMjSjLG5oBd9w4P9vXPKQksWuf5ghsE3Q88ZV", "dir-index-html")
var initDirIndex = []string{
	filepath.Join(initDirPath, "knownIcons.txt"),
	filepath.Join(initDirPath, "dir-index.html"),
}

func SeedInitDirIndex(nd *core.IpfsNode) (cid.Cid, error) {
	return addAssetList(nd, initDirIndex)
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

	basePath := iface.IpfsPath(dirb.Cid())

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
