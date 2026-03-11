//go:build embed_frontend

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:webdist
var embeddedFrontendDist embed.FS

func loadEmbeddedFrontend() (fs.FS, bool) {
	sub, err := fs.Sub(embeddedFrontendDist, "webdist")
	if err != nil {
		return nil, false
	}
	return sub, true
}
