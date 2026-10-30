package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// TestStoCbtCommandsInert (STO/CBT contract): the storage and combat
// commands route to no-ops, so a colony that gets every one of them reads
// and hashes as before, and keeps running in step with a twin that got
// none; the new view fields stay zero and none of the new kinds exists.
func TestStoCbtCommandsInert(t *testing.T) {
	t.Parallel()
	cfg := Config{Seed: 7, Width: 40, Height: 30}
	g, twin := NewColony(cfg), NewColony(cfg)
	g.Step(200)
	twin.Step(200)
	before := g.StateHash()
	pawn := uint64(g.Snapshot()[0].ID)
	var zoneID, bldg uint64
	if w := g.World(); len(w.Zones) > 0 {
		zoneID = uint64(w.Zones[0].ID)
	}
	if w := g.World(); len(w.Buildings) > 0 {
		bldg = uint64(w.Buildings[0].ID)
	}
	for _, c := range []command.Command{
		{Kind: command.HaulUrgently, X: 0, Y: 0, X2: cfg.Width - 1, Y2: cfg.Height - 1, Enabled: true},
		{Kind: command.DropWeapon, Pawn: pawn},
		{Kind: command.LinkStorage, Target: bldg, Zone: bldg},
		{Kind: command.UnlinkStorage, Target: bldg},
		{Kind: command.SetStorageSource, X: 1, Y: 1, Index: 0, Zone: zoneID},
		{Kind: command.SetZoneOverlay, Enabled: true},
	} {
		g.Enqueue(c)
	}
	g.ApplyPending()
	if g.StateHash() != before {
		t.Fatal("an inert contract command changed the state")
	}
	g.Step(300)
	twin.Step(300)
	if g.StateHash() != twin.StateHash() {
		t.Fatal("the commanded colony drifted from its twin")
	}
	for _, p := range g.Snapshot() {
		if p.WeaponQuality != 0 || p.WeaponStuff != "" || p.Kills != 0 || p.DamageDealt != 0 {
			t.Fatalf("%s: combat fields filled before CBT-3/CBT-5", p.Name)
		}
	}
	w := g.World()
	for _, it := range w.Items {
		if k, _ := item.ParseKind(it.Kind); k >= item.NutrientPasteMeal || it.Urgent {
			t.Fatalf("contract item state on the map: %+v", it)
		}
	}
	for _, b := range w.Buildings {
		if k, _ := construction.Parse(b.Kind); k >= construction.Hopper || b.Group != 0 || b.Stored != 0 || b.Capacity != 0 {
			t.Fatalf("contract building state on the map: %+v", b)
		}
	}
	for _, z := range w.Zones {
		if z.Group != 0 || z.Stored != 0 || z.Capacity != 0 {
			t.Fatalf("zone %d: storage readouts filled before STO-2", z.ID)
		}
	}
}

// TestStoCbtKindsDefined: the new buildings are valid kinds with wiki
// costs and hit points (TestStoCbtCommandsInert shows none is placed).
func TestStoCbtKindsDefined(t *testing.T) {
	t.Parallel()
	for _, k := range []construction.Kind{construction.Hopper, construction.NutrientPasteDispenser, construction.Sarcophagus} {
		if d := construction.DefOf(k); !k.Valid() || d.Wood <= 0 || d.Work <= 0 || construction.MaxHP(k, item.Steel) <= 0 {
			t.Errorf("%s: %+v", k, d)
		}
	}
}
