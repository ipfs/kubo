//go:generate go-bindata -pkg=assets init-doc

package assets

var InitDir = map[string][]byte{
	"about":          MustAsset("init-doc/about"),
	"readme":         MustAsset("init-doc/readme"),
	"help":           MustAsset("init-doc/help"),
	"contact":        MustAsset("init-doc/contact"),
	"security-notes": MustAsset("init-doc/security-notes"),
	"quick-start":    MustAsset("init-doc/quick-start"),
}
