package domain

import "sort"

// Evaluation is the result of checking an arbitrary set of placements (e.g.
// after manual repositioning) against a truck's limits and geometry.
type Evaluation struct {
	AxleLoads   []AxleLoad `json:"axleLoads"`
	TotalWeight int        `json:"totalWeight"` // kg
	OverGross   bool       `json:"overGross"`
	// Collisions lists case IDs that overlap another placement.
	Collisions []string `json:"collisions"`
	// OutOfBounds lists case IDs sticking outside the truck load space.
	OutOfBounds []string `json:"outOfBounds"`
	// Unsupported lists stacked case IDs that float (no box or floor beneath).
	Unsupported []string `json:"unsupported"`
	// IllegalStacks lists case IDs resting on a case they may not stack on
	// (support not stackable, or type not permitted).
	IllegalStacks []string `json:"illegalStacks"`
	// Overloaded lists case IDs bearing more weight on top than their
	// MaxStackWeight allows.
	Overloaded []string `json:"overloaded"`
}

// EvaluatePlan checks placements against the truck. cases maps case ID to the
// full Case (for weight, type, and stacking rules); a placement whose case ID
// is absent is treated as an empty Case. It reuses the axle-load model and
// reports overlaps, out-of-bounds boxes, floating boxes, illegal stacks, and
// bearing overloads so a manual editor can flag violations without re-running
// the solver.
func EvaluatePlan(t Truck, placements []Placement, cases map[string]Case) Evaluation {
	var e Evaluation

	loads := make([]PointLoad, 0, len(placements))
	collision := map[string]bool{}

	for i, p := range placements {
		c := cases[p.CaseID]
		e.TotalWeight += c.Weight
		loads = append(loads, PointLoad{X: p.Pos[0] + p.Size[0]/2, Weight: c.Weight})

		if p.Pos[0] < 0 || p.Pos[1] < 0 || p.Pos[2] < 0 ||
			p.Pos[0]+p.Size[0] > t.Dim.L ||
			p.Pos[1]+p.Size[1] > t.Dim.W ||
			p.Pos[2]+p.Size[2] > t.Dim.H {
			e.OutOfBounds = append(e.OutOfBounds, p.CaseID)
		}

		for j := i + 1; j < len(placements); j++ {
			if boxesOverlap(p, placements[j]) {
				collision[p.CaseID] = true
				collision[placements[j].CaseID] = true
			}
		}
	}

	for _, p := range placements {
		if collision[p.CaseID] {
			e.Collisions = append(e.Collisions, p.CaseID)
		}
	}

	evaluateStacking(&e, placements, cases)

	e.AxleLoads = ComputeAxleLoads(t, loads)
	e.OverGross = e.TotalWeight > t.GrossMax
	return e
}

// evaluateStacking builds the support graph and checks stack legality, support,
// and bearing capacity, appending violations to e in placement order.
func evaluateStacking(e *Evaluation, placements []Placement, cases map[string]Case) {
	n := len(placements)
	supporters := make([][]int, n) // indices of boxes directly beneath box i

	for i, p := range placements {
		if p.Pos[2] == 0 {
			continue // on the floor
		}
		for j, q := range placements {
			if i == j {
				continue
			}
			// q supports p if q's top meets p's bottom and their footprints overlap.
			if q.Pos[2]+q.Size[2] == p.Pos[2] && footprintOverlap(p, q) {
				supporters[i] = append(supporters[i], j)
			}
		}
	}

	unsupported := map[string]bool{}
	illegal := map[string]bool{}
	for i, p := range placements {
		if p.Pos[2] == 0 {
			continue
		}
		if len(supporters[i]) == 0 {
			unsupported[p.CaseID] = true
			continue
		}
		top := cases[p.CaseID]
		for _, s := range supporters[i] {
			sup := cases[placements[s].CaseID]
			if !sup.Stackable || !stackableOnType(top, sup.Type) {
				illegal[p.CaseID] = true
			}
		}
	}

	// Bearing: process boxes top-first so load accumulates downward. Each box's
	// total load (own weight + what rests on it) is shared equally among its
	// direct supporters.
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return placements[order[a]].Pos[2] > placements[order[b]].Pos[2]
	})

	loadOn := make([]float64, n)
	for _, i := range order {
		total := float64(cases[placements[i].CaseID].Weight) + loadOn[i]
		sup := supporters[i]
		if len(sup) == 0 {
			continue
		}
		share := total / float64(len(sup))
		for _, s := range sup {
			loadOn[s] += share
		}
	}

	overloaded := map[string]bool{}
	for i, p := range placements {
		if loadOn[i] > float64(cases[p.CaseID].MaxStackWeight) {
			overloaded[p.CaseID] = true
		}
	}

	for _, p := range placements {
		if unsupported[p.CaseID] {
			e.Unsupported = append(e.Unsupported, p.CaseID)
		}
		if illegal[p.CaseID] {
			e.IllegalStacks = append(e.IllegalStacks, p.CaseID)
		}
		if overloaded[p.CaseID] {
			e.Overloaded = append(e.Overloaded, p.CaseID)
		}
	}
}

func stackableOnType(c Case, supportType string) bool {
	for _, ty := range c.StackableOn {
		if ty == supportType {
			return true
		}
	}
	return false
}

// footprintOverlap reports whether two placements overlap in plan view (x/y),
// ignoring height.
func footprintOverlap(a, b Placement) bool {
	return a.Pos[0] < b.Pos[0]+b.Size[0] && b.Pos[0] < a.Pos[0]+a.Size[0] &&
		a.Pos[1] < b.Pos[1]+b.Size[1] && b.Pos[1] < a.Pos[1]+a.Size[1]
}

// boxesOverlap reports whether two placements share interior volume. Touching
// faces do not count.
func boxesOverlap(a, b Placement) bool {
	for i := range 3 {
		if a.Pos[i]+a.Size[i] <= b.Pos[i] || b.Pos[i]+b.Size[i] <= a.Pos[i] {
			return false
		}
	}
	return true
}
