// Package server wires up the Truck Tetris HTTP API and static file serving.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/kenfaulkner/trucktetris/internal/domain"
	"github.com/kenfaulkner/trucktetris/internal/packer"
	"github.com/kenfaulkner/trucktetris/internal/store"
)

type api struct {
	store  *store.Store
	packer packer.Packer
}

// New builds the HTTP handler. staticFS serves the frontend assets; st is the
// persistence layer.
func New(staticFS fs.FS, st *store.Store) http.Handler {
	a := &api{store: st, packer: packer.Stacker{}}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handleHealth)

	mux.HandleFunc("GET /api/cases", a.listCases)
	mux.HandleFunc("POST /api/cases", a.createCase)
	mux.HandleFunc("GET /api/cases/{id}", a.getCase)
	mux.HandleFunc("PUT /api/cases/{id}", a.updateCase)
	mux.HandleFunc("DELETE /api/cases/{id}", a.deleteCase)

	mux.HandleFunc("GET /api/trucks", a.listTrucks)
	mux.HandleFunc("POST /api/trucks", a.createTruck)
	mux.HandleFunc("GET /api/trucks/{id}", a.getTruck)
	mux.HandleFunc("PUT /api/trucks/{id}", a.updateTruck)
	mux.HandleFunc("DELETE /api/trucks/{id}", a.deleteTruck)

	mux.HandleFunc("POST /api/solve", a.solve)

	mux.Handle("GET /", http.FileServerFS(staticFS))

	return logRequests(mux)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- cases -------------------------------------------------------------------

func (a *api) listCases(w http.ResponseWriter, _ *http.Request) {
	cases, err := a.store.ListCases()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cases)
}

func (a *api) getCase(w http.ResponseWriter, r *http.Request) {
	c, err := a.store.GetCase(r.PathValue("id"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *api) createCase(w http.ResponseWriter, r *http.Request) {
	var c domain.Case
	if !decode(w, r, &c) {
		return
	}
	if c.ID == "" {
		c.ID = newID("case")
	}
	if err := a.store.SaveCase(c); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (a *api) updateCase(w http.ResponseWriter, r *http.Request) {
	var c domain.Case
	if !decode(w, r, &c) {
		return
	}
	c.ID = r.PathValue("id")
	if err := a.store.SaveCase(c); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *api) deleteCase(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteCase(r.PathValue("id")); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- trucks ------------------------------------------------------------------

func (a *api) listTrucks(w http.ResponseWriter, _ *http.Request) {
	trucks, err := a.store.ListTrucks()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, trucks)
}

func (a *api) getTruck(w http.ResponseWriter, r *http.Request) {
	t, err := a.store.GetTruck(r.PathValue("id"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *api) createTruck(w http.ResponseWriter, r *http.Request) {
	var t domain.Truck
	if !decode(w, r, &t) {
		return
	}
	if t.ID == "" {
		t.ID = newID("truck")
	}
	if err := a.store.SaveTruck(t); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (a *api) updateTruck(w http.ResponseWriter, r *http.Request) {
	var t domain.Truck
	if !decode(w, r, &t) {
		return
	}
	t.ID = r.PathValue("id")
	if err := a.store.SaveTruck(t); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (a *api) deleteTruck(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DeleteTruck(r.PathValue("id")); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- solve -------------------------------------------------------------------

// selection picks a stored truck plus a subset of stored cases to pack.
type selection struct {
	TruckID string   `json:"truckId"`
	CaseIDs []string `json:"caseIds"`
}

func (a *api) solve(w http.ResponseWriter, r *http.Request) {
	var sel selection
	if !decode(w, r, &sel) {
		return
	}
	truck, err := a.store.GetTruck(sel.TruckID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	cases := make([]domain.Case, 0, len(sel.CaseIDs))
	for _, id := range sel.CaseIDs {
		c, err := a.store.GetCase(id)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		cases = append(cases, c)
	}
	plan := a.packer.Pack(domain.SolveRequest{Truck: truck, Cases: cases})
	writeJSON(w, http.StatusOK, plan)
}

// --- helpers -----------------------------------------------------------------

// decode reads a JSON body into v, writing a 400 and returning false on error.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func newID(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}

func writeStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeErr(w, http.StatusInternalServerError, err)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
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
