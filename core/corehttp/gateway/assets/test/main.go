package main

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"

	"github.com/ipfs/kubo/core/corehttp/gateway/assets"
)

const (
	directoryTemplateFile = "../directory-index.html"
	dagTemplateFile       = "../dag-index.html"

	testPath = "/ipfs/QmFooBarQXB2mzChmMeKY47C43LxUdg1NDJ5MWcKMKxDu7/a/b/c"
)

var directoryTestData = assets.DirectoryTemplateData{
	GatewayURL: "//localhost:3000",
	DNSLink:    true,
	Listing: []assets.DirectoryItem{{
		Size:      "25 MiB",
		Name:      "short-film.mov",
		Path:      testPath + "/short-film.mov",
		Hash:      "QmbWqxBEKC3P8tqsKc98xmWNzrzDtRLMiMPL8wBuTGsMnR",
		ShortHash: "QmbW\u2026sMnR",
	}, {
		Size:      "23 KiB",
		Name:      "250pxيوسف_الوزاني_صورة_ملتقطة_بواسطة_مرصد_هابل_الفضائي_توضح_سديم_السرطان،_وهو_بقايا_مستعر_أعظم._.jpg",
		Path:      testPath + "/250pxيوسف_الوزاني_صورة_ملتقطة_بواسطة_مرصد_هابل_الفضائي_توضح_سديم_السرطان،_وهو_بقايا_مستعر_أعظم._.jpg",
		Hash:      "QmUwrKrMTrNv8QjWGKMMH5QV9FMPUtRCoQ6zxTdgxATQW6",
		ShortHash: "QmUw\u2026TQW6",
	}, {
		Size:      "1 KiB",
		Name:      "this-piece-of-papers-got-47-words-37-sentences-58-words-we-wanna-know.txt",
		Path:      testPath + "/this-piece-of-papers-got-47-words-37-sentences-58-words-we-wanna-know.txt",
		Hash:      "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		ShortHash: "bafy\u2026bzdi",
	}},
	Size: "25 MiB",
	Path: testPath,
	Breadcrumbs: []assets.Breadcrumb{{
		Name: "ipfs",
	}, {
		Name: "QmFooBarQXB2mzChmMeKY47C43LxUdg1NDJ5MWcKMKxDu7",
		Path: testPath + "/../../..",
	}, {
		Name: "a",
		Path: testPath + "/../..",
	}, {
		Name: "b",
		Path: testPath + "/..",
	}, {
		Name: "c",
		Path: testPath,
	}},
	BackLink: testPath + "/..",
	Hash:     "QmFooBazBar2mzChmMeKY47C43LxUdg1NDJ5MWcKMKxDu7",
}

var dagTestData = assets.DagTemplateData{
	Path:      "/ipfs/baguqeerabn4wonmz6icnk7dfckuizcsf4e4igua2ohdboecku225xxmujepa",
	CID:       "baguqeerabn4wonmz6icnk7dfckuizcsf4e4igua2ohdboecku225xxmujepa",
	CodecName: "dag-json",
	CodecHex:  "0x129",
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dag":
			dagTemplate, err := template.New("dag-index.html").ParseFiles(dagTemplateFile)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to parse template file: %s", err), http.StatusInternalServerError)
				return
			}
			err = dagTemplate.Execute(w, &dagTestData)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to execute template: %s", err), http.StatusInternalServerError)
				return
			}
		case "/directory":
			directoryTemplate, err := template.New("directory-index.html").Funcs(template.FuncMap{
				"iconFromExt": func(name string) string {
					return "ipfs-_blank" // place-holder
				},
				"urlEscape": func(rawUrl string) string {
					pathURL := url.URL{Path: rawUrl}
					return pathURL.String()
				},
			}).ParseFiles(directoryTemplateFile)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to parse template file: %s", err), http.StatusInternalServerError)
				return
			}
			err = directoryTemplate.Execute(w, &directoryTestData)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to execute template: %s", err), http.StatusInternalServerError)
				return
			}
		case "/":
			html := `<p>Test paths: <a href="/dag">DAG</a>, <a href="/directory">Directory</a>.`
			_, _ = w.Write([]byte(html))
		default:
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	})

	if _, err := os.Stat(directoryTemplateFile); err != nil {
		wd, _ := os.Getwd()
		fmt.Printf("could not open template file %q, relative to %q: %s\n", directoryTemplateFile, wd, err)
		os.Exit(1)
	}

	if _, err := os.Stat(dagTemplateFile); err != nil {
		wd, _ := os.Getwd()
		fmt.Printf("could not open template file %q, relative to %q: %s\n", dagTemplateFile, wd, err)
		os.Exit(1)
	}

	fmt.Printf("listening on localhost:3000\n")
	_ = http.ListenAndServe("localhost:3000", mux)
}
