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
			StackableOn: nil,           // floor only
			UprightAxes: []Axis{AxisH}, // upright only
		},
		{
			ID: "case-speaker", Name: "Speaker case", Type: "speaker",
			Dim: Dimensions{L: 1000, W: 700, H: 700}, Weight: 60,
			StackableOn: []string{"speaker", "rack"},
			UprightAxes: []Axis{AxisH, AxisW}, // may lie on side
		},
		{
			ID: "case-cable", Name: "Cable trunk", Type: "trunk",
			Dim: Dimensions{L: 900, W: 600, H: 600}, Weight: 45,
			StackableOn: []string{"trunk", "rack", "speaker"},
			UprightAxes: []Axis{AxisH, AxisW, AxisL}, // any orientation
		},
		{
			ID: "case-lighting", Name: "Lighting case", Type: "trunk",
			Dim: Dimensions{L: 1200, W: 600, H: 500}, Weight: 55,
			StackableOn: []string{"trunk"},
			UprightAxes: []Axis{AxisH},
		},
	}
}
