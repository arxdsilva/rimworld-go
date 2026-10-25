package ebiten

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestInspectorShowsWaitingFor(t *testing.T) {
	names := map[entity.EntityID]string{7: "Dustbuck"}
	lookup := func(id entity.EntityID) string { return names[id] }
	if got := waitingForLine(7, lookup); got != "waiting for Dustbuck" {
		t.Fatalf("got %q", got)
	}
	if got := waitingForLine(0, lookup); got != "" {
		t.Fatalf("unblocked should show nothing, got %q", got)
	}
	if got := waitingForLine(9, lookup); got != "waiting for something" {
		t.Fatalf("unknown blocker got %q", got)
	}
}

func findButton(a *App, id string) (button, bool) {
	for _, b := range a.inspectorButtons() {
		if b.id == id {
			return b, true
		}
	}
	return button{}, false
}

func selectPawn(t *testing.T, a *App, id entity.EntityID) {
	t.Helper()
	click(t, a, barID(id))
	a.renderer.speed = speedPaused
	if !a.renderer.HasSelect() {
		t.Fatal("pawn not selected")
	}
}

// The action bar shows Draft and Clear orders on every tab.
func TestPawnActionBar(t *testing.T) {
	a, _, id := liveColony(t, 0)
	selectPawn(t, a, id)
	for _, tab := range []string{"character", "needs", "gear"} {
		a.inspectTab = tab
		for _, want := range []string{"pawn.draft", "pawn.clearorders"} {
			b, ok := findButton(a, want)
			if !ok {
				t.Fatalf("tab %s: no %s button", tab, want)
			}
			if b.y < inspectorRect().y+inspectorRect().h-70 {
				t.Fatalf("tab %s: %s not on the bottom bar (y=%v)", tab, want, b.y)
			}
		}
	}
}

func TestClearOrdersDisabledWhenEmpty(t *testing.T) {
	a, g, id := liveColony(t, 0)
	selectPawn(t, a, id)
	b, _ := findButton(a, "pawn.clearorders")
	if !b.disabled || b.reason != "no queued orders" {
		t.Fatalf("empty queue: %+v", b)
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 8, Y: 8})
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 9, Y: 9, Queue: true})
	g.ApplyPending()
	b, _ = findButton(a, "pawn.clearorders")
	if b.disabled {
		t.Fatalf("queued order should enable Clear orders: %+v", b)
	}
	a.applyClick("pawn.clearorders")
	b, _ = findButton(a, "pawn.clearorders")
	if !b.disabled {
		t.Fatalf("queue should be empty after clearing: %+v", b)
	}
}

func TestEquipNearbyListsReachableWeapons(t *testing.T) {
	a, g, id := liveColony(t, 0)
	near, _ := g.SpawnStack(item.Rifle, 1, 5, 2)
	far, _ := g.SpawnStack(item.Rifle, 1, 12, 2)
	selectPawn(t, a, id)
	a.inspectTab = "gear"
	n, ok := findButton(a, "pawn.equip."+eid(near))
	if !ok || !strings.Contains(n.label, "3 tiles") || !strings.HasPrefix(n.label, "rifle") && !strings.HasPrefix(n.label, "Rifle") {
		t.Fatalf("near rifle button = %+v ok=%v", n, ok)
	}
	if _, ok := findButton(a, "pawn.equip."+eid(far)); !ok {
		t.Fatal("far rifle should be listed")
	}
	// Other tabs do not list weapons.
	a.inspectTab = "needs"
	if _, ok := findButton(a, "pawn.equip."+eid(near)); ok {
		t.Fatal("equip list belongs to the gear tab")
	}
}

func TestEquipNearbyNoWeapons(t *testing.T) {
	a, _, id := liveColony(t, 0)
	selectPawn(t, a, id)
	a.inspectTab = "gear"
	b, ok := findButton(a, "pawn.equip.none")
	if !ok || !b.disabled || b.reason != "no weapons in reach" {
		t.Fatalf("empty list = %+v ok=%v", b, ok)
	}
}

func TestEquipNearbyCapsAtFour(t *testing.T) {
	a, g, id := liveColony(t, 0)
	for x := 4; x < 12; x++ {
		if _, err := g.SpawnStack(item.Rifle, 1, x, 4); err != nil {
			t.Fatal(err)
		}
	}
	selectPawn(t, a, id)
	a.inspectTab = "gear"
	n := 0
	for _, b := range a.inspectorButtons() {
		if strings.HasPrefix(b.id, "pawn.equip.") {
			n++
		}
	}
	if n != 4 {
		t.Fatalf("equip buttons = %d, want 4", n)
	}
}

func TestEquipNearbyEnqueuesOrder(t *testing.T) {
	a, g, id := liveColony(t, 0)
	rid, _ := g.SpawnStack(item.Rifle, 1, 6, 2)
	selectPawn(t, a, id)
	a.inspectTab = "gear"
	if _, ok := findButton(a, "pawn.equip."+eid(rid)); !ok {
		t.Fatal("no equip button")
	}
	a.applyPawnEquipClick("pawn.equip." + eid(rid)) // routed from applyClick by the lead
	g.Step(1)
	p, _ := a.selectedPawn()
	if p.Job == "" || !strings.Contains(strings.ToLower(p.Job), "equip") {
		t.Fatalf("equip order not applied: job=%q activity=%q queued=%v", p.Job, p.Activity, p.QueuedOrders)
	}
}

func TestDeadPawnActionsDisabled(t *testing.T) {
	a, g, id := liveColony(t, 1)
	g.SpawnStack(item.Rifle, 1, 5, 2)
	selectPawn(t, a, id)
	a.inspectTab = "gear"
	g.Injure(id, 100000)
	g.ApplyPending()
	p, _ := a.selectedPawn()
	if p.Status == "alive" {
		t.Skipf("pawn survived: %s", p.Status)
	}
	for _, b := range a.inspectorButtons() {
		if b.id == "pawn.draft" || b.id == "pawn.clearorders" || strings.HasPrefix(b.id, "pawn.equip.") {
			if !b.disabled || b.reason != p.Status {
				t.Fatalf("%s: disabled=%v reason=%q, want reason %q", b.id, b.disabled, b.reason, p.Status)
			}
		}
	}
}

func eid(id entity.EntityID) string { return strconv.FormatUint(uint64(id), 10) }

func TestPostponedLine(t *testing.T) {
	if postponedLine(nil) != "" {
		t.Fatal("nothing put off: no line")
	}
	got := postponedLine([]string{"build wall: blocked, retry in 12 s", "haul wood: can't reach"})
	if got != "put off: build wall: blocked, retry in 12 s (+1)" {
		t.Fatalf("line %q", got)
	}
}

// PLAY-017: the Gear tab marks the garments the player chose and names the
// one the colonist is walking to put on; the layout below follows.
func TestGearRows(t *testing.T) {
	p := game.PawnView{Apparel: []string{"parka", "tuque"}, ApparelForced: []bool{true, false}, ChangingInto: "duster"}
	rows := gearRows(p)
	want := []string{"everyday clothes", "leather parka  (you chose this)", "knitted tuque  (warmer)", "Changing into: duster"}
	if !slices.Equal(rows, want) {
		t.Fatalf("rows %q\nwant %q", rows, want)
	}
	if got := gearRows(game.PawnView{}); !slices.Equal(got, []string{"everyday clothes"}) {
		t.Fatalf("bare colonist rows %q", got)
	}
	r := inspectorRect()
	if got, want := equipListY(r, p), r.y+insBodyY+gearLayoutFor(len(rows)-1).equipList; got != want {
		t.Fatalf("Equip nearby at %v, want %v (below every row)", got, want)
	}
}
