package ebiten

import (
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// RSCH-2: on a real colony a click on a box selects the project; a locked
// one's Start research says what it needs first, an available one starts;
// locked tools wait on their project.
func TestResearchTabPicksProjectAndLocksTools(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	g.ResetResearch()
	click(t, a, "tab.research")
	if !a.win.Open || a.win.Mode != "research" {
		t.Fatal("research tab opens the research panel")
	}
	// a locked project whose missing prerequisite can start now
	locked, need := -1, -1
	views := g.World().Research
	for i, p := range views {
		if p.Available || p.Done || len(p.Requires) == 0 {
			continue
		}
		for j, q := range views {
			if q.Available && q.Label == p.Requires[0] {
				locked, need = i, j
			}
		}
		if locked >= 0 {
			break
		}
	}
	if locked < 0 {
		t.Fatal("no locked project with an available prerequisite")
	}
	a.win.Research().Pick("research.pick." + strconv.Itoa(int(views[locked].ID)))
	a.updateOnce(t)
	start, ok := a.uiSt().Find("research.start")
	if !ok || !start.Disabled || !strings.Contains(start.Reason, views[need].Label) {
		t.Fatalf("%s's Start research %+v should wait on %s", views[locked].Label, start, views[need].Label)
	}
	click(t, a, "research.pick."+strconv.Itoa(int(views[need].ID)))
	if id, ok := a.win.Research().Selected(); !ok || int(id) != int(views[need].ID) {
		t.Fatalf("clicking %s selected %v", views[need].Label, id)
	}
	click(t, a, "research.start")
	g.Step(1)
	if !g.World().Research[need].Current {
		t.Fatal("Start research makes the selected project current")
	}
	click(t, a, "tab.research") // close
	want := g.World().Locked["build:spike trap"]
	if want == "" {
		t.Skip("spike traps need no research in this table")
	}
	click(t, a, "tab.architect")
	click(t, a, "arch.cat.security")
	var trap uikit.Button
	for _, b := range a.layoutButtons(a.sw, a.sh) {
		if b.ID == "arch.tool.spike_trap" {
			trap = b
		}
	}
	if !trap.Disabled || !strings.Contains(trap.Reason, want) {
		t.Fatalf("spike trap tool locked until %s is researched: %+v", want, trap)
	}
}
