package domain

import "sort"

// PointLoad is a weight acting at a position along the truck length.
type PointLoad struct {
	X      int // mm from front (centre of mass along length)
	Weight int // kg
}

// ComputeAxleLoads distributes point loads across the truck's axles and returns
// one AxleLoad per axle, in Truck.Axles order.
//
// Distribution model (a practical heuristic, not full statics): each load is
// split between the two axles that bracket it, by the lever rule — the closer
// axle takes the larger share. A load ahead of the first axle or behind the
// last is assigned entirely to that nearest axle (overhang clamped, no negative
// reactions). With a single axle, it carries everything.
func ComputeAxleLoads(t Truck, loads []PointLoad) []AxleLoad {
	out := make([]AxleLoad, len(t.Axles))
	for i, a := range t.Axles {
		out[i] = AxleLoad{Position: a.Position, MaxLoad: a.MaxLoad}
	}
	if len(t.Axles) == 0 {
		return out
	}

	// Work on axle indices sorted by position so bracketing is well-defined,
	// but report in the original order.
	order := make([]int, len(t.Axles))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return t.Axles[order[i]].Position < t.Axles[order[j]].Position
	})

	// loadKg accumulates onto original axle indices.
	loadKg := make([]float64, len(t.Axles))

	for _, pl := range loads {
		if len(order) == 1 {
			loadKg[order[0]] += float64(pl.Weight)
			continue
		}
		first, last := order[0], order[len(order)-1]
		switch {
		case pl.X <= t.Axles[first].Position:
			loadKg[first] += float64(pl.Weight)
		case pl.X >= t.Axles[last].Position:
			loadKg[last] += float64(pl.Weight)
		default:
			// Find the bracketing pair (lo, hi) in sorted order.
			for k := 0; k < len(order)-1; k++ {
				lo, hi := order[k], order[k+1]
				pLo, pHi := t.Axles[lo].Position, t.Axles[hi].Position
				if pl.X >= pLo && pl.X <= pHi {
					span := float64(pHi - pLo)
					fracHi := float64(pl.X-pLo) / span
					loadKg[hi] += float64(pl.Weight) * fracHi
					loadKg[lo] += float64(pl.Weight) * (1 - fracHi)
					break
				}
			}
		}
	}

	for i := range out {
		out[i].Load = int(loadKg[i] + 0.5) // round to nearest kg
		out[i].Over = out[i].Load > out[i].MaxLoad
	}
	return out
}
