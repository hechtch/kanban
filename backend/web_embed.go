//go:build embed

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var webFS embed.FS

func staticFS() (fs.FS, bool) {
	sub, err := fs.Sub(webFS, "web/frontend/browser")
	if err != nil {
		return nil, false
	}
	return sub, true
}
