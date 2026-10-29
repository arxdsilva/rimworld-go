package ebiten

import (
	inspectorpkg "rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// findButton finds id among what the selected colonist offers: its command
// buttons (the label carries the live key, as the grid's badge does), the
// pane's tabs and its chips.
func findButton(a *App, id string) (uikit.Button, bool) {
	p, ok := a.insp.SelectedPawn()
	if !ok {
		return uikit.Button{}, false
	}
	for _, c := range a.insp.Pawn().PawnCommands(p.ID) {
		if c.ID == id {
			return uikit.Button{ID: c.ID, Label: c.Label + a.keyHint(c.Hotkey), Disabled: c.Disabled, Reason: c.Reason, Tip: c.Desc, Selected: c.On}, true
		}
	}
	spec := a.insp.Pawn().PawnPane(p.ID)
	for _, tb := range spec.Tabs {
		if tb.ID == id {
			return uikit.Button{ID: tb.ID, Label: tb.Label, Disabled: tb.Disabled, Reason: tb.Reason,
				Selected: a.insp.Tab == strings.TrimPrefix(tb.ID, "inspector.")}, true
		}
	}
	for _, c := range spec.Chips {
		if c.ID == id {
			return uikit.Button{ID: c.ID, Label: c.Label, Disabled: c.Disabled, Reason: c.Reason, Tip: c.Tip}, true
		}
	}
	return uikit.Button{}, false
}

func selectPawn(t *testing.T, a *App, id entity.EntityID) {
	t.Helper()
	click(t, a, barID(id))
	a.renderer.speed = speedPaused
	if !a.renderer.HasSelect() {
		t.Fatal("pawn not selected")
	}
}

// equipOptions are the options of the selected colonist's Equip command.
func equipOptions(t *testing.T, a *App) []inspectorpkg.Option {
	t.Helper()
	b, ok := findCmd(a, "cmd.pawn.equip")
	if !ok || b.Options == nil {
		t.Fatalf("no Equip command with options: %+v ok=%v", b, ok)
	}
	return b.Options()
}

// findCmd finds a command button of the selected colonist.
func findCmd(a *App, id string) (inspectorpkg.Button, bool) {
	p, ok := a.insp.SelectedPawn()
	if !ok {
		return inspectorpkg.Button{}, false
	}
	return cmdByID(a.insp.Pawn().PawnCommands(p.ID), id)
}

// Draft and Clear orders are commands whatever tab is open; the legacy
// inspector panel no longer has an action bar.
func TestPawnActionBar(t *testing.T) {
	a, _, id := liveColony(t, 0)
	selectPawn(t, a, id)
	for _, tab := range []string{"character", "needs", "gear"} {
		a.insp.Tab = tab
		for _, want := range []string{"pawn.draft", "pawn.clearorders"} {
			if _, ok := findButton(a, want); !ok {
				t.Fatalf("tab %s: no %s command", tab, want)
			}
		}
	}
	assertButtonsInRegions(t, a) // the legacy panel has no action bar any more
}

func TestClearOrdersDisabledWhenEmpty(t *testing.T) {
	a, g, id := liveColony(t, 0)
	selectPawn(t, a, id)
	b, _ := findButton(a, "pawn.clearorders")
	if !b.Disabled || b.Reason != "no queued orders" {
		t.Fatalf("empty queue: %+v", b)
	}
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 8, Y: 8})
	g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(id), X: 9, Y: 9, Queue: true})
	g.ApplyPending()
	b, _ = findButton(a, "pawn.clearorders")
	if b.Disabled {
		t.Fatalf("queued order should enable Clear orders: %+v", b)
	}
	a.applyClick("pawn.clearorders")
	b, _ = findButton(a, "pawn.clearorders")
	if !b.Disabled {
		t.Fatalf("queue should be empty after clearing: %+v", b)
	}
}

func TestEquipNearbyListsReachableWeapons(t *testing.T) {
	a, g, id := liveColony(t, 0)
	near, _ := g.SpawnStack(item.Rifle, 1, 5, 2)
	g.SpawnStack(item.Rifle, 1, 12, 2)
	selectPawn(t, a, id)
	opts := equipOptions(t, a)
	if len(opts) != 2 {
		t.Fatalf("options = %+v, want the near and the far rifle", opts)
	}
	n := opts[0]
	if !strings.Contains(n.Label, "3 tiles") || !strings.Contains(strings.ToLower(n.Label), "rifle") || !strings.HasPrefix(n.Label, "Equip") {
		t.Fatalf("near rifle option = %+v", n)
	}
	if n.Cmd == nil || n.Cmd.Order != command.OrderEquip {
		t.Fatalf("near rifle option carries the OrderEquip command: %+v", n.Cmd)
	}
	if !strings.Contains(opts[1].Label, "10 tiles") {
		t.Fatalf("far rifle should be listed: %+v", opts[1])
	}
	// the Gear tab no longer lists weapons: the Equip command does
	a.insp.Tab = "gear"
	if _, ok := findButton(a, "pawn.equip."+eid(near)); ok {
		t.Fatal("equip rows moved to the Equip command's options")
	}
}

// Garments lying about are offered too (Wear is an OrderEquip).
func TestEquipNearbyOffersGarments(t *testing.T) {
	a, g, id := liveColony(t, 0)
	if _, err := g.SpawnStack(item.Parka, 1, 4, 2); err != nil {
		t.Fatal(err)
	}
	selectPawn(t, a, id)
	opts := equipOptions(t, a)
	if len(opts) != 1 || !strings.HasPrefix(opts[0].Label, "Wear") || !strings.Contains(opts[0].Label, "2 tiles") {
		t.Fatalf("options = %+v, want one Wear parka option", opts)
	}
	if opts[0].Cmd == nil || opts[0].Cmd.Order != command.OrderEquip {
		t.Fatalf("Wear is an OrderEquip: %+v", opts[0].Cmd)
	}
}

func TestEquipNearbyNoWeapons(t *testing.T) {
	a, _, id := liveColony(t, 0)
	selectPawn(t, a, id)
	b, ok := findButton(a, "cmd.pawn.equip")
	if !ok || !b.Disabled || b.Reason != "nothing to equip in reach" {
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
	if n := len(equipOptions(t, a)); n != 4 {
		t.Fatalf("equip options = %d, want 4", n)
	}
}

func TestEquipNearbyEnqueuesOrder(t *testing.T) {
	a, g, id := liveColony(t, 0)
	g.SpawnStack(item.Rifle, 1, 6, 2)
	selectPawn(t, a, id)
	opts := equipOptions(t, a)
	if len(opts) != 1 || opts[0].Cmd == nil {
		t.Fatalf("options = %+v", opts)
	}
	a.renderer.enqueue(*opts[0].Cmd) // what picking the option does
	g.Step(1)
	p, _ := a.insp.SelectedPawn()
	if p.Job == "" || !strings.Contains(strings.ToLower(p.Job), "equip") {
		t.Fatalf("equip order not applied: job=%q activity=%q queued=%v", p.Job, p.Activity, p.QueuedOrders)
	}
}

func TestDeadPawnActionsDisabled(t *testing.T) {
	a, g, id := liveColony(t, 1)
	g.SpawnStack(item.Rifle, 1, 5, 2)
	selectPawn(t, a, id)
	a.insp.Tab = "gear"
	g.Injure(id, 100000)
	g.ApplyPending()
	p, _ := a.insp.SelectedPawn()
	if p.Status == "alive" {
		t.Skipf("pawn survived: %s", p.Status)
	}
	for _, c := range a.insp.Pawn().PawnCommands(p.ID) {
		if c.ID == "pawn.draft" || c.ID == "pawn.clearorders" || c.ID == "cmd.pawn.equip" {
			if !c.Disabled || c.Reason != p.Status {
				t.Fatalf("%s: disabled=%v reason=%q, want reason %q", c.ID, c.Disabled, c.Reason, p.Status)
			}
		}
	}
}

func eid(id entity.EntityID) string { return strconv.FormatUint(uint64(id), 10) }
