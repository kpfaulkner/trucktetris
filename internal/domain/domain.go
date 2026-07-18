// Package domain holds the core Truck Tetris types.
//
// Units convention: all dimensions are in millimetres (mm) and all weights are
// in kilograms (kg). No other units appear anywhere in the system.
package domain

// Axis identifies one of a case's three dimensions. Used to describe which face
// of a case may sit on the ground (its orientation on the truck floor).
type Axis string

const (
	AxisL Axis = "L" // length runs vertically (case stood on its end)
	AxisW Axis = "W" // width runs vertically (case laid on its side)
	AxisH Axis = "H" // height runs vertically (regular, upright orientation)
)

// Dimensions of a box in millimetres.
type Dimensions struct {
	L int `json:"l"` // length, mm
	W int `json:"w"` // width, mm
	H int `json:"h"` // height, mm
}

// Case is a single road case to be loaded.
type Case struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Dim    Dimensions `json:"dim"`
	Weight int        `json:"weight"` // kg
	Type   string     `json:"type"`   // category, drives stacking + colour

	// Stackable reports whether other cases may be placed on top of this one at
	// all. False means nothing stacks on it, regardless of weight.
	Stackable bool `json:"stackable"`

	// StackableOn lists the case types this case may be placed on top of.
	// Empty means it may only sit on the truck floor. This is a finer,
	// type-level rule layered on top of the supporting case's Stackable flag.
	StackableOn []string `json:"stackableOn"`

	// MaxStackWeight is the maximum weight, kg, this case may bear on top of it.
	// Only meaningful when Stackable is true.
	MaxStackWeight int `json:"maxStackWeight"`

	// CanLieOnSide reports whether the case may be rotated off its upright
	// orientation (onto a side or end). The upright orientation is always
	// allowed; when true the packer may also lay the case down.
	CanLieOnSide bool `json:"canLieOnSide"`
}

// Axle is one axle (or axle group) of a truck.
type Axle struct {
	// Position is the distance from the front of the load space, mm.
	Position int `json:"position"`
	// MaxLoad is the maximum weight this axle may carry, kg.
	MaxLoad int `json:"maxLoad"`
}

// Truck (or trailer) load space.
type Truck struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Dim is the internal usable load space, mm.
	Dim Dimensions `json:"dim"`
	// Axles along the length of the load space.
	Axles []Axle `json:"axles"`
	// GrossMax is the maximum total cargo weight, kg.
	GrossMax int `json:"grossMax"`
	// HeavyThreshold is the weight, kg, at or above which a case should be
	// biased to sit over the axles. Lighter cases ignore the axle bias and pack
	// for density. Zero disables the bias entirely. Per-axle max load is always
	// enforced regardless of this value.
	HeavyThreshold int `json:"heavyThreshold"`
}

// IsHeavy reports whether case weight w should be biased over the axles.
func (t Truck) IsHeavy(w int) bool {
	return t.HeavyThreshold > 0 && w >= t.HeavyThreshold
}

// Placement positions one case inside the truck.
type Placement struct {
	CaseID string `json:"caseId"`
	// Pos is the origin corner (minimum x,y,z) of the case, mm, relative to the
	// front-left-floor of the load space. x = along length, y = across width,
	// z = up.
	Pos [3]int `json:"pos"`
	// Size is the case's world-aligned extent (dx,dy,dz) in mm once oriented,
	// matching the Pos axes. Fully describes the box footprint + height, which
	// Up alone cannot when length != width.
	Size [3]int `json:"size"`
	// Up is which case axis points up in this placement.
	Up Axis `json:"up"`
}

// LoadPlan is the result of a packing run.
type LoadPlan struct {
	Truck      Truck       `json:"truck"`
	Placements []Placement `json:"placements"`
	// Unplaced lists case IDs that did not fit.
	Unplaced []string `json:"unplaced"`
	Summary  Summary  `json:"summary"`
	// AxleLoads reports the computed load on each truck axle, one per Truck.Axles
	// entry in the same order.
	AxleLoads []AxleLoad `json:"axleLoads"`
}

// AxleLoad is the computed load carried by one axle.
type AxleLoad struct {
	Position int  `json:"position"` // mm from front, mirrors Axle.Position
	Load     int  `json:"load"`     // kg carried by this axle
	MaxLoad  int  `json:"maxLoad"`  // kg limit, mirrors Axle.MaxLoad
	Over     bool `json:"over"`     // true when Load exceeds MaxLoad
}

// Summary holds headline stats for a LoadPlan.
type Summary struct {
	PlacedCount   int `json:"placedCount"`
	UnplacedCount int `json:"unplacedCount"`
	TotalWeight   int `json:"totalWeight"` // kg, placed cases only
	// VolumeUtilPct is placed-case volume as a percentage of the truck's load
	// space volume. WeightUtilPct is total weight as a percentage of GrossMax.
	VolumeUtilPct int `json:"volumeUtilPct"`
	WeightUtilPct int `json:"weightUtilPct"`
}

// SolveRequest is the input to the solver API.
type SolveRequest struct {
	Truck Truck  `json:"truck"`
	Cases []Case `json:"cases"`
}

// SavedPlan is a named, persisted load plan capturing the truck selection and
// the (possibly manually edited) placements.
type SavedPlan struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	TruckID    string      `json:"truckId"`
	Placements []Placement `json:"placements"`
	Unplaced   []string    `json:"unplaced"`
	CreatedAt  string      `json:"createdAt"`
}
