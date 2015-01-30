//go:generate doc2go -in=init-doc/readme -out=readme.go -package=assets
//go:generate doc2go -in=init-doc/help -out=help.go -package=assets
//go:generate doc2go -in=init-doc/contact -out=contact.go -package=assets
//go:generate doc2go -in=init-doc/security-notes -out=security-notes.go -package=assets
//go:generate doc2go -in=init-doc/quick-start -out=quick-start.go -package=assets
package assets

var Init_dir = map[string]string{
	"readme":         Init_doc_readme,
	"help":           Init_doc_help,
	"contact":        Init_doc_contact,
	"security-notes": Init_doc_security_notes,
	"quick-start":    Init_doc_quick_start,
}
