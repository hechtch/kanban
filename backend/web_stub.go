//go:build !embed

package main

import "io/fs"

// staticFS returns the embedded SPA bundle, or (nil, false) when the binary
// was built without the `embed` tag (e.g. during `go run .` in dev — the
// frontend runs on its own dev server in that mode).
func staticFS() (fs.FS, bool) { return nil, false }
