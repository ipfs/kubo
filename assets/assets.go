package assets

import (
	"embed"
	"fmt"
	gopath "path"

	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/files"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

//go:embed init-doc
var Asset embed.FS

// initDocPaths lists the paths for the docs we want to seed during --init
var initDocPaths = []string{
	gopath.Join("init-doc", "about"),
	gopath.Join("init-doc", "readme"),
	gopath.Join("init-doc", "help"),
	gopath.Join("init-doc", "contact"),
	gopath.Join("init-doc", "security-notes"),
	gopath.Join("init-doc", "quick-start"),
	gopath.Join("init-doc", "ping"),
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
		d, err := Asset.ReadFile(p)
		if err != nil {
			return cid.Cid{}, fmt.Errorf("assets: could load Asset '%s': %s", p, err)
		}

		fp, err := api.Unixfs().Add(nd.Context(), files.NewBytesFile(d))
		if err != nil {
			return cid.Cid{}, err
		}

		fname := gopath.Base(p)

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
