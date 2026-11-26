// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/mapview"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func configBench(t *testing.T, g *Game, id entity.EntityID) mapview.BenchConfig {
	t.Helper()
	for _, b := range Configuration(g).Benches {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("no bench %d on map %d", id, g.ViewedMap())
	return mapview.BenchConfig{}
}

func TestNativeConfigurationBillEdits(t *testing.T) {
	g, _, bid := clubGame(t)
	b := configBench(t, g, bid)
	if b.LastEdit.Revision != 1 || b.Bills[0].Identity == 0 || b.WorkSpot != [2]int{6, 3} {
		t.Fatalf("unresolved native row or work spot: %+v", b)
	}
	row := b.Bills[0].Identity
	// Submit another bill on a non-origin footprint cell and update both in
	// one batch: the receipts must identify all seven real edits in order.
	add := command.Command{Kind: command.AddBill, X: 6, Y: 2, Recipe: uint8(recipe.Spear), Mode: uint8(recipe.DoCount), Count: 3}
	set := billCmd(command.SetBill, 0)
	set.Mode = uint8(recipe.UntilHave)
	set.Count = 8
	set.Enabled = true
	cfg := billCmd(command.ConfigureBill, 0)
	cfg.Radius = 7
	cfg.Store = uint8(recipe.DropOnFloor)
	ingredient := billCmd(command.SetBillIngredient, 0)
	ingredient.Stuff = uint8(item.Wood)
	source := billCmd(command.SetStorageSource, 0)
	source.Zone = 99
	reorder := billCmd(command.ReorderBill, 0)
	reorder.Count = 1
	remove := billCmd(command.RemoveBill, 0)
	for _, c := range []command.Command{add, set, cfg, ingredient, source, reorder, remove} {
		g.Enqueue(c)
	}
	g.Enqueue(billCmd(command.RemoveBill, 99))
	g.ApplyPending()
	v := Configuration(g)
	b = configBench(t, g, bid)
	if len(v.Mutations) != 7 || len(b.Bills) != 1 {
		t.Fatalf("edits %+v rows %+v", v.Mutations, b.Bills)
	}
	for i, m := range v.Mutations {
		if m.Map != v.Map || m.Owner != uint64(bid) || m.Edit.Tick != v.Tick || m.Edit.Revision != uint64(i+2) || m.Edit.Kind != m.Command.Kind {
			t.Fatalf("mutation %d: %+v", i, m)
		}
	}
	if v.Mutations[5].Edit.Row != row || v.Mutations[5].Edit.Index != 1 || v.Mutations[6].Edit.Row == row || v.Mutations[6].Edit.Row != v.Mutations[0].Edit.Row {
		t.Fatalf("unstable identities: %+v", v.Mutations)
	}
	bl := b.Bills[0]
	if bl.Identity != row || bl.Revision != 7 || bl.Mode != recipe.UntilHave || bl.Target != 8 || !bl.Suspended || bl.Radius != 7 || bl.Store != recipe.DropOnFloor || bl.Source != 99 || !slices.Contains(bl.Forbid, item.Wood) || b.LastEdit.Revision != 8 {
		t.Fatalf("edited settings lost: %+v", b)
	}
	// Detached reads cannot modify the actual bill, including its nested filter.
	b.Bills[0].Forbid[0] = item.Meat
	v.Mutations[0].Edit.Revision = 999
	if !slices.Contains(configBench(t, g, bid).Bills[0].Forbid, item.Wood) || Configuration(g).Mutations[0].Edit.Revision != 2 {
		t.Fatal("mutable alias")
	}
	h := saveLoad(t, g)
	if !reflect.DeepEqual(configBench(t, g, bid), configBench(t, h, bid)) || len(Configuration(h).Mutations) != 0 || g.StateHash() != h.StateHash() {
		t.Fatal("native settings/stamps not restored")
	}
	// Actual re-add after removal/reload must not reuse the old row's identity.
	applyNow(h, add)
	b = configBench(t, h, bid)
	if b.LastEdit.Revision != 9 || b.Bills[1].Identity == v.Mutations[6].Edit.Row {
		t.Fatalf("reused removed identity: %+v", b)
	}
	// Non-ingredient, absent bench, and an invalid row do not stamp an edit.
	bad := billCmd(command.SetBillIngredient, 0)
	bad.Stuff = uint8(item.Meat)
	h.Enqueue(bad)
	h.Enqueue(billCmd(command.SetBill, 20))
	h.Enqueue(command.Command{Kind: command.AddBill, X: 0, Y: 0, Recipe: uint8(recipe.Club)})
	h.ApplyPending()
	if b := configBench(t, h, bid); b.LastEdit.Revision != 9 || len(Configuration(h).Mutations) != 0 {
		t.Fatalf("invalid edit recorded: %+v", b)
	}
}

func TestNativeConfigurationUnknownAndWorkProgress(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	camp := buildNow(t, g, construction.Campfire, item.Wood, construction.South, 5, 2)
	h := saveLoad(t, g)
	for _, sim := range []*Game{g, h} {
		b := configBench(t, sim, camp)
		if len(b.Bills) != 1 || b.LastEdit.Revision != 0 || b.Bills[0].Identity != 0 || b.Bills[0].Revision != 0 {
			t.Fatalf("untouched/old default provenance invented: %+v", b)
		}
	}
	// Real production completes without being misclassified as a config edit.
	sim, ada, bid := clubGame(t)
	sim.SpawnStack(item.Wood, 5, 9, 6)
	before := configBench(t, sim, bid)
	runBillTicks(sim, ada, 6000, func() bool { return sim.craft.Bills.Of(bid)[0].Done == 1 })
	after := configBench(t, sim, bid)
	if after.Bills[0].Done != 1 || after.LastEdit != before.LastEdit || after.Bills[0].Revision != before.Bills[0].Revision {
		t.Fatalf("work completion changed config provenance: %+v", after)
	}
}

func TestNativeConfigurationPenTroughEdits(t *testing.T) {
	g, m := penWorld(t)
	cow := tameAt(t, g, "cow", "Bess", 24, 9)
	tr := buildNow(t, g, construction.FeedingTrough, item.Wood, construction.South, 22, 6)
	base := Configuration(g)
	if len(base.Pens) != 1 || base.Pens[0].LastEdit.Revision != 0 || !slices.Contains(base.Pens[0].Occupants, cow) || base.Troughs[0].LastEdit.Revision != 0 {
		t.Fatalf("unexpected initial configuration: %+v", base)
	}
	settings := PenSettings{Species: []PenSpecies{{Key: "cow", Allow: false}}, AutoCut: []string{"bramble"}}
	penEdit := command.Command{Kind: command.SetPenSettings, Target: uint64(m), Name: EncodePenSettings(settings)}
	troughEdit := command.Command{Kind: command.SetTroughFilter, Target: uint64(tr), Mode: TroughMeat}
	g.Enqueue(penEdit)
	g.Enqueue(troughEdit)
	g.Enqueue(command.Command{Kind: command.SetPenSettings, Target: uint64(m), Name: "invalid"})
	g.Enqueue(command.Command{Kind: command.SetTroughFilter, Target: uint64(tr), Mode: 99})
	g.Enqueue(command.Command{Kind: command.SetPenSettings, Target: uint64(tr), Name: penEdit.Name})
	g.Enqueue(command.Command{Kind: command.SetTroughFilter, Target: uint64(m), Mode: TroughAny})
	g.ApplyPending()
	v := Configuration(g)
	p, f := v.Pens[0], v.Troughs[0]
	if len(v.Mutations) != 2 || v.Mutations[0].Owner != uint64(m) || v.Mutations[1].Owner != uint64(tr) || p.LastEdit.Revision != 1 || f.LastEdit.Revision != 1 || f.Pen != m || f.Filter != TroughMeat || !f.Initialized || !reflect.DeepEqual(p.Settings, settings) || !slices.Contains(p.Occupants, cow) {
		t.Fatalf("settings or physical scope incorrect: %+v", v)
	}
	p.Settings.Species[0].Allow = true
	p.Settings.AutoCut[0] = "changed"
	p.Bits[0] = 123
	again := Configuration(g)
	if again.Pens[0].Settings.Species[0].Allow || again.Pens[0].Settings.AutoCut[0] != "bramble" || again.Pens[0].Bits[0] == 123 {
		t.Fatal("pen read is mutable alias")
	}
	// Finish existing lazy room/roof read caches on this freshly built fixture.
	g.World()
	h := saveLoad(t, g)
	again.Mutations = nil
	if got := Configuration(h); !reflect.DeepEqual(got, again) || h.StateHash() != g.StateHash() {

		t.Fatalf("pen/trough edit lost after save: got=%+v want=%+v equal=%v hash=%x/%x", got, again, reflect.DeepEqual(got, again), h.StateHash(), g.StateHash())
	}
	applyNow(h, troughEdit)
	if x := Configuration(h); x.Troughs[0].LastEdit.Revision != 2 || x.Pens[0].LastEdit.Revision != 1 {
		t.Fatalf("same-tick subsequent edit not recorded: %+v", x)
	}
}

func TestNativeConfigurationMapScope(t *testing.T) {
	g, _, first := clubGame(t)
	mid, ok := DevOpenMap(g, -1)
	if !ok {
		t.Fatal("second map")
	}
	g.ViewMap(mid)
	second := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 5, 2)
	// Coordinates overlap, but owner is resolved on the command's actual map.
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Club), Count: 4, Map: uint32(mid), MapSet: true})
	g.Enqueue(command.Command{Kind: command.SetBill, X: 5, Y: 2, Index: 0, Count: 3, Map: 0, MapSet: true})
	g.ApplyPending()
	v := Configuration(g)
	if v.Map != uint32(mid) || len(v.Benches) != 1 || v.Benches[0].ID != second || len(v.Mutations) != 1 || v.Mutations[0].Owner != uint64(second) {
		t.Fatalf("wrong map owner: %+v", v)
	}
	g.ViewMap(0)
	g.ApplyPending()
	v = Configuration(g)
	if v.Map != 0 || len(v.Benches) != 1 || v.Benches[0].ID != first || v.Benches[0].Bills[0].Target != 3 || len(v.Mutations) != 1 || v.Mutations[0].Owner != uint64(first) {
		t.Fatalf("cross-map edit: %+v", v)
	}
	h := saveLoad(t, g)
	if !reflect.DeepEqual(configBench(t, g, first), configBench(t, h, first)) {
		t.Fatal("primary edit lost")
	}
	h.ViewMap(mid)
	if b := configBench(t, h, second); b.LastEdit.Revision != 1 || b.Bills[0].Target != 4 {
		t.Fatalf("secondary edit lost: %+v", b)
	}
}
