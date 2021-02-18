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
	DNSLink     bool
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

func breadcrumbs(urlPath string, dnslinkOrigin bool) []breadcrumb {
	var ret []breadcrumb

	p, err := ipfspath.ParsePath(urlPath)
	if err != nil {
		// No breadcrumbs, fallback to bare Path in template
		return ret
	}
	segs := p.Segments()
	contentRoot := segs[1]
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

	// Drop the /ipns/<fqdn> prefix from breadcrumb Paths when directory
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
		// Make contentRoot breadcrumb link to the website root
		ret[1].Path = "/"
	}

	return ret
}

func shortHash(hash string) string {
	return (hash[0:4] + "\u2026" + hash[len(hash)-4:])
}

// helper to detect DNSLink website context
// (when hostname from gwURL is matching /ipns/<fqdn> in path)
func hasDNSLinkOrigin(gwURL string, path string) bool {
	if gwURL != "" {
		fqdn := stripPort(strings.TrimPrefix(gwURL, "//"))
		return strings.HasPrefix(path, "/ipns/"+fqdn)
	}
	return false
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
