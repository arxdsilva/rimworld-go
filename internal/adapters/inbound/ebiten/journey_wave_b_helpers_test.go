package ebiten

// journey_wave_b_helpers_test.go: fixtures for the wave B journeys (PRD-1
// bill settings, BED-1 bedrolls and forbidden beds, DEF-1 hit chance and
// burst fire). The read-model names of those phases land with their
// agents, so lookups of not-yet-pinned names go through reflection and
// fail with a message that says what the contract lacks.

import (
	"sort"
	"strings"

	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// wbIDs lists every button and widget id on screen after one Update.
func wbIDs(j *journey) []string {
	j.t.Helper()
	j.update(1)
	seen := map[string]bool{}
	for _, b := range j.a.layoutHits(j.a.sw, j.a.sh) {
		seen[b.ID] = true
	}
	for _, w := range j.a.uiSt().Cur {
		seen[w.ID] = true
	}
	var out []string
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// wbFind is the first on-screen id containing every substring (lower case
// compare); "" when none does.
func wbFind(j *journey, subs ...string) string {
	j.t.Helper()
	for _, id := range wbIDs(j) {
		low := strings.ToLower(id)
		ok := true
		for _, s := range subs {
			ok = ok && strings.Contains(low, s)
		}
		if ok {
			return id
		}
	}
	return ""
}

// wbClick clicks the first control whose id holds every substring; any of
// the alternatives lists (separated by "|" inside one substring) match.
func wbClick(j *journey, what string, alts ...[]string) {
	j.t.Helper()
	for _, subs := range alts {
		if id := wbFind(j, subs...); id != "" {
			if !j.clickID(id) {
				j.t.Fatalf("%s: %q is disabled", what, id)
			}
			return
		}
	}
	var billish []string
	for _, id := range wbIDs(j) {
		if strings.Contains(strings.ToLower(id), "bill") {
			billish = append(billish, id)
		}
	}
	j.t.Fatalf("%s: no control matching any of %q (contract not landed or ids differ); bill ids on screen: %q", what, alts, billish)
}

// wbBench builds a workbench at (x, y), adds recipe r bills through the
// command path and selects the bench with a real click, Bills tab open.
func wbBench(j *journey, x, y int, recipes ...uint8) {
	j.t.Helper()
	if !j.g.BuildNow("workbench", "wood", "south", x, y) {
		j.t.Fatalf("no workbench at (%d, %d)", x, y)
	}
	for _, r := range recipes {
		j.g.Enqueue(command.Command{Kind: command.AddBill, X: x, Y: y, Recipe: r, Mode: 0, Count: 1})
	}
	j.g.ApplyPending()
	j.lookAt(x, y)
	j.update(1)
	j.clickTile(x, y, 0, 0)
	if !j.a.sel.HasBuilding {
		j.t.Fatalf("clicking the workbench at (%d, %d) selected no building", x, y)
	}
	j.clickID("inspector.bills")
}

// wbBuildingAt is the building view of the kind at (x, y).
func wbBuildingAt(j *journey, x, y int) (id entity.EntityID) {
	j.t.Helper()
	for _, b := range j.world().Buildings {
		if b.X == x && b.Y == y && !b.Floor {
			return b.ID
		}
	}
	j.t.Fatalf("no building at (%d, %d)", x, y)
	return 0
}

// wbPaneLines are the selected building's inspector lines, joined.
func wbPaneLines(j *journey, id entity.EntityID) string {
	j.t.Helper()
	return strings.ToLower(strings.Join(j.a.insp.Building().BuildingPane(id).Lines, "\n"))
}
