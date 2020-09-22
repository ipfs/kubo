package corehttp

import (
	"html/template"
	"net/url"
	"path"
	"strings"

	"github.com/ipfs/go-ipfs/assets"
	ipfspath "github.com/ipfs/go-path"
)

// structs for directory listing
type listingTemplateData struct {
	GatewayURL  string
	Listing     []directoryItem
	Size        string
	Path        string
	Breadcrumbs []breadcrumb
	BackLink    string
	Hash        string
}

type directoryItem struct {
	Size      string
	Name      string
	Path      string
	Hash      string
	ShortHash string
}

type breadcrumb struct {
	Name string
	Path string
}

func breadcrumbs(urlPath string) []breadcrumb {
	var ret []breadcrumb

	p, err := ipfspath.ParsePath(urlPath)
	if err != nil {
		// No breadcrumbs, fallback to bare Path in template
		return ret
	}

	segs := p.Segments()
	for i, seg := range segs {
		if i == 0 {
			ret = append(ret, breadcrumb{Name: seg})
		} else {
			ret = append(ret, breadcrumb{
				Name: seg,
				Path: "/" + strings.Join(segs[0:i+1], "/"),
			})
		}
	}

	return ret
}

func shortHash(hash string) string {
	return (hash[0:4] + "\u2026" + hash[len(hash)-4:])
}

var listingTemplate *template.Template

func init() {
	knownIconsBytes, err := assets.Asset("dir-index-html/knownIcons.txt")
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
		pathUrl := url.URL{Path: rawUrl}
		return pathUrl.String()
	}

	// Directory listing template
	dirIndexBytes, err := assets.Asset("dir-index-html/dir-index.html")
	if err != nil {
		panic(err)
	}

	listingTemplate = template.Must(template.New("dir").Funcs(template.FuncMap{
		"iconFromExt": iconFromExt,
		"urlEscape":   urlEscape,
	}).Parse(string(dirIndexBytes)))
}
