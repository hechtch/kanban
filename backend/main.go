package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

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

	log.Printf("kanban listening on %s · db=%s", *addr, cfg.DBPath)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
