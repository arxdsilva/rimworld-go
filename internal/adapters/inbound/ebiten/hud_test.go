package ebiten

import (
	"fmt"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

func liveColony(t *testing.T, extraPawns int) (*App, *game.Game, entity.EntityID) {
	t.Helper()
	g := game.New(game.Config{Seed: 1, Width: 16, Height: 16})
	id, err := g.SpawnPawn(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < extraPawns; i++ {
		if _, err := g.SpawnPawn(3+i, 2); err != nil {
			t.Fatal(err)
		}
	}
	a := testApp(g)
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != screenColony || a.renderer == nil {
		t.Fatalf("startGame = %s renderer=%v", a.nav.screen(), a.renderer != nil)
	}
	return a, g, id
}

func barID(id entity.EntityID) string {
	return fmt.Sprintf("bar.pawn.%d", id)
}

func TestColonistBarClickSelects(t *testing.T) {
	t.Parallel()
	a, _, id := liveColony(t, 1)
	click(t, a, barID(id))
	if !a.renderer.HasSelect() || a.renderer.Selected() != id {
		t.Fatalf("select = %v/%d, want %d", a.renderer.HasSelect(), a.renderer.Selected(), id)
	}
	if len(a.inspectorButtons()) == 0 {
		t.Fatal("inspector should be visible after bar select")
	}
}

func TestWorkTabOpensOverlay(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	click(t, a, "tab.work")
	if !a.workOpen {
		t.Fatal("Work tab should open the overlay")
	}
	found := false
	for _, b := range a.hudTabButtons(a.sw, a.sh) {
		if b.id == "tab.work" && b.selected {
			found = true
		}
	}
	if !found {
		t.Fatal("Work tab should be selected while overlay is open")
	}
}

func TestDisabledTabsDoNotNavigate(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	click(t, a, "tab.architect")
	if a.workOpen {
		t.Fatal("disabled Architect tab opened Work")
	}
	if a.nav.screen() != screenColony || a.nav.overlay() != overlayNone {
		t.Fatalf("disabled tab navigated to %s/%s", a.nav.screen(), a.nav.overlay())
	}
	if a.hint != "not implemented" {
		t.Fatalf("hint = %q, want not implemented", a.hint)
	}
}

func TestMenuTabOpensPause(t *testing.T) {
	t.Parallel()
	a, _, _ := liveColony(t, 0)
	click(t, a, "tab.menu")
	if a.nav.overlay() != overlayPause {
		t.Fatalf("overlay = %s, want pause", a.nav.overlay())
	}
}

func TestEscapeClosesWorkThenInspectorThenPause(t *testing.T) {
	t.Parallel()
	a, _, id := liveColony(t, 0)
	a.renderer.Select(id)
	click(t, a, "tab.work")

	a.pressEscape()
	if a.workOpen {
		t.Fatal("first Escape should close Work")
	}
	if a.nav.overlay() != overlayNone {
		t.Fatal("first Escape should not pause")
	}
	if !a.renderer.HasSelect() {
		t.Fatal("first Escape should keep inspector selection")
	}

	a.pressEscape()
	if a.renderer.HasSelect() {
		t.Fatal("second Escape should clear inspector")
	}
	if a.nav.overlay() != overlayNone {
		t.Fatal("second Escape should not pause")
	}

	a.pressEscape()
	if a.nav.overlay() != overlayPause {
		t.Fatalf("third Escape overlay = %s, want pause", a.nav.overlay())
	}
}

func TestWorkCellsDoNotStep(t *testing.T) {
	t.Parallel()
	a, g, _ := liveColony(t, 0)
	before := g.Tick()
	click(t, a, "tab.work")
	if g.Tick() != before {
		t.Fatalf("opening Work stepped %d -> %d", before, g.Tick())
	}
	for _, b := range a.layoutButtons(a.sw, a.sh) {
		if len(b.id) >= 4 && b.id[:4] == "work" && b.id != "tab.work" {
			t.Fatalf("Work cells must not be buttons, got %q", b.id)
		}
	}
	a.tick()
	if g.Tick() != before+1 {
		t.Fatalf("one colony tick after Work open = %d, want %d", g.Tick(), before+1)
	}
}

func TestColonyChromeHitBeforeMap(t *testing.T) {
	t.Parallel()
	a, _, id := liveColony(t, 0)
	sw, sh := a.sw, a.sh
	if !a.colonyChromeHit(10, 10, sw, sh) {
		t.Fatal("top resource/time strip should consume hits")
	}
	if !a.colonyChromeHit(10, sh-8, sw, sh) {
		t.Fatal("bottom tabs should consume hits")
	}
	if !a.colonyChromeHit(hudPad+8, int(hudBarY(sh))+8, sw, sh) {
		t.Fatal("colonist bar should consume hits")
	}
	if a.colonyChromeHit(sw/2, sh/2, sw, sh) {
		t.Fatal("map center should pass through to selection")
	}

	a.renderer.Select(id)
	ir := inspectorRect()
	if !a.colonyChromeHit(int(ir.x)+20, int(ir.y)+20, sw, sh) {
		t.Fatal("inspector window should consume hits")
	}

	click(t, a, "tab.work")
	wr := workRect(sw, sh)
	if !a.colonyChromeHit(int(wr.x)+20, int(wr.y)+40, sw, sh) {
		t.Fatal("Work overlay should consume hits")
	}
}

func TestInspectorCloseClearsSelect(t *testing.T) {
	t.Parallel()
	a, _, id := liveColony(t, 0)
	a.renderer.Select(id)
	click(t, a, "inspector.close")
	if a.renderer.HasSelect() {
		t.Fatal("inspector close should clear selection")
	}
}

func TestRendererSelectAPI(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 1, Width: 8, Height: 8})
	id, err := g.SpawnPawn(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	r := New(g)
	if r.HasSelect() {
		t.Fatal("new renderer should have no selection")
	}
	r.Select(id)
	if !r.HasSelect() || r.Selected() != id {
		t.Fatalf("Select = %v/%d, want %d", r.HasSelect(), r.Selected(), id)
	}
	r.ClearSelect()
	if r.HasSelect() {
		t.Fatal("ClearSelect should drop selection")
	}
}
