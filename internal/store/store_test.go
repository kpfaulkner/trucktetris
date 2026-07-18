package store

import (
	"errors"
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
	if len(trucks) != 1 {
		t.Fatalf("seeded %d trucks, want 1", len(trucks))
	}
}

func TestCaseRoundTrip(t *testing.T) {
	s := openTest(t)
	want := domain.Case{
		ID: "c1", Name: "Test", Type: "rack",
		Dim: domain.Dimensions{L: 100, W: 200, H: 300}, Weight: 50,
		StackableOn: []string{"rack", "trunk"},
		UprightAxes: []domain.Axis{domain.AxisH, domain.AxisW},
	}
	if err := s.SaveCase(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetCase("c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != want.Name || got.Dim != want.Dim || got.Weight != want.Weight ||
		len(got.StackableOn) != 2 || len(got.UprightAxes) != 2 {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestSaveCaseUpserts(t *testing.T) {
	s := openTest(t)
	c := domain.Case{ID: "c1", Name: "V1", Dim: domain.Dimensions{L: 1, W: 1, H: 1},
		Weight: 1, UprightAxes: []domain.Axis{domain.AxisH}}
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
		Dim: domain.Dimensions{L: -1, W: 1, H: 1}, Weight: 1,
		UprightAxes: []domain.Axis{domain.AxisH}})
	if err == nil {
		t.Fatal("expected validation error for negative dimension")
	}
}

func TestDeleteCase(t *testing.T) {
	s := openTest(t)
	c := domain.Case{ID: "c1", Name: "x", Dim: domain.Dimensions{L: 1, W: 1, H: 1},
		Weight: 1, UprightAxes: []domain.Axis{domain.AxisH}}
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

func TestSaveTruckRejectsAxleOutsideLength(t *testing.T) {
	s := openTest(t)
	err := s.SaveTruck(domain.Truck{ID: "t1", Name: "x",
		Dim: domain.Dimensions{L: 1000, W: 1000, H: 1000}, GrossMax: 5000,
		Axles: []domain.Axle{{Position: 2000, MaxLoad: 1000}}})
	if err == nil {
		t.Fatal("expected validation error for axle beyond truck length")
	}
}
