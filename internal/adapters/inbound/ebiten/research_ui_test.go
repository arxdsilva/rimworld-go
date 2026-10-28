package ebiten

import (
	"strings"
	"testing"
)

func TestResearchTabPicksProjectAndLocksTools(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	g.ResetResearch()
	click(t, a, "tab.research")
	if !a.workOpen || a.panelMode != "research" {
		t.Fatal("research tab opens the research panel")
	}
	var gun, traps button
	for _, b := range a.layoutButtons(a.sw, a.sh) {
		switch b.ID {
		case "research.pick.4":
			gun = b
		case "research.pick.3":
			traps = b
		}
	}
	if !gun.Disabled || !strings.Contains(gun.Reason, "Traps") || traps.Disabled {
		t.Fatalf("gunsmithing waits on traps: %+v / %+v", gun, traps)
	}
	click(t, a, "research.pick.3")
	g.Step(1)
	if !g.World().Research[3].Current {
		t.Fatal("clicking an available project makes it current")
	}
	click(t, a, "tab.research") // close
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.security")
	for _, b := range a.layoutButtons(a.sw, a.sh) {
		if b.ID == "arch.tool.spike_trap" && (!b.Disabled || !strings.Contains(b.Reason, "Traps")) {
			t.Fatalf("spike trap tool locked until researched: %+v", b)
		}
	}
}
