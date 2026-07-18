package domain

import "testing"

func validCase() Case {
	return Case{
		Name: "ok", Dim: Dimensions{L: 100, W: 100, H: 100}, Weight: 10,
	}
}

func TestCaseValidate(t *testing.T) {
	tests := map[string]struct {
		mut     func(*Case)
		wantErr bool
	}{
		"valid":              {func(*Case) {}, false},
		"empty name":         {func(c *Case) { c.Name = "" }, true},
		"zero dimension":     {func(c *Case) { c.Dim.W = 0 }, true},
		"negative weight":    {func(c *Case) { c.Weight = -1 }, true},
		"negative bearing":   {func(c *Case) { c.MaxStackWeight = -5 }, true},
		"stackable is valid": {func(c *Case) { c.Stackable = true; c.MaxStackWeight = 50 }, false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := validCase()
			tc.mut(&c)
			if err := c.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func validTruck() Truck {
	return Truck{
		Name: "ok", Dim: Dimensions{L: 5000, W: 2000, H: 2000}, GrossMax: 8000,
		Axles: []Axle{{Position: 1000, MaxLoad: 4000}},
	}
}

func TestTruckValidate(t *testing.T) {
	tests := map[string]struct {
		mut     func(*Truck)
		wantErr bool
	}{
		"valid":           {func(*Truck) {}, false},
		"empty name":      {func(t *Truck) { t.Name = "" }, true},
		"zero dimension":  {func(t *Truck) { t.Dim.L = 0 }, true},
		"zero gross":      {func(t *Truck) { t.GrossMax = 0 }, true},
		"axle beyond len": {func(t *Truck) { t.Axles[0].Position = 9999 }, true},
		"axle zero load":  {func(t *Truck) { t.Axles[0].MaxLoad = 0 }, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tk := validTruck()
			tc.mut(&tk)
			if err := tk.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
