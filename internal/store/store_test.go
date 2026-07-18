package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kenfaulkner/trucktetris/internal/domain"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenUpgradesOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	// Simulate a pre-M5 database: cases table without max_stack_weight.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE cases (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, l INTEGER, w INTEGER, h INTEGER,
		weight INTEGER, type TEXT, stackable_on TEXT DEFAULT '[]', upright_axes TEXT DEFAULT '[]');
		INSERT INTO cases (id, name, l, w, h, weight, type, upright_axes)
		VALUES ('old1', 'Legacy', 100, 100, 100, 10, 'x', '["H"]');`)
	if err != nil {
		t.Fatal(err)
	}
	raw.Close()

	// Open should add the missing column and read the legacy row.
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open old db: %v", err)
	}
	defer s.Close()

	c, err := s.GetCase("old1")
	if err != nil {
		t.Fatalf("get legacy case: %v", err)
	}
	if c.MaxStackWeight != 0 {
		t.Fatalf("legacy case max stack weight = %d, want 0", c.MaxStackWeight)
	}
}

func TestSeedPopulatesSamples(t *testing.T) {
	s := openTest(t)
	cases, err := s.ListCases()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != len(domain.SampleCases()) {
		t.Fatalf("seeded %d cases, want %d", len(cases), len(domain.SampleCases()))
	}
	trucks, err := s.ListTrucks()
	if err != nil {
		t.Fatal(err)
	}
	if len(trucks) != len(domain.SampleTrucks()) {
		t.Fatalf("seeded %d trucks, want %d", len(trucks), len(domain.SampleTrucks()))
	}
}

func TestCaseRoundTrip(t *testing.T) {
	s := openTest(t)
	want := domain.Case{
		ID: "c1", Name: "Test", Type: "rack",
		Dim: domain.Dimensions{L: 100, W: 200, H: 300}, Weight: 50,
		Stackable: true, StackableOn: []string{"rack", "trunk"},
		MaxStackWeight: 70, CanLieOnSide: true,
	}
	if err := s.SaveCase(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetCase("c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != want.Name || got.Dim != want.Dim || got.Weight != want.Weight ||
		got.Stackable != true || len(got.StackableOn) != 2 ||
		got.MaxStackWeight != 70 || got.CanLieOnSide != true {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestSaveCaseUpserts(t *testing.T) {
	s := openTest(t)
	c := domain.Case{ID: "c1", Name: "V1", Dim: domain.Dimensions{L: 1, W: 1, H: 1},
		Weight: 1}
	if err := s.SaveCase(c); err != nil {
		t.Fatal(err)
	}
	c.Name = "V2"
	if err := s.SaveCase(c); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetCase("c1")
	if got.Name != "V2" {
		t.Fatalf("upsert failed, name = %q", got.Name)
	}
}

func TestSaveCaseRejectsInvalid(t *testing.T) {
	s := openTest(t)
	err := s.SaveCase(domain.Case{ID: "bad", Name: "x",
		Dim: domain.Dimensions{L: -1, W: 1, H: 1}, Weight: 1})
	if err == nil {
		t.Fatal("expected validation error for negative dimension")
	}
}

func TestDeleteCase(t *testing.T) {
	s := openTest(t)
	c := domain.Case{ID: "c1", Name: "x", Dim: domain.Dimensions{L: 1, W: 1, H: 1},
		Weight: 1}
	s.SaveCase(c)
	if err := s.DeleteCase("c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteCase("c1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetCase("c1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
}

func TestTruckRoundTrip(t *testing.T) {
	s := openTest(t)
	want := domain.Truck{
		ID: "t1", Name: "Rig", Dim: domain.Dimensions{L: 5000, W: 2000, H: 2000},
		GrossMax: 8000,
		Axles:    []domain.Axle{{Position: 1000, MaxLoad: 4000}, {Position: 4000, MaxLoad: 5000}},
	}
	if err := s.SaveTruck(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetTruck("t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != want.Name || got.Dim != want.Dim || got.GrossMax != want.GrossMax ||
		len(got.Axles) != 2 || got.Axles[1].MaxLoad != 5000 {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestPlanRoundTripAndList(t *testing.T) {
	s := openTest(t)
	p := domain.SavedPlan{
		ID: "p1", Name: "Friday show", TruckID: "truck-sample",
		Placements: []domain.Placement{
			{CaseID: "c1", Pos: [3]int{0, 0, 0}, Size: [3]int{100, 100, 100}, Up: domain.AxisH},
		},
		Unplaced: []string{"c9"},
	}
	if err := s.SavePlan(p); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetPlan("p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Friday show" || got.TruckID != "truck-sample" ||
		len(got.Placements) != 1 || got.Placements[0].CaseID != "c1" ||
		len(got.Unplaced) != 1 || got.CreatedAt == "" {
		t.Fatalf("round trip mismatch: %+v", got)
	}

	list, err := s.ListPlans()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "p1" {
		t.Fatalf("list = %+v, want one plan p1", list)
	}
}

func TestSavePlanRejectsMissingFields(t *testing.T) {
	s := openTest(t)
	if err := s.SavePlan(domain.SavedPlan{ID: "p1", TruckID: "t"}); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestDeletePlan(t *testing.T) {
	s := openTest(t)
	s.SavePlan(domain.SavedPlan{ID: "p1", Name: "x", TruckID: "t"})
	if err := s.DeletePlan("p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeletePlan("p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
}

func TestSaveTruckRejectsAxleOutsideLength(t *testing.T) {
	s := openTest(t)
	err := s.SaveTruck(domain.Truck{ID: "t1", Name: "x",
		Dim: domain.Dimensions{L: 1000, W: 1000, H: 1000}, GrossMax: 5000,
		Axles: []domain.Axle{{Position: 2000, MaxLoad: 1000}}})
	if err == nil {
		t.Fatal("expected validation error for axle beyond truck length")
	}
}
