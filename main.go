// Command trucktetris serves the Truck Tetris web app and packing API.
package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/kenfaulkner/trucktetris/internal/server"
)

//go:embed static
var staticFiles embed.FS

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		slog.Error("mount static", "err", err)
		os.Exit(1)
	}

	handler := server.New(staticFS)

	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
