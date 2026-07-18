package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/kenfaulkner/trucktetris/internal/domain"
	"github.com/kenfaulkner/trucktetris/internal/server"
	"github.com/kenfaulkner/trucktetris/internal/store"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return server.New(fstest.MapFS{}, st)
}

// do issues a request and returns the recorder. body may be nil, a string, or
// any JSON-marshalable value.
func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
		r = nil
	case string:
		r = bytes.NewBufferString(b)
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, r)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestHealth(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "GET", "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := decodeBody[map[string]string](t, rec)
	if got["status"] != "ok" {
		t.Fatalf("status body = %v", got)
	}
}

func TestListCasesSeeded(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "GET", "/api/cases", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	got := decodeBody[[]domain.Case](t, rec)
	if len(got) != len(domain.SampleCases()) {
		t.Fatalf("got %d cases, want %d", len(got), len(domain.SampleCases()))
	}
}

func TestCreateCaseAssignsIDAndPersists(t *testing.T) {
	h := newTestServer(t)
	body := domain.Case{
		Name: "New rack", Type: "rack", Dim: domain.Dimensions{L: 500, W: 500, H: 900},
		Weight: 40,
	}
	rec := do(t, h, "POST", "/api/cases", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	created := decodeBody[domain.Case](t, rec)
	if created.ID == "" {
		t.Fatal("expected an assigned ID")
	}
	// It should now be retrievable.
	rec = do(t, h, "GET", "/api/cases/"+created.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get created: status = %d", rec.Code)
	}
}

func TestCreateCaseRejectsInvalid(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "POST", "/api/cases", domain.Case{
		Name: "bad", Dim: domain.Dimensions{L: -1, W: 1, H: 1}, Weight: 1,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateCaseRejectsBadJSON(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "POST", "/api/cases", "{not json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGetCaseNotFound(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "GET", "/api/cases/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpdateCase(t *testing.T) {
	h := newTestServer(t)
	body := domain.Case{
		Name: "Amp rack XL", Type: "rack", Dim: domain.Dimensions{L: 800, W: 700, H: 1200},
		Weight: 77, Stackable: true, MaxStackWeight: 120,
	}
	rec := do(t, h, "PUT", "/api/cases/case-amp", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	got := decodeBody[domain.Case](t, rec)
	if got.ID != "case-amp" || got.Weight != 77 {
		t.Fatalf("update mismatch: %+v", got)
	}
}

func TestDeleteCase(t *testing.T) {
	h := newTestServer(t)
	if rec := do(t, h, "DELETE", "/api/cases/case-amp", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/api/cases/case-amp", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404", rec.Code)
	}
}

func TestSolveSelection(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "POST", "/api/solve", map[string]any{
		"truckId": "truck-sample",
		"caseIds": []string{"case-amp", "case-speaker"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	plan := decodeBody[domain.LoadPlan](t, rec)
	if plan.Summary.PlacedCount == 0 {
		t.Fatal("expected some cases placed")
	}
	if len(plan.AxleLoads) != len(plan.Truck.Axles) {
		t.Fatalf("axle loads = %d, want %d", len(plan.AxleLoads), len(plan.Truck.Axles))
	}
}

func TestSolveUnknownTruck(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "POST", "/api/solve", map[string]any{"truckId": "ghost", "caseIds": []string{}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestEvaluate(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "POST", "/api/evaluate", map[string]any{
		"truckId": "truck-sample",
		"placements": []domain.Placement{
			{InstanceID: "case-amp#0", CaseID: "case-amp", Pos: [3]int{0, 0, 0}, Size: [3]int{800, 700, 1200}},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	ev := decodeBody[domain.Evaluation](t, rec)
	if ev.TotalWeight != 90 {
		t.Fatalf("total weight = %d, want 90", ev.TotalWeight)
	}
}

func TestEvaluateUnknownTruck(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "POST", "/api/evaluate", map[string]any{
		"truckId":    "ghost",
		"placements": []domain.Placement{},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestEvaluateUnknownCase(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "POST", "/api/evaluate", map[string]any{
		"truckId": "truck-sample",
		"placements": []domain.Placement{
			{InstanceID: "ghost#0", CaseID: "ghost", Pos: [3]int{0, 0, 0}, Size: [3]int{100, 100, 100}},
		},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestSolveGrossOverflowLeavesUnplaced(t *testing.T) {
	h := newTestServer(t)
	// A heavy case: three of them (15000 kg) exceed the sample truck's 12000 kg
	// gross, so at least one must be reported unplaced.
	rec := do(t, h, "POST", "/api/cases", domain.Case{
		Name: "Heavy", Type: "heavy", Dim: domain.Dimensions{L: 500, W: 500, H: 500}, Weight: 5000,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create heavy: status = %d; body=%s", rec.Code, rec.Body)
	}
	id := decodeBody[domain.Case](t, rec).ID

	rec = do(t, h, "POST", "/api/solve", map[string]any{
		"truckId": "truck-sample",
		"caseIds": []string{id, id, id},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("solve status = %d; body=%s", rec.Code, rec.Body)
	}
	plan := decodeBody[domain.LoadPlan](t, rec)
	if plan.Summary.TotalWeight > 12000 {
		t.Fatalf("total weight %d exceeds gross 12000", plan.Summary.TotalWeight)
	}
	if plan.Summary.UnplacedCount == 0 {
		t.Fatalf("expected at least one unplaced over gross, got %+v", plan.Summary)
	}
}

func TestPlanLifecycle(t *testing.T) {
	h := newTestServer(t)

	// Create.
	rec := do(t, h, "POST", "/api/plans", map[string]any{
		"name":    "Friday show",
		"truckId": "truck-sample",
		"placements": []domain.Placement{
			{InstanceID: "case-amp#0", CaseID: "case-amp", Pos: [3]int{0, 0, 0}, Size: [3]int{800, 700, 1200}},
		},
		"unplaced": []string{},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	created := decodeBody[domain.SavedPlan](t, rec)
	if created.ID == "" {
		t.Fatal("expected plan ID")
	}

	// List.
	rec = do(t, h, "GET", "/api/plans", nil)
	if list := decodeBody[[]domain.SavedPlan](t, rec); len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	// Get with placements.
	rec = do(t, h, "GET", "/api/plans/"+created.ID, nil)
	if got := decodeBody[domain.SavedPlan](t, rec); len(got.Placements) != 1 {
		t.Fatalf("get placements = %d, want 1", len(got.Placements))
	}

	// Delete.
	if rec = do(t, h, "DELETE", "/api/plans/"+created.ID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	if rec = do(t, h, "GET", "/api/plans/"+created.ID, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", rec.Code)
	}
}

func TestCreateTruckAndValidation(t *testing.T) {
	h := newTestServer(t)
	// Valid.
	rec := do(t, h, "POST", "/api/trucks", domain.Truck{
		Name: "Big", Dim: domain.Dimensions{L: 13000, W: 2480, H: 2700}, GrossMax: 24000,
		Axles: []domain.Axle{{Position: 2000, MaxLoad: 8000}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	// Invalid: axle beyond truck length.
	rec = do(t, h, "POST", "/api/trucks", domain.Truck{
		Name: "Bad", Dim: domain.Dimensions{L: 1000, W: 1000, H: 1000}, GrossMax: 5000,
		Axles: []domain.Axle{{Position: 9999, MaxLoad: 100}},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
