//go:generate ./build.sh
package assets

import (
	"embed"
	"io"
	"io/fs"
	"net"
	"strconv"

	"html/template"
	"net/url"
	"path"
	"strings"

	"github.com/cespare/xxhash"

	ipfspath "github.com/ipfs/go-path"
)

//go:embed dag-index.html directory-index.html knownIcons.txt
var asset embed.FS

// AssetHash a non-cryptographic hash of all embedded assets
var AssetHash string

var (
	DirectoryTemplate *template.Template
	DagTemplate       *template.Template
)

func init() {
	initAssetsHash()
	initTemplates()
}

func initAssetsHash() {
	sum := xxhash.New()
	err := fs.WalkDir(asset, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		file, err := asset.Open(path)
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

func initTemplates() {
	knownIconsBytes, err := asset.ReadFile("knownIcons.txt")
	if err != nil {
		panic(err)
	}
	knownIcons := make(map[string]struct{})
	for _, ext := range strings.Split(strings.TrimSuffix(string(knownIconsBytes), "\n"), "\n") {
		knownIcons[ext] = struct{}{}
	}

	// helper to guess the type/icon for it by the extension name
	iconFromExt := func(name string) string {
		ext := path.Ext(name)
		_, ok := knownIcons[ext]
		if !ok {
			// default blank icon
			return "ipfs-_blank"
		}
		return "ipfs-" + ext[1:] // slice of the first dot
	}

	// custom template-escaping function to escape a full path, including '#' and '?'
	urlEscape := func(rawUrl string) string {
		pathURL := url.URL{Path: rawUrl}
		return pathURL.String()
	}

	// Directory listing template
	dirIndexBytes, err := asset.ReadFile("directory-index.html")
	if err != nil {
		panic(err)
	}

	DirectoryTemplate = template.Must(template.New("dir").Funcs(template.FuncMap{
		"iconFromExt": iconFromExt,
		"urlEscape":   urlEscape,
	}).Parse(string(dirIndexBytes)))

	// DAG Index template
	dagIndexBytes, err := asset.ReadFile("dag-index.html")
	if err != nil {
		panic(err)
	}

	DagTemplate = template.Must(template.New("dir").Parse(string(dagIndexBytes)))
}

type DagTemplateData struct {
	Path      string
	CID       string
	CodecName string
	CodecHex  string
}

type DirectoryTemplateData struct {
	GatewayURL  string
	DNSLink     bool
	Listing     []DirectoryItem
	Size        string
	Path        string
	Breadcrumbs []Breadcrumb
	BackLink    string
	Hash        string
}

type DirectoryItem struct {
	Size      string
	Name      string
	Path      string
	Hash      string
	ShortHash string
}

type Breadcrumb struct {
	Name string
	Path string
}

func Breadcrumbs(urlPath string, dnslinkOrigin bool) []Breadcrumb {
	var ret []Breadcrumb

	p, err := ipfspath.ParsePath(urlPath)
	if err != nil {
		// No assets.Breadcrumbs, fallback to bare Path in template
		return ret
	}
	segs := p.Segments()
	contentRoot := segs[1]
	for i, seg := range segs {
		if i == 0 {
			ret = append(ret, Breadcrumb{Name: seg})
		} else {
			ret = append(ret, Breadcrumb{
				Name: seg,
				Path: "/" + strings.Join(segs[0:i+1], "/"),
			})
		}
	}

	// Drop the /ipns/<fqdn> prefix from assets.Breadcrumb Paths when directory
	// listing on a DNSLink website (loaded due to Host header in HTTP
	// request).  Necessary because the hostname most likely won't have a
	// public gateway mounted.
	if dnslinkOrigin {
		prefix := "/ipns/" + contentRoot
		for i, crumb := range ret {
			if strings.HasPrefix(crumb.Path, prefix) {
				ret[i].Path = strings.Replace(crumb.Path, prefix, "", 1)
			}
		}
		// Make contentRoot assets.Breadcrumb link to the website root
		ret[1].Path = "/"
	}

	return ret
}

func ShortHash(hash string) string {
	if len(hash) <= 8 {
		return hash
	}
	return (hash[0:4] + "\u2026" + hash[len(hash)-4:])
}

// helper to detect DNSLink website context
// (when hostname from gwURL is matching /ipns/<fqdn> in path)
func HasDNSLinkOrigin(gwURL string, path string) bool {
	if gwURL != "" {
		fqdn := stripPort(strings.TrimPrefix(gwURL, "//"))
		return strings.HasPrefix(path, "/ipns/"+fqdn)
	}
	return false
}

func stripPort(hostname string) string {
	host, _, err := net.SplitHostPort(hostname)
	if err == nil {
		return host
	}
	return hostname
}
