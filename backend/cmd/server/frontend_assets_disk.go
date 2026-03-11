//go:build !embed_frontend

package main

import "io/fs"

func loadEmbeddedFrontend() (fs.FS, bool) {
	return nil, false
}
