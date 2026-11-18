// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wizard

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/faction"
)

func TestFactionTipShowsThresholds(t *testing.T) {
	t.Parallel()
	got := GoodwillTip(FacRow{Relation: faction.Neutral})
	if want := "Neutral. At -75 or less they turn hostile; at +50 or more, friendly."; got != want {
		t.Errorf("neutral tip %q, want %q", got, want)
	}
	if h := GoodwillTip(FacRow{Relation: faction.Hostile}); !strings.Contains(h, "climb to 0 before") {
		t.Errorf("hostile tip %q", h)
	}
	if f := GoodwillTip(FacRow{Relation: faction.Friendly}); !strings.Contains(f, "+30 or more") {
		t.Errorf("friendly tip %q", f)
	}
}

func TestHostileCapableUsesBand(t *testing.T) {
	t.Parallel()
	for typ, want := range map[faction.Type]bool{
		faction.Drifters: true, faction.Freebooters: true, faction.Settlers: false, faction.Clans: false,
	} {
		if got := HostileCapable([]faction.Slot{{Type: typ, Count: 1}}); got != want {
			t.Errorf("HostileCapable(%v) = %v, want %v", typ, got, want)
		}
	}
	if faction.DefOf(faction.Drifters).GoodwillMin > faction.HostileAt {
		t.Error("drifters can no longer start hostile")
	}
}
