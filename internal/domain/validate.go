package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Validate reports whether the case is well-formed. IDs are not checked here;
// the store owns identity.
func (c Case) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if c.Dim.L <= 0 || c.Dim.W <= 0 || c.Dim.H <= 0 {
		errs = append(errs, fmt.Errorf("dimensions must be positive, got %dx%dx%d",
			c.Dim.L, c.Dim.W, c.Dim.H))
	}
	if c.Weight <= 0 {
		errs = append(errs, fmt.Errorf("weight must be positive, got %d", c.Weight))
	}
	if c.MaxStackWeight < 0 {
		errs = append(errs, fmt.Errorf("max stack weight cannot be negative, got %d", c.MaxStackWeight))
	}
	return errors.Join(errs...)
}

// Validate reports whether the truck is well-formed.
func (t Truck) Validate() error {
	var errs []error
	if strings.TrimSpace(t.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if t.Dim.L <= 0 || t.Dim.W <= 0 || t.Dim.H <= 0 {
		errs = append(errs, fmt.Errorf("dimensions must be positive, got %dx%dx%d",
			t.Dim.L, t.Dim.W, t.Dim.H))
	}
	if t.GrossMax <= 0 {
		errs = append(errs, fmt.Errorf("gross max weight must be positive, got %d", t.GrossMax))
	}
	for i, a := range t.Axles {
		if a.Position < 0 || a.Position > t.Dim.L {
			errs = append(errs, fmt.Errorf("axle %d position %d outside truck length %d",
				i, a.Position, t.Dim.L))
		}
		if a.MaxLoad <= 0 {
			errs = append(errs, fmt.Errorf("axle %d max load must be positive, got %d",
				i, a.MaxLoad))
		}
	}
	return errors.Join(errs...)
}
