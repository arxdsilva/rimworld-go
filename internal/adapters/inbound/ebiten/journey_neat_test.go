package ebiten

// journey_neat_test.go: the Neat Storage journeys (NST-C): the Storage
// category, a crate placed from its card and filled by a hauler, the pane's
// n/m stacks and the Storage button, and a colonist admiring a display case.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/domain/economy/item"
)

// TestJourneyNeatStorage: the Architect's Storage category lists the 27
// pieces and the older shelves; a crate armed from its card is placed, built,
// and filled by a hauler; its pane reads n/m stacks and its Storage button
// opens the filter window on the crate's own item set.
func TestJourneyNeatStorage(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(30, 24), jPawns([2]int{8, 8}))
	j.g.ClearPlants(0, 0, 29, 23)
	j.openArch("storage", "")
	if j.a.arch.Cat != "storage" {
		t.Fatalf("category %q after opening Storage", j.a.arch.Cat)
	}
	for _, id := range []string{"display_case", "safe", "corpse_dump_spot", "shelf", "egg_box", "bookcase"} {
		if _, ok := architect.ToolByID(id); !ok {
			t.Fatalf("no %s card", id)
		}
	}
	ax2Place(j, "storage", "crate", "crate", 14, 2) // arms the crate card, places it
	if _, ok := ax2Frame(j, "crate", 14, 2); !ok {
		t.Fatal("no crate blueprint after placing it")
	}
	j.leaveArch()
	for i := 0; i < 3; i++ {
		if _, err := j.g.SpawnStack(item.Wood, 30, 8, 12+i); err != nil {
			t.Fatal(err)
		}
	}
	j.key("Digit3", 0)
	j.until(func() bool { _, ok := ax1Building(j, "crate", 14, 2); return ok }, 30000)
	if _, err := j.g.SpawnStack(item.Stone, 20, 8, 16); err != nil {
		t.Fatal(err)
	}
	held := func() int {
		n := 0
		for _, it := range j.world().Items {
			if !it.Carried && it.Store != 0 {
				n++
			}
		}
		return n
	}
	j.until(func() bool { return held() > 0 }, 30000)
	j.key("Space", 0)
	b, _ := ax1Building(j, "crate", 14, 2)
	id := b.ID
	j.lookAt(14, 2)
	j.update(1)
	for i := 0; i < 5 && !(j.a.sel.HasBuilding && j.a.sel.SelBuilding == id); i++ { // a stored stack lies on top: clicking again cycles down to the crate
		j.settle()
		j.clickTile(14, 2, inputframe.Left, 0)
	}
	if !j.a.sel.HasBuilding || j.a.sel.SelBuilding != id {
		t.Fatalf("clicking the crate selected building %d, want %d", j.a.sel.SelBuilding, id)
	}
	lines := wbPaneLines(j, id)
	if !strings.Contains(lines, "/12 stacks") || strings.Contains(lines, "Stored: 0/") {
		t.Fatalf("crate %d pane %q, want n/12 stacks with something in it", b.ID, lines)
	}
	if !j.clickID(inspector.StorageID) {
		t.Fatal("the crate has no Storage button")
	}
	j.update(2)
	if wbFind(j, strings.ToLower(windows.StorageModalID)) == "" || wbFind(j, "storage.filter.") == "" {
		t.Fatalf("the Storage button opened no filter window; ids %q", wbIDs(j))
	}
	j.draw(1)
}

// TestJourneyNeatAdmire: a colonist with free time stops at a display case
// that holds a stack and admires it; an empty case draws nobody.
func TestJourneyNeatAdmire(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(30, 24), jPawns([2]int{8, 8}))
	wdBuild(j, "display case", 14, 8)
	if _, err := j.g.SpawnStack(item.Gold, 5, 14, 8); err != nil {
		t.Fatal(err)
	}
	pid := j.g.Snapshot()[0].ID
	j.key("Digit3", 0)
	j.until(func() bool { return strings.Contains(j.pawn(uint64(pid)).Acting, "admire") }, 30000)
	j.key("Space", 0)
}
