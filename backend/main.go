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

	"github.com/chrishecht/kanban/backend/internal/api"
	"github.com/chrishecht/kanban/backend/internal/config"
	"github.com/chrishecht/kanban/backend/internal/store"
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
		if _, err := fs.Stat(web, name); err != nil {
			// Unknown path → SPA fallback.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
