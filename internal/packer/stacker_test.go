package packer

import (
	"testing"

	"github.com/kenfaulkner/trucktetris/internal/domain"
)

// validateStack asserts placements are in-bounds, non-overlapping, gross weight
// is respected, and every stacked case rests legally on a supporting case.
func validateStack(t *testing.T, req domain.SolveRequest, plan domain.LoadPlan) {
	t.Helper()
	tk := req.Truck
	byID := map[string]domain.Case{}
	for _, c := range req.Cases {
		byID[c.ID] = c
	}

	boxes := make([]box, len(plan.Placements))
	for i, p := range plan.Placements {
		boxes[i] = resolve(p)
		b := boxes[i]
		if b.max[0] > tk.Dim.L || b.max[1] > tk.Dim.W || b.max[2] > tk.Dim.H {
			t.Errorf("case %s outside truck: max=%v", p.CaseID, b.max)
		}
		for j := 0; j < i; j++ {
			if overlaps(b, boxes[j]) {
				t.Errorf("case %s overlaps %s", p.CaseID, plan.Placements[j].CaseID)
			}
		}
	}

	if plan.Summary.TotalWeight > tk.GrossMax {
		t.Errorf("total weight %d exceeds gross max %d", plan.Summary.TotalWeight, tk.GrossMax)
	}

	for i, p := range plan.Placements {
		if boxes[i].min[2] == 0 {
			continue // on the floor
		}
		c := byID[p.CaseID]
		supported := false
		for j, q := range plan.Placements {
			if i == j {
				continue
			}
			s := boxes[j]
			topAtBase := s.max[2] == boxes[i].min[2]
			covers := s.min[0] <= boxes[i].min[0] && s.max[0] >= boxes[i].max[0] &&
				s.min[1] <= boxes[i].min[1] && s.max[1] >= boxes[i].max[1]
			if topAtBase && covers {
				if !stackableOn(c, byID[q.CaseID].Type) {
					t.Errorf("case %s stacked on disallowed type %s", c.ID, byID[q.CaseID].Type)
				}
				supported = true
			}
		}
		if !supported {
			t.Errorf("case %s floats: no support at z=%d", c.ID, boxes[i].min[2])
		}
	}
}

func TestStackerPacksSampleValid(t *testing.T) {
	req := domain.SolveRequest{Truck: domain.SampleTruck(), Cases: domain.SampleCases()}
	plan := Stacker{}.Pack(req)
	if plan.Summary.PlacedCount == 0 {
		t.Fatal("expected some cases placed")
	}
	validateStack(t, req, plan)
}

func TestStackerRespectsGrossMax(t *testing.T) {
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "t", Dim: domain.Dimensions{L: 10000, W: 3000, H: 3000}, GrossMax: 100},
		Cases: []domain.Case{
			{ID: "a", Type: "x", Dim: domain.Dimensions{L: 100, W: 100, H: 100}, Weight: 60},
			{ID: "b", Type: "x", Dim: domain.Dimensions{L: 100, W: 100, H: 100}, Weight: 60},
		},
	}
	plan := Stacker{}.Pack(req)
	if plan.Summary.TotalWeight > 100 {
		t.Fatalf("gross max exceeded: %d", plan.Summary.TotalWeight)
	}
	if plan.Summary.PlacedCount != 1 {
		t.Fatalf("expected exactly 1 placed under gross limit, got %d", plan.Summary.PlacedCount)
	}
}

func TestStackerRespectsBearingLimit(t *testing.T) {
	// Bottom bears only 10kg; a 50kg case must not stack on it. Footprint fills
	// the floor so the only place for the second case is on top.
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "t", Dim: domain.Dimensions{L: 100, W: 100, H: 1000}, GrossMax: 10000},
		Cases: []domain.Case{
			{ID: "bottom", Type: "base", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 80, Stackable: true, MaxStackWeight: 10},
			{ID: "top", Type: "load", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 50, StackableOn: []string{"base"}},
		},
	}
	plan := Stacker{}.Pack(req)
	if plan.Summary.PlacedCount != 1 {
		t.Fatalf("heavy case should not fit on weak base, placed=%d", plan.Summary.PlacedCount)
	}
	validateStack(t, req, plan)
}

func TestStackerStacksWhenAllowed(t *testing.T) {
	// Floor only fits one footprint; the second case must stack, and is allowed.
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "t", Dim: domain.Dimensions{L: 100, W: 100, H: 1000}, GrossMax: 10000},
		Cases: []domain.Case{
			{ID: "bottom", Type: "base", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 80, Stackable: true, MaxStackWeight: 100},
			{ID: "top", Type: "load", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 50, StackableOn: []string{"base"}},
		},
	}
	plan := Stacker{}.Pack(req)
	if plan.Summary.PlacedCount != 2 {
		t.Fatalf("both cases should fit by stacking, placed=%d", plan.Summary.PlacedCount)
	}
	validateStack(t, req, plan)
}

func TestStackerRejectsDisallowedStack(t *testing.T) {
	// Second case has empty StackableOn (floor only) and cannot go on top.
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "t", Dim: domain.Dimensions{L: 100, W: 100, H: 1000}, GrossMax: 10000},
		Cases: []domain.Case{
			{ID: "bottom", Type: "base", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 80, Stackable: true, MaxStackWeight: 100},
			{ID: "floorer", Type: "load", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 50, StackableOn: nil},
		},
	}
	plan := Stacker{}.Pack(req)
	if plan.Summary.PlacedCount != 1 {
		t.Fatalf("floor-only case must not stack, placed=%d", plan.Summary.PlacedCount)
	}
}

func TestStackerReportsAxleLoads(t *testing.T) {
	req := domain.SolveRequest{Truck: domain.SampleTruck(), Cases: domain.SampleCases()}
	plan := Stacker{}.Pack(req)

	if len(plan.AxleLoads) != len(req.Truck.Axles) {
		t.Fatalf("axle loads = %d, want %d", len(plan.AxleLoads), len(req.Truck.Axles))
	}
	sum := 0
	for _, a := range plan.AxleLoads {
		sum += a.Load
		if a.Over {
			t.Errorf("sample should not overload axle @ %d (%d/%d)", a.Position, a.Load, a.MaxLoad)
		}
	}
	if sum != plan.Summary.TotalWeight {
		t.Fatalf("axle loads sum %d != total weight %d", sum, plan.Summary.TotalWeight)
	}
}

func TestStackerRejectsAxleOverload(t *testing.T) {
	// One axle with a tiny limit; a heavy case cannot be placed anywhere without
	// overloading it, so it stays unplaced and no axle ends up over limit.
	req := domain.SolveRequest{
		Truck: domain.Truck{
			ID: "t", Dim: domain.Dimensions{L: 4000, W: 2000, H: 2000}, GrossMax: 100000,
			Axles: []domain.Axle{{Position: 1000, MaxLoad: 100}, {Position: 3000, MaxLoad: 100}},
		},
		Cases: []domain.Case{
			{ID: "heavy", Type: "x", Dim: domain.Dimensions{L: 500, W: 500, H: 500}, Weight: 5000},
		},
	}
	plan := Stacker{}.Pack(req)
	if plan.Summary.PlacedCount != 0 {
		t.Fatalf("heavy case should be rejected on axle overload, placed=%d", plan.Summary.PlacedCount)
	}
	for _, a := range plan.AxleLoads {
		if a.Over {
			t.Errorf("no axle should be over after rejection: @ %d %d/%d", a.Position, a.Load, a.MaxLoad)
		}
	}
}

func TestFindSpotBiasesOnlyHeavyCases(t *testing.T) {
	// Two floor candidates: x=0 (front, away from axle) and x=1500 (near the
	// rear axle at 2000). Heavy cases should prefer the axle; light cases should
	// pack front-most for density.
	tk := domain.Truck{
		Dim: domain.Dimensions{L: 2000, W: 500, H: 500}, GrossMax: 1_000_000,
		Axles:          []domain.Axle{{Position: 2000, MaxLoad: 1_000_000}},
		HeavyThreshold: 100,
	}
	pts := []point{{0, 0, 0}, {1500, 0, 0}}
	cube := domain.Dimensions{L: 500, W: 500, H: 500}

	light := domain.Case{ID: "l", Dim: cube, Weight: 10}
	pos, _, _, ok := findSpot(light, tk, nil, map[int]int{}, pts, nil)
	if !ok || pos.x != 0 {
		t.Fatalf("light case should pack front-most (x=0), got x=%d ok=%v", pos.x, ok)
	}

	heavy := domain.Case{ID: "h", Dim: cube, Weight: 150}
	pos, _, _, ok = findSpot(heavy, tk, nil, map[int]int{}, pts, nil)
	if !ok || pos.x != 1500 {
		t.Fatalf("heavy case should bias to axle (x=1500), got x=%d ok=%v", pos.x, ok)
	}
}

func TestStackerRejectsWhenBottomNotStackable(t *testing.T) {
	// Type + weight would allow it, but the bottom case is flagged not
	// stackable, so nothing may rest on it.
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "t", Dim: domain.Dimensions{L: 100, W: 100, H: 1000}, GrossMax: 10000},
		Cases: []domain.Case{
			{ID: "bottom", Type: "base", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 80, Stackable: false, MaxStackWeight: 100},
			{ID: "top", Type: "load", Dim: domain.Dimensions{L: 100, W: 100, H: 100},
				Weight: 50, StackableOn: []string{"base"}},
		},
	}
	plan := Stacker{}.Pack(req)
	if plan.Summary.PlacedCount != 1 {
		t.Fatalf("nothing may stack on a non-stackable case, placed=%d", plan.Summary.PlacedCount)
	}
}
