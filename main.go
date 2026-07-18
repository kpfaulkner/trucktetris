// Command trucktetris serves the Truck Tetris web app and packing API.
package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/kenfaulkner/trucktetris/internal/server"
	"github.com/kenfaulkner/trucktetris/internal/store"
)

//go:embed static
var staticFiles embed.FS

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8081"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "trucktetris.db"
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("mount static", "err", err)
		os.Exit(1)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	handler := server.New(staticFS, st)

	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
