//go:generate npm run build --prefix ./dir-index-html/
package assets

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strconv"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"

	"github.com/cespare/xxhash"
	cid "github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

//go:embed init-doc dir-index-html/dir-index.html dir-index-html/knownIcons.txt
var Asset embed.FS

// AssetHash a non-cryptographic hash of all embedded assets
var AssetHash string

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

func init() {
	sum := xxhash.New()
	err := fs.WalkDir(Asset, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		file, err := Asset.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(sum, file)
		return err
	})
	if err != nil {
		panic("error creating asset sum: " + err.Error())
	}

	AssetHash = strconv.FormatUint(sum.Sum64(), 32)
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
