// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
)

func TestFloorPlacementRespectsObjectsCoveringFloors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		kind  construction.Kind
		allow bool
	}{
		{construction.Wall, false},
		{construction.Cooler, false},
		{construction.Vent, false},
		{construction.Fence, false},
		{construction.FenceGate, false},
		{construction.Grave, false},
		{construction.Bed, true},
		{construction.Table, true},
		{construction.Door, true},
		{construction.Lamp, true},
	} {
		t.Run(tc.kind.String(), func(t *testing.T) {
			for _, planned := range []bool{false, true} {
				g := New(Config{Seed: 1, Width: 16, Height: 12})
				id := g.entities.Next()
				if planned {
					g.frameIDs = append(g.frameIDs, id)
					g.frames.Set(id, construction.Frame{Kind: tc.kind, Stuff: item.Wood, X: 7, Y: 6})
					g.noteFrame(id)
				} else {
					g.bldgIDs = append(g.bldgIDs, id)
					g.bldgs.Set(id, construction.Building{Kind: tc.kind, Stuff: item.Wood, X: 7, Y: 6})
					g.noteBuilding(id)
				}
				if got, why := g.CanPlace("floor", 7, 6); got != tc.allow {
					t.Errorf("%s planned=%v: floor allowed %v, reason %q; want allowed %v",
						tc.kind, planned, got, why, tc.allow)
				}
			}
		})
	}
}

func TestNativeFloorAndBedShareTileInEitherBuildOrder(t *testing.T) {
	t.Parallel()
	for _, first := range []string{"floor", "bed"} {
		t.Run(first, func(t *testing.T) {
			g := New(Config{Seed: 5, Width: 18, Height: 12})
			second := "floor"
			if first == "floor" {
				second = "bed"
			}
			for _, kind := range []string{first, second} {
				if !g.BuildNow(kind, "wood", "south", 8, 6) {
					t.Fatalf("could not build %s after %s", kind, first)
				}
			}
			floor := g.layerBldgAt(8, 6, construction.LayerFloor)
			bed := g.layerBldgAt(8, 6, construction.LayerObject)
			if floor == 0 || bed == 0 || floor == bed {
				t.Fatalf("shared cell: floor=%d bed=%d", floor, bed)
			}
			for _, kind := range []string{"floor", "bed"} {
				if ok, reason := g.CanPlace(kind, 8, 6); ok {
					t.Errorf("duplicate %s placement accepted (%s)", kind, reason)
				}
			}
		})
	}
}
