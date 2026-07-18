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
		GrossMax: 12000,
	}
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
