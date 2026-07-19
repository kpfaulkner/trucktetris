package packer

import (
	"testing"

	"github.com/kenfaulkner/trucktetris/internal/domain"
)

// The solver must rotate a case to fit when only a rotated orientation works.
func TestRotatesYawToFit(t *testing.T) {
	// 300-long case in a 150-long, 300-wide truck: fits only yawed across width.
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "t", Dim: domain.Dimensions{L: 150, W: 300, H: 200}, GrossMax: 9999},
		Cases: []domain.Case{{ID: "c", Type: "x", Dim: domain.Dimensions{L: 300, W: 100, H: 100}, Weight: 10}},
	}
	if p := (Stacker{}).Pack(req); p.Summary.PlacedCount != 1 {
		t.Fatalf("yaw-to-fit failed, placed=%d", p.Summary.PlacedCount)
	}
}

func TestLaysOnSideToFitOnlyWhenAllowed(t *testing.T) {
	// 200-tall case in a 120-high truck: fits only laid down.
	truck := domain.Truck{ID: "t", Dim: domain.Dimensions{L: 400, W: 400, H: 120}, GrossMax: 9999}
	base := domain.Case{ID: "s", Type: "x", Dim: domain.Dimensions{L: 300, W: 100, H: 200}, Weight: 10}

	yes := base
	yes.CanLieOnSide = true
	if p := (Stacker{}).Pack(domain.SolveRequest{Truck: truck, Cases: []domain.Case{yes}}); p.Summary.PlacedCount != 1 {
		t.Fatalf("should lay on side to fit, placed=%d", p.Summary.PlacedCount)
	}

	no := base
	no.CanLieOnSide = false
	if p := (Stacker{}).Pack(domain.SolveRequest{Truck: truck, Cases: []domain.Case{no}}); p.Summary.PlacedCount != 0 {
		t.Fatalf("upright-only case must not be laid down, placed=%d", p.Summary.PlacedCount)
	}
}

// The coarse-grid fallback should place a case into a gap the extreme points
// miss. Fill the floor front with one row, leave a gap behind it that only a
// grid position covers.
func TestFallbackGridFitsGap(t *testing.T) {
	// Long thin truck; three 1000-long cases fit end to end (3000 of 3200).
	req := domain.SolveRequest{
		Truck: domain.Truck{ID: "t", Dim: domain.Dimensions{L: 3200, W: 1000, H: 1000}, GrossMax: 99999},
		Cases: []domain.Case{
			{ID: "a", Type: "x", Dim: domain.Dimensions{L: 1000, W: 1000, H: 1000}, Weight: 10},
			{ID: "a", Type: "x", Dim: domain.Dimensions{L: 1000, W: 1000, H: 1000}, Weight: 10},
			{ID: "a", Type: "x", Dim: domain.Dimensions{L: 1000, W: 1000, H: 1000}, Weight: 10},
		},
	}
	if p := (Stacker{}).Pack(req); p.Summary.PlacedCount != 3 {
		t.Fatalf("all three should fit end to end, placed=%d", p.Summary.PlacedCount)
	}
}
