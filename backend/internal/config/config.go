package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	DBPath string
}

func Load() Config {
	dbPath := os.Getenv("KANBAN_DB_PATH")
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dbPath = filepath.Join(home, ".kanban", "data", "kanban.db")
	}
	return Config{DBPath: dbPath}
}
