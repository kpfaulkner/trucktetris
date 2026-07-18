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
