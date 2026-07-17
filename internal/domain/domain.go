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

	// StackableOn lists the case types this case may be placed on top of.
	// Empty means it may only sit on the truck floor.
	StackableOn []string `json:"stackableOn"`

	// UprightVertical lists which case axes are allowed to point up, i.e. the
	// permitted orientations. A case that must stay regular/upright has only
	// {AxisH}; a case that may also lie on its side includes AxisW and/or AxisL.
	UprightAxes []Axis `json:"uprightAxes"`
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
}

// Placement positions one case inside the truck.
type Placement struct {
	CaseID string `json:"caseId"`
	// Pos is the origin corner (minimum x,y,z) of the case, mm, relative to the
	// front-left-floor of the load space. x = along length, y = across width,
	// z = up.
	Pos [3]int `json:"pos"`
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
}

// Summary holds headline stats for a LoadPlan.
type Summary struct {
	PlacedCount   int `json:"placedCount"`
	UnplacedCount int `json:"unplacedCount"`
	TotalWeight   int `json:"totalWeight"` // kg, placed cases only
}

// SolveRequest is the input to the solver API.
type SolveRequest struct {
	Truck Truck  `json:"truck"`
	Cases []Case `json:"cases"`
}
