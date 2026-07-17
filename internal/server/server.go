// Package server wires up the Truck Tetris HTTP API and static file serving.
package server

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/kenfaulkner/trucktetris/internal/domain"
)

// New builds the HTTP handler. staticFS serves the frontend assets.
func New(staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/sample", handleSample)
	mux.HandleFunc("POST /api/solve", handleSolve)

	mux.Handle("GET /", http.FileServerFS(staticFS))

	return logRequests(mux)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleSample returns the hardcoded sample truck + cases (M1; replaced by
// stored data in M4).
func handleSample(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, domain.SolveRequest{
		Truck: domain.SampleTruck(),
		Cases: domain.SampleCases(),
	})
}

// handleSolve is a stub for M1: it echoes the request back as a trivial
// LoadPlan with everything unplaced. The real packer arrives in M2.
func handleSolve(w http.ResponseWriter, r *http.Request) {
	var req domain.SolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	unplaced := make([]string, len(req.Cases))
	for i, c := range req.Cases {
		unplaced[i] = c.ID
	}

	plan := domain.LoadPlan{
		Truck:      req.Truck,
		Placements: []domain.Placement{},
		Unplaced:   unplaced,
		Summary: domain.Summary{
			UnplacedCount: len(unplaced),
		},
	}
	writeJSON(w, http.StatusOK, plan)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
