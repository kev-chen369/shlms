package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html styles.css app.js share.mjs promoter.mjs
var files embed.FS

func Handler() http.Handler {
	root, _ := fs.Sub(files, ".")
	return http.FileServer(http.FS(root))
}
