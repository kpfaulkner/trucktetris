package domain

// SampleTruck returns a hardcoded truck for testing until the data-entry page
// (M4) exists. Rough numbers for a typical rigid box truck load space.
func SampleTruck() Truck {
	return Truck{
		ID:   "truck-sample",
		Name: "Sample 12t rigid",
		Dim:  Dimensions{L: 7200, W: 2400, H: 2400},
		Axles: []Axle{
			{Position: 1200, MaxLoad: 6000},  // steer/front
			{Position: 6000, MaxLoad: 10000}, // drive/rear
		},
		GrossMax:       12000,
		HeavyThreshold: 80, // only cases >= 80kg need to sit over the axles
	}
}

// StandardSemiTrailer returns a typical tautliner semi-trailer load space:
// ~13.6 m x 2.4 m x 2.7 m internal, ~24 t payload. Axle positions are measured
// from the front of the load space (kingpin end): the prime mover's drive-axle
// group sits near the front, the trailer's tri-axle group near the rear.
func StandardSemiTrailer() Truck {
	return Truck{
		ID:   "truck-semi-tautliner",
		Name: "Semi-trailer (tautliner, 13.6m)",
		Dim:  Dimensions{L: 13600, W: 2400, H: 2700},
		Axles: []Axle{
			{Position: 1600, MaxLoad: 15000},  // prime mover drive-axle group
			{Position: 10500, MaxLoad: 20000}, // trailer tri-axle group
		},
		GrossMax:       24000,
		HeavyThreshold: 0, // let the operator decide via the Manage UI
	}
}

// SampleTrucks returns the trucks seeded into an empty database.
func SampleTrucks() []Truck {
	return []Truck{SampleTruck(), StandardSemiTrailer()}
}

// SampleCases returns a hardcoded set of road cases for testing.
func SampleCases() []Case {
	return []Case{
		{
			ID: "case-amp", Name: "Amp rack", Type: "rack",
			Dim: Dimensions{L: 800, W: 700, H: 1200}, Weight: 90,
			Stackable:      true,
			StackableOn:    nil, // floor only
			MaxStackWeight: 120,
			CanLieOnSide:   false, // upright only
		},
		{
			ID: "case-speaker", Name: "Speaker case", Type: "speaker",
			Dim: Dimensions{L: 1000, W: 700, H: 700}, Weight: 60,
			Stackable:      true,
			StackableOn:    []string{"speaker", "rack"},
			MaxStackWeight: 80,
			CanLieOnSide:   true,
		},
		{
			ID: "case-cable", Name: "Cable trunk", Type: "trunk",
			Dim: Dimensions{L: 900, W: 600, H: 600}, Weight: 45,
			Stackable:      true,
			StackableOn:    []string{"trunk", "rack", "speaker"},
			MaxStackWeight: 60,
			CanLieOnSide:   true,
		},
		{
			ID: "case-lighting", Name: "Lighting case", Type: "trunk",
			Dim: Dimensions{L: 1200, W: 600, H: 500}, Weight: 55,
			Stackable:      false, // sloped top, nothing on top
			StackableOn:    []string{"trunk"},
			MaxStackWeight: 0,
			CanLieOnSide:   false,
		},
	}
}
