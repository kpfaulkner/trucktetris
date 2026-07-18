package packer

import (
	"testing"

	"github.com/kenfaulkner/trucktetris/internal/domain"
)

// box is a placed case resolved to its world-space min/max extents.
type box struct {
	min, max [3]int
}

// resolve turns a placement into world extents from its stored size.
func resolve(p domain.Placement) box {
	return box{
		min: p.Pos,
		max: [3]int{p.Pos[0] + p.Size[0], p.Pos[1] + p.Size[1], p.Pos[2] + p.Size[2]},
	}
}

func overlaps(a, b box) bool {
	for i := range 3 {
		if a.max[i] <= b.min[i] || b.max[i] <= a.min[i] {
			return false // separated on this axis
		}
	}
	return true
}

func caseByID(cases []domain.Case, id string) domain.Case {
	for _, c := range cases {
		if c.ID == id {
			return c
		}
	}
	panic("unknown case " + id)
}

// validatePlan asserts every placement is inside the truck, non-overlapping,
// and respects the case's allowed orientations.
func validatePlan(t *testing.T, req domain.SolveRequest, plan domain.LoadPlan) {
	t.Helper()
	boxes := make([]box, 0, len(plan.Placements))
	tk := req.Truck

	for _, p := range plan.Placements {
		c := caseByID(req.Cases, p.CaseID)

		allowed := false
		for _, up := range c.UprightAxes {
			if up == p.Up {
				allowed = true
			}
		}
		if !allowed {
			t.Errorf("case %s placed with disallowed up-axis %s", c.ID, p.Up)
		}

		b := resolve(p)
		for i := range 3 {
			if b.min[i] < 0 {
				t.Errorf("case %s extends below origin on axis %d", c.ID, i)
			}
		}
		if b.max[0] > tk.Dim.L || b.max[1] > tk.Dim.W || b.max[2] > tk.Dim.H {
			t.Errorf("case %s extends outside truck: max=%v truck=%v", c.ID, b.max, tk.Dim)
		}
		for _, other := range boxes {
			if overlaps(b, other) {
				t.Errorf("case %s overlaps another placement", c.ID)
			}
		}
		boxes = append(boxes, b)
	}
}

func TestShelfPacksSample(t *testing.T) {
	req := domain.SolveRequest{Truck: domain.SampleTruck(), Cases: domain.SampleCases()}
	plan := Shelf{}.Pack(req)

	if plan.Summary.PlacedCount == 0 {
		t.Fatal("expected some cases placed")
	}
	if got := plan.Summary.PlacedCount + plan.Summary.UnplacedCount; got != len(req.Cases) {
		t.Fatalf("placed+unplaced = %d, want %d", got, len(req.Cases))
	}
	validatePlan(t, req, plan)
}

func TestShelfWeightSumsPlacedOnly(t *testing.T) {
	req := domain.SolveRequest{Truck: domain.SampleTruck(), Cases: domain.SampleCases()}
	plan := Shelf{}.Pack(req)

	want := 0
	placed := map[string]bool{}
	for _, p := range plan.Placements {
		placed[p.CaseID] = true
	}
	for _, c := range req.Cases {
		if placed[c.ID] {
			want += c.Weight
		}
	}
	if plan.Summary.TotalWeight != want {
		t.Fatalf("total weight = %d, want %d", plan.Summary.TotalWeight, want)
	}
}

func TestShelfReportsOversizedCaseUnplaced(t *testing.T) {
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "tiny", Dim: domain.Dimensions{L: 500, W: 500, H: 500}},
		Cases: []domain.Case{{
			ID: "big", Dim: domain.Dimensions{L: 1000, W: 1000, H: 1000},
			UprightAxes: []domain.Axis{domain.AxisH},
		}},
	}
	plan := Shelf{}.Pack(req)
	if plan.Summary.PlacedCount != 0 || len(plan.Unplaced) != 1 {
		t.Fatalf("oversized case should be unplaced, got %+v", plan.Summary)
	}
}

func TestUprightOnlyCaseNeverOnSide(t *testing.T) {
	c := domain.Case{
		Dim:         domain.Dimensions{L: 800, W: 700, H: 1200},
		UprightAxes: []domain.Axis{domain.AxisH},
	}
	for _, o := range orientations(c) {
		if o.up != domain.AxisH {
			t.Fatalf("upright-only case produced orientation up=%s", o.up)
		}
		if o.dz != 1200 {
			t.Fatalf("upright orientation should keep H=1200 vertical, got dz=%d", o.dz)
		}
	}
}
