package domain

import (
	"slices"
	"testing"
)

func evalTruck() Truck {
	return Truck{
		Dim: Dimensions{L: 5000, W: 2000, H: 2000}, GrossMax: 1000,
		Axles: []Axle{{Position: 1000, MaxLoad: 800}, {Position: 4000, MaxLoad: 800}},
	}
}

func place(id string, x, y, z, dx, dy, dz int) Placement {
	// Tests use unique ids, so instance == case id here.
	return Placement{InstanceID: id, CaseID: id, Pos: [3]int{x, y, z}, Size: [3]int{dx, dy, dz}}
}

// wcases builds a case map from id->weight, with no stacking rules.
func wcases(m map[string]int) map[string]Case {
	out := map[string]Case{}
	for id, w := range m {
		out[id] = Case{ID: id, Weight: w}
	}
	return out
}

func TestEvaluatePlanClean(t *testing.T) {
	tk := evalTruck()
	ps := []Placement{
		place("a", 0, 0, 0, 500, 500, 500),
		place("b", 600, 0, 0, 500, 500, 500),
	}
	ev := EvaluatePlan(tk, ps, wcases(map[string]int{"a": 100, "b": 200}))

	if ev.TotalWeight != 300 || ev.OverGross {
		t.Fatalf("weight=%d overGross=%v, want 300/false", ev.TotalWeight, ev.OverGross)
	}
	if len(ev.Collisions) != 0 || len(ev.OutOfBounds) != 0 {
		t.Fatalf("expected clean, got collisions=%v oob=%v", ev.Collisions, ev.OutOfBounds)
	}
	if len(ev.AxleLoads) != 2 {
		t.Fatalf("expected 2 axle loads, got %d", len(ev.AxleLoads))
	}
}

func TestEvaluatePlanDetectsOverlap(t *testing.T) {
	tk := evalTruck()
	ps := []Placement{
		place("a", 0, 0, 0, 500, 500, 500),
		place("b", 200, 200, 0, 500, 500, 500), // overlaps a
	}
	ev := EvaluatePlan(tk, ps, wcases(map[string]int{"a": 100, "b": 100}))
	if !slices.Contains(ev.Collisions, "a") || !slices.Contains(ev.Collisions, "b") {
		t.Fatalf("expected a and b flagged as colliding, got %v", ev.Collisions)
	}
}

func TestEvaluatePlanTouchingFacesNoOverlap(t *testing.T) {
	tk := evalTruck()
	ps := []Placement{
		place("a", 0, 0, 0, 500, 500, 500),
		place("b", 500, 0, 0, 500, 500, 500), // shares a face, no interior overlap
	}
	ev := EvaluatePlan(tk, ps, wcases(map[string]int{"a": 100, "b": 100}))
	if len(ev.Collisions) != 0 {
		t.Fatalf("touching faces should not collide, got %v", ev.Collisions)
	}
}

func TestEvaluatePlanDetectsOutOfBounds(t *testing.T) {
	tk := evalTruck()
	ps := []Placement{place("a", 4800, 0, 0, 500, 500, 500)} // pokes past L=5000
	ev := EvaluatePlan(tk, ps, wcases(map[string]int{"a": 100}))
	if !slices.Contains(ev.OutOfBounds, "a") {
		t.Fatalf("expected a out of bounds, got %v", ev.OutOfBounds)
	}
}

func TestEvaluatePlanOverGross(t *testing.T) {
	tk := evalTruck() // gross 1000
	ps := []Placement{place("a", 0, 0, 0, 500, 500, 500)}
	ev := EvaluatePlan(tk, ps, wcases(map[string]int{"a": 1500}))
	if !ev.OverGross {
		t.Fatal("expected over gross")
	}
}

func TestEvaluatePlanBearingOverload(t *testing.T) {
	tk := evalTruck()
	// bertha (200kg) stacked on base that only bears 50kg.
	ps := []Placement{
		place("base", 0, 0, 0, 500, 500, 500),
		place("bertha", 0, 0, 500, 500, 500, 500),
	}
	cases := map[string]Case{
		"base":   {ID: "base", Type: "b", Weight: 80, Stackable: true, MaxStackWeight: 50},
		"bertha": {ID: "bertha", Type: "x", Weight: 200, StackableOn: []string{"b"}},
	}
	ev := EvaluatePlan(tk, ps, cases)
	if !slices.Contains(ev.Overloaded, "base") {
		t.Fatalf("base should be overloaded by 200kg on top, got %v", ev.Overloaded)
	}
}

func TestEvaluatePlanBearingWithinLimit(t *testing.T) {
	tk := evalTruck()
	ps := []Placement{
		place("base", 0, 0, 0, 500, 500, 500),
		place("light", 0, 0, 500, 500, 500, 500),
	}
	cases := map[string]Case{
		"base":  {ID: "base", Type: "b", Weight: 80, Stackable: true, MaxStackWeight: 100},
		"light": {ID: "light", Type: "x", Weight: 40, StackableOn: []string{"b"}},
	}
	ev := EvaluatePlan(tk, ps, cases)
	if len(ev.Overloaded) != 0 {
		t.Fatalf("40kg within 100kg limit, got overloaded=%v", ev.Overloaded)
	}
}

func TestEvaluatePlanIllegalStackNotStackable(t *testing.T) {
	tk := evalTruck()
	ps := []Placement{
		place("base", 0, 0, 0, 500, 500, 500),
		place("top", 0, 0, 500, 500, 500, 500),
	}
	cases := map[string]Case{
		"base": {ID: "base", Type: "b", Weight: 80, Stackable: false, MaxStackWeight: 999},
		"top":  {ID: "top", Type: "x", Weight: 10, StackableOn: []string{"b"}},
	}
	ev := EvaluatePlan(tk, ps, cases)
	if !slices.Contains(ev.IllegalStacks, "top") {
		t.Fatalf("stacking on a non-stackable base should be illegal, got %v", ev.IllegalStacks)
	}
}

func TestEvaluatePlanIllegalStackWrongType(t *testing.T) {
	tk := evalTruck()
	ps := []Placement{
		place("base", 0, 0, 0, 500, 500, 500),
		place("top", 0, 0, 500, 500, 500, 500),
	}
	cases := map[string]Case{
		"base": {ID: "base", Type: "b", Weight: 80, Stackable: true, MaxStackWeight: 999},
		"top":  {ID: "top", Type: "x", Weight: 10, StackableOn: []string{"other"}}, // not "b"
	}
	ev := EvaluatePlan(tk, ps, cases)
	if !slices.Contains(ev.IllegalStacks, "top") {
		t.Fatalf("stacking on a disallowed type should be illegal, got %v", ev.IllegalStacks)
	}
}

func TestEvaluatePlanFloatingUnsupported(t *testing.T) {
	tk := evalTruck()
	// box sits at z=500 with nothing beneath.
	ps := []Placement{place("floater", 0, 0, 500, 500, 500, 500)}
	cases := map[string]Case{"floater": {ID: "floater", Weight: 10}}
	ev := EvaluatePlan(tk, ps, cases)
	if !slices.Contains(ev.Unsupported, "floater") {
		t.Fatalf("floating box should be unsupported, got %v", ev.Unsupported)
	}
}
