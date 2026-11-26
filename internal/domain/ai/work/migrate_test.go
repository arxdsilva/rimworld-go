// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package work

import "testing"

func TestLegacyMaskWidths(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
		bits    uint16
		want    Mask
	}{
		{"v1", 1, 1<<Grow | 1<<Craft | 1<<11,
			Mask(1<<Grow | 1<<Craft | 1<<Handle | 1<<Patient | 1<<BedRest | 1<<Basic | 1<<Smith | 1<<Tailor | 1<<Art)},
		{"v2", 2, 1<<Handle | 1<<Warden | 1<<Craft,
			Mask(1<<Handle | 1<<Warden | 1<<Craft | 1<<Patient | 1<<BedRest | 1<<Basic | 1<<Smith | 1<<Tailor | 1<<Art)},
		{"v7", 7, 1<<PlantCut | 1<<Grow,
			Mask(1<<PlantCut | 1<<Grow | 1<<Patient | 1<<BedRest | 1<<Basic)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := LegacyMask(tc.version, tc.bits, 0, false)
			if got != tc.want {
				t.Fatalf("legacy mask %b, want %b", got, tc.want)
			}
		})
	}
	old := LegacyMask(1, 1<<Craft, Mask(1<<PlantCut), true)
	if !old.Enabled(PlantCut) {
		t.Fatal("legacy derived plant-cut flag lost")
	}
}
