// Package packer contains the box-packing solvers.
//
// M2 provides a volume-only shelf/layer heuristic. Weight, axle, stacking and
// unload-order constraints arrive in later milestones as new Packer
// implementations or extensions of this one.
package packer

import (
	"sort"

	"github.com/kenfaulkner/trucktetris/internal/domain"
)

// Packer turns a solve request into a load plan. Every solver milestone is a
// new implementation behind this interface.
type Packer interface {
	Pack(req domain.SolveRequest) domain.LoadPlan
}

// orientation is one allowed way to place a case: dx,dy,dz are the case's
// footprint/height once oriented, and up is which case axis points up.
type orientation struct {
	dx, dy, dz int
	up         domain.Axis
}

// orientations returns every axis-aligned placement allowed for a case: for
// each permitted up-axis, both 90°-yaw footprints. Duplicates (square faces)
// are removed.
func orientations(c domain.Case) []orientation {
	d := c.Dim
	var out []orientation
	seen := map[orientation]bool{}

	add := func(fx, fy, fz int, up domain.Axis) {
		for _, o := range []orientation{{fx, fy, fz, up}, {fy, fx, fz, up}} {
			if !seen[o] {
				seen[o] = true
				out = append(out, o)
			}
		}
	}

	for _, up := range c.UprightAxes {
		switch up {
		case domain.AxisH: // height vertical (upright)
			add(d.L, d.W, d.H, up)
		case domain.AxisW: // width vertical (on its side)
			add(d.L, d.H, d.W, up)
		case domain.AxisL: // length vertical (on its end)
			add(d.W, d.H, d.L, up)
		}
	}
	return out
}

// Shelf is the M2 volume-only packer. It fills the load space with a
// shelf/layer sweep: cases run along the length (x); when a case will not fit
// the remaining length the cursor wraps to a new row across the width (y); when
// a row will not fit the remaining width it wraps to a new layer up the height
// (z).
type Shelf struct{}

// Pack implements Packer.
func (Shelf) Pack(req domain.SolveRequest) domain.LoadPlan {
	t := req.Truck

	// Largest-volume cases first — big items are hardest to place late.
	cases := make([]domain.Case, len(req.Cases))
	copy(cases, req.Cases)
	sort.SliceStable(cases, func(i, j int) bool {
		return volume(cases[i]) > volume(cases[j])
	})

	plan := domain.LoadPlan{
		Truck:      t,
		Placements: []domain.Placement{},
		Unplaced:   []string{},
	}

	// Shelf cursor and running maxima for the current row and layer.
	var x, y, z int
	var rowDepth, layerHeight int

	for _, c := range cases {
		o, ok := placeCase(c, t, x, y, z)
		if !ok {
			// Try wrapping to a new row, then a new layer, before giving up.
			x = 0
			y += rowDepth
			rowDepth = 0
			o, ok = placeCase(c, t, x, y, z)
			if !ok {
				y = 0
				z += layerHeight
				layerHeight = 0
				rowDepth = 0
				o, ok = placeCase(c, t, x, y, z)
			}
		}
		if !ok {
			plan.Unplaced = append(plan.Unplaced, c.ID)
			continue
		}

		plan.Placements = append(plan.Placements, domain.Placement{
			CaseID: c.ID,
			Pos:    [3]int{x, y, z},
			Up:     o.up,
		})
		plan.Summary.TotalWeight += c.Weight

		x += o.dx
		rowDepth = max(rowDepth, o.dy)
		layerHeight = max(layerHeight, o.dz)
	}

	plan.Summary.PlacedCount = len(plan.Placements)
	plan.Summary.UnplacedCount = len(plan.Unplaced)
	return plan
}

// placeCase picks the first allowed orientation of c that fits within the truck
// bounds when its origin corner sits at (x,y,z).
func placeCase(c domain.Case, t domain.Truck, x, y, z int) (orientation, bool) {
	for _, o := range orientations(c) {
		if x+o.dx <= t.Dim.L && y+o.dy <= t.Dim.W && z+o.dz <= t.Dim.H {
			return o, true
		}
	}
	return orientation{}, false
}

func volume(c domain.Case) int {
	return c.Dim.L * c.Dim.W * c.Dim.H
}
