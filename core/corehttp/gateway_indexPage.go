package corehttp

import (
	"html/template"
	"path"
)

// structs for directory listing
type listingTemplateData struct {
	Listing  []directoryItem
	Path     string
	BackLink string
}

type directoryItem struct {
	Size uint64
	Name string
	Path string
}

// Directory listing template
var listingTemplate = template.Must(template.New("dir").Funcs(template.FuncMap{"iconFromExt": iconFromExt}).Parse(`
<!DOCTYPE html>
<html>
		<head>
				<meta charset="utf-8" />
				<!-- TODO: seed these - maybe like the starter ex or the webui? -->
				<link rel="stylesheet" href="/ipfs/QmXB7PLRWH6bCiwrGh2MrBBjNkLv3mY3JdYXCikYZSwLED/bootstrap.min.css"/>
				<!-- helper to construct this is here: https://github.com/cryptix/exp/blob/master/imgesToCSSData/convert.go -->
				<link rel="stylesheet" href="/ipfs/QmXB7PLRWH6bCiwrGh2MrBBjNkLv3mY3JdYXCikYZSwLED/icons.css">
				<style>
						.narrow {width: 0px;}
						.padding { margin: 100px;}
						#header {
							background: #000;
						}
						#logo {
							height: 25px;
							margin: 10px;
						}
						.ipfs-icon {
							width:16px;
						}
				</style>
				<title>{{ .Path }}</title>
		</head>
		<body>
				<div id="header" class="row">
						<div class="col-xs-2">
								<div id="logo" class="ipfs-logo">&nbsp;</div>
						</div>
				</div>
				<br/>
				<div class="col-xs-12">
						<div class="panel panel-default">
								<div class="panel-heading">
										<strong>Index of {{ .Path }}</strong>
								</div>
								<table class="table table-striped">
										<tr>
												<td class="narrow">
														<div class="ipfs-icon ipfs-_blank">&nbsp;</div>
												</td>
												<td class="padding">
														<a href="{{.BackLink}}">..</a>
												</td>
												<td></td>
										</tr>
										{{ range .Listing }}
										<tr>
												<td>
														<div class="ipfs-icon {{iconFromExt .Name}}">&nbsp;</div>
												</td>
												<td>
														<a href="{{ .Path }}">{{ .Name }}</a>
												</td>
												<td>{{ .Size }} bytes</td>
										</tr>
										{{ end }}
								</table>
						</div>
				</div>
		</body>
</html>
`))

// helper to guess the type/icon for it by the extension name
func iconFromExt(name string) string {
	ext := path.Ext(name)
	_, ok := knownIcons[ext]
	if !ok {
		// default blank icon
		return "ipfs-_blank"
	}
	return "ipfs-" + ext[1:] // slice of the first dot
}

var knownIcons = map[string]bool{
	".aac":  true,
	".aiff": true,
	".ai":   true,
	".avi":  true,
	".bmp":  true,
	".c":    true,
	".cpp":  true,
	".css":  true,
	".dat":  true,
	".dmg":  true,
	".doc":  true,
	".dotx": true,
	".dwg":  true,
	".dxf":  true,
	".eps":  true,
	".exe":  true,
	".flv":  true,
	".gif":  true,
	".h":    true,
	".hpp":  true,
	".html": true,
	".ics":  true,
	".iso":  true,
	".java": true,
	".jpg":  true,
	".js":   true,
	".key":  true,
	".less": true,
	".mid":  true,
	".mp3":  true,
	".mp4":  true,
	".mpg":  true,
	".odf":  true,
	".ods":  true,
	".odt":  true,
	".otp":  true,
	".ots":  true,
	".ott":  true,
	".pdf":  true,
	".php":  true,
	".png":  true,
	".ppt":  true,
	".psd":  true,
	".py":   true,
	".qt":   true,
	".rar":  true,
	".rb":   true,
	".rtf":  true,
	".sass": true,
	".scss": true,
	".sql":  true,
	".tga":  true,
	".tgz":  true,
	".tiff": true,
	".txt":  true,
	".wav":  true,
	".xls":  true,
	".xlsx": true,
	".xml":  true,
	".yml":  true,
	".zip":  true,
}
