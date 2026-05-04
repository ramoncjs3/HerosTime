package admin

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist
var embeddedWeb embed.FS

func webFileSystem() (http.FileSystem, error) {
	dist, err := fs.Sub(embeddedWeb, "web/dist")
	if err != nil {
		return nil, err
	}
	return http.FS(dist), nil
}
