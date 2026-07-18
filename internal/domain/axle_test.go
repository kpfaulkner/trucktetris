package domain

import "testing"

func twoAxle() Truck {
	return Truck{
		Dim: Dimensions{L: 6000, W: 2400, H: 2400}, GrossMax: 20000,
		Axles: []Axle{{Position: 1000, MaxLoad: 8000}, {Position: 5000, MaxLoad: 12000}},
	}
}

func TestComputeAxleLoadsConservesTotal(t *testing.T) {
	tk := twoAxle()
	loads := []PointLoad{{X: 2000, Weight: 300}, {X: 4000, Weight: 500}, {X: 500, Weight: 200}}
	got := ComputeAxleLoads(tk, loads)

	sum := 0
	for _, a := range got {
		sum += a.Load
	}
	if sum != 1000 {
		t.Fatalf("axle loads sum = %d, want 1000 (total weight)", sum)
	}
}

func TestComputeAxleLoadsLeverRule(t *testing.T) {
	tk := twoAxle() // axles at 1000 and 5000
	// A load exactly at the midpoint (3000) splits evenly.
	got := ComputeAxleLoads(tk, []PointLoad{{X: 3000, Weight: 1000}})
	if got[0].Load != 500 || got[1].Load != 500 {
		t.Fatalf("midpoint load = %d/%d, want 500/500", got[0].Load, got[1].Load)
	}
	// A load right over the front axle goes entirely to it.
	got = ComputeAxleLoads(tk, []PointLoad{{X: 1000, Weight: 1000}})
	if got[0].Load != 1000 || got[1].Load != 0 {
		t.Fatalf("over-front load = %d/%d, want 1000/0", got[0].Load, got[1].Load)
	}
}

func TestComputeAxleLoadsOverhangClamps(t *testing.T) {
	tk := twoAxle()
	// Ahead of the front axle: all to front, no negative on rear.
	got := ComputeAxleLoads(tk, []PointLoad{{X: 0, Weight: 800}})
	if got[0].Load != 800 || got[1].Load != 0 {
		t.Fatalf("overhang = %d/%d, want 800/0", got[0].Load, got[1].Load)
	}
}

func TestComputeAxleLoadsFlagsOver(t *testing.T) {
	tk := twoAxle() // rear max 12000
	got := ComputeAxleLoads(tk, []PointLoad{{X: 5000, Weight: 15000}})
	if !got[1].Over {
		t.Fatal("rear axle should be flagged over")
	}
	if got[0].Over {
		t.Fatal("front axle should not be over")
	}
}

func TestComputeAxleLoadsSingleAxle(t *testing.T) {
	tk := Truck{Dim: Dimensions{L: 3000, W: 2000, H: 2000}, GrossMax: 5000,
		Axles: []Axle{{Position: 1500, MaxLoad: 5000}}}
	got := ComputeAxleLoads(tk, []PointLoad{{X: 100, Weight: 400}, {X: 2900, Weight: 600}})
	if got[0].Load != 1000 {
		t.Fatalf("single axle load = %d, want 1000", got[0].Load)
	}
}
