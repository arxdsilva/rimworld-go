package game

import (
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// DEF-4 mechanoids and crashed ship parts in the whole game (unit tests:
// raid/mech_test.go, raid/shippart_test.go).

func mechView(t *testing.T, g *Game, id entity.EntityID) AnimalView {
	t.Helper()
	for _, a := range g.World().Animals {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("mech %d not in the view", id)
	return AnimalView{}
}

// TestMechFightsAsRaider: a mechanoid shows as a hostile machine, never
// hungers or tires, and opens fire on the colonist with its own gun.
func TestMechFightsAsRaider(t *testing.T) {
	g := New(Config{Seed: 7, Width: 40, Height: 30})
	ada, _ := g.SpawnNamedPawn(25, 15, "Ada")
	id, err := g.SpawnAnimal("spindle", 8, 15)
	if err != nil {
		t.Fatal(err)
	}
	fired := false
	for i := 0; i < 900 && !fired; i++ {
		g.Step(1)
		v, _ := g.vital.Get(ada)
		fired = len(g.combat.Shots.Export()) > 0 || v.Injury > 0
	}
	if !fired {
		t.Fatal("the spindle never fired on Ada")
	}
	v := mechView(t, g, id)
	a, _ := g.animals.Get(id)
	if !v.Mech || !v.Hostile || v.Stunned || a.Food != animal.Full || a.Rest != animal.Full || a.Target != ada {
		t.Fatalf("view %+v state %+v", v, a)
	}
}

// TestMechEMPAndWreck: an EMP blast stuns a mechanoid for its window (the
// view says so); its plating turns blows; hit, it bleeds no stain; broken,
// it leaves a wreck that keeps, which no predator eats.
func TestMechEMPAndWreck(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 30})
	id, err := g.SpawnAnimal("harrow", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	emp := explosion.Def{Kind: explosion.EMP, Radius: 3500, Damage: 50}
	g.combat.Blasts.Explode(0, weapon.EMPGrenade, 10, 10, emp)
	if !mechView(t, g, id).Stunned {
		t.Fatal("the EMP did not stun the harrow")
	}
	g.Step(emp.StunTicks(0) + 1)
	if mechView(t, g, id).Stunned {
		t.Fatal("still stunned after the window")
	}
	cut := damage.Hit{Form: damage.FormCut, Amount: 10}
	if l := (combatHarm{g}).Absorb(id, cut, func(int) int { return 199 }); l.Outcome != armor.Deflected || l.Placed {
		t.Fatalf("plating (sharp 400) deflects under 200: %+v", l)
	}
	if l := (combatHarm{g}).Absorb(id, cut, func(int) int { return 400 }); l.Amount != 10 {
		t.Fatalf("and passes from 400: %+v", l)
	}
	if aim, _ := (combatSkills{g}).ShotAim(id); aim <= 950_000 {
		t.Fatalf("a mechanoid aims at 96 %% a tile, got %d ppm", aim)
	}
	p, _ := g.positions.Get(id)
	g.hurt(id, 10, 0)
	if a, _ := g.animals.Get(id); a.Body != nil || a.Injury != 10 || g.bleeding(id) || len(g.stains.tiles) != 0 {
		t.Fatalf("a machine takes damage, not wounds or stains: %+v stains %v", a, g.stains.tiles)
	}
	g.KillAnimal(id)
	if len(g.stains.tiles) != 0 {
		t.Fatalf("a wreck, no blood: %v", g.stains.tiles)
	}
	var wreck ItemView
	for _, it := range g.World().Items {
		if it.Animal == "harrow" {
			wreck = it
		}
	}
	if !wreck.Wreck || wreck.Kind != "harrow wreck" || wreck.X != p.X || wreck.Y != p.Y {
		t.Fatalf("wreck %+v", wreck)
	}
	wolf, _ := g.SpawnAnimal("timber_wolf", p.X+2, p.Y)
	w, _ := g.animals.Get(wolf)
	if _, ok := g.findFeed(&w, Position{X: p.X + 2, Y: p.Y}, animal.EcologyOf(animal.TimberWolf).Diet, func(int, int) bool { return true }); ok {
		t.Fatal("a wolf found a meal in a wreck")
	}
	g.Step(4 * clock.TicksPerDay)
	if meat, _ := animal.Yield(animal.Harrow); meat != 0 {
		t.Fatal("a wreck is butchered for no meat")
	}
	found := false
	for _, it := range g.World().Items {
		found = found || it.Wreck
	}
	if !found {
		t.Fatal("the wreck rotted away")
	}
}

// TestShipPartInGame: a crashed part is an ancient building with its
// wiki hit points and dormant guards; a colonist far off leaves them
// asleep, a shot at the hull wakes them, and broken it leaves its salvage
// and no blueprint; it saves with its guards.
func TestShipPartInGame(t *testing.T) {
	g := New(Config{Seed: 5, Width: 80, Height: 80})
	_, _ = g.SpawnNamedPawn(40, 40, "Ada")
	now := uint64(g.clock.Now())
	if !g.raid.Ships.Crash(800, now) {
		t.Fatal("no room for the part")
	}
	var part entity.EntityID
	var b construction.Building
	for _, bid := range g.bldgIDs {
		if bb, _ := g.bldgs.Get(bid); bb.Kind == construction.ShipPart {
			part, b = bid, bb
		}
	}
	if part == 0 || !b.Ancient || b.HP() != 1200 {
		t.Fatalf("part %d %+v", part, b)
	}
	dormant := func() (n, all int) {
		for _, a := range g.World().Animals {
			if a.Mech {
				all++
				if a.Dormant {
					n++
				}
			}
		}
		return n, all
	}
	if n, all := dormant(); n == 0 || n != all {
		t.Fatalf("%d of %d guards asleep", n, all)
	}
	h := saveLoad(t, g)
	if h.raid.Ships.Parts() != 1 {
		t.Fatal("the part did not survive the save")
	}
	g.Step(120)
	if n, all := dormant(); n != all {
		t.Fatalf("a far colonist woke %d of %d guards", all-n, all)
	}
	g.damageBuilding(part, 50)
	if n, _ := dormant(); n != 0 {
		t.Fatalf("a shot at the hull left %d guards asleep", n)
	}
	g.damageBuilding(part, 2000)
	if _, ok := g.bldgs.Get(part); ok || g.raid.Ships.Parts() != 0 || g.layerFrameAt(b.X, b.Y, construction.LayerObject) != 0 {
		t.Fatal("the broken part stands, or left a blueprint")
	}
	got := map[string]int{}
	for _, it := range g.World().Items {
		got[it.Kind] += it.Qty
	}
	if got[item.Steel.String()] != 60 || got[item.Plasteel.String()] != 10 {
		t.Fatalf("salvage %v", got)
	}
}

// TestShipPartNeedsPoints: below the wiki's 400 raid points the threat
// does not fit, and a bare colony has nowhere near that.
func TestShipPartNeedsPoints(t *testing.T) {
	g := New(Config{Seed: 2, Width: 40, Height: 30})
	_, _ = g.SpawnNamedPawn(20, 15, "Ada")
	if g.raidPoints(0) >= raid.ShipPartMinPoints {
		t.Skipf("a bare colony at %d points", g.raidPoints(0))
	}
	if g.FireIncident("crashed_ship_part") || g.raid.Ships.Parts() != 0 {
		t.Fatal("a ship part crashed on a poor colony")
	}
}
