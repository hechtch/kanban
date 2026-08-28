package main

import (
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hechtch/kanban/backend/internal/api"
	"github.com/hechtch/kanban/backend/internal/config"
	"github.com/hechtch/kanban/backend/internal/store"
)

func main() {
	addr := flag.String("addr", ":8000", "listen address")
	flag.Parse()

	cfg := config.Load()
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	mux := http.NewServeMux()
	api.Register(mux, st)

	// Serve the embedded SPA on non-API routes when it's available.
	if web, ok := staticFS(); ok {
		mux.Handle("/", spaHandler(web))
		log.Printf("kanban serving UI + API on %s · db=%s", *addr, cfg.DBPath)
	} else {
		log.Printf("kanban serving API only on %s · db=%s (build with -tags embed for UI)", *addr, cfg.DBPath)
	}

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

// spaHandler serves files from `web`, falling back to index.html for any
// non-asset path so client-side routing works.
func spaHandler(web fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(r.URL.Path)
		if clean == "/" {
			clean = "/index.html"
		}
		// Strip leading slash for fs.FS lookups.
		name := strings.TrimPrefix(clean, "/")
		if _, err := fs.Stat(web, name); err != nil || name == "index.html" {
			// index.html (directly or as the SPA fallback for an unknown
			// path) must never be cached: it names the hashed chunks, and a
			// stale copy keeps serving an old bundle after a redeploy.
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		// Everything else in the bundle is content-hashed → immutable.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, r)
	})
}
