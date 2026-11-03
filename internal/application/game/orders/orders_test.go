package orders

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

func TestFloorRefundWikiRule(t *testing.T) {
	if FloorRefund(construction.Flagstone) != 0 {
		t.Fatal("flagstone refunds nothing (wiki: Floors)")
	}
	for _, k := range []construction.Kind{construction.Floor, construction.SilverTile, construction.GoldTile} {
		if got, want := FloorRefund(k), construction.DefOf(k).Wood/2; got != want {
			t.Fatalf("%v refunds half, rounded down: got %d want %d", k, got, want)
		}
	}
	d := construction.DefOf(construction.SterileTile)
	for i, c := range d.Extra {
		if got := FloorExtraRefund(construction.SterileTile, i); got != c.N/2 {
			t.Fatalf("sterile tile extra %d refunds half: %d", i, got)
		}
	}
	if FloorExtraRefund(construction.Bed, 0) != 0 || FloorExtraRefund(construction.SterileTile, -1) != 0 {
		t.Fatal("only floors, only real slots")
	}
}

func TestChopWoodRule(t *testing.T) {
	g := plant.DefOf(plant.Tree).GrowTicks
	if Choppable(plant.Wild{Species: plant.Tree, Growth: g * 39 / 100}) {
		t.Fatal("a tree under 40 % is cut, not chopped")
	}
	if !Choppable(plant.Wild{Species: plant.Tree, Growth: g * 40 / 100}) {
		t.Fatal("a tree from 40 % is chopped")
	}
	if !Choppable(plant.Wild{Species: plant.Cactus, Growth: plant.DefOf(plant.Cactus).GrowTicks}) {
		t.Fatal("the saguaro counts as a tree")
	}
	if Choppable(plant.Wild{Species: plant.BerryBush, Growth: plant.DefOf(plant.BerryBush).GrowTicks}) {
		t.Fatal("a bush is never chopped")
	}
}

func TestSlaughterAndClaimRules(t *testing.T) {
	if Slaughterable(animal.State{}) || !Slaughterable(animal.State{Tame: true}) {
		t.Fatal("slaughter marks tame animals only")
	}
	if Claimable(construction.Building{Kind: construction.Bed}) {
		t.Fatal("the colony's own bed is not claimed")
	}
	if !Claimable(construction.Building{Kind: construction.Bed, Ruin: true}) || !Claimable(construction.Building{Kind: construction.Wall, Ancient: true}) {
		t.Fatal("ruins and raider or vault pieces are claimed")
	}
	if Claimable(construction.Building{Kind: construction.SteamVent, Ancient: true}) {
		t.Fatal("a natural feature is nobody's")
	}
	// MAP-13: a settlement's turret stays hostile; the rest of the compound is claimable
	if Claimable(construction.Building{Kind: construction.SentryTurret, Ancient: true, Hostile: true}) {
		t.Fatal("a settlement's turret is not claimed")
	}
	if !Claimable(construction.Building{Kind: construction.Wall, Ancient: true}) || !Claimable(construction.Building{Kind: construction.SentryTurret, Ancient: true}) {
		t.Fatal("a compound's other pieces stay claimable")
	}
}

func TestPackableAndCrate(t *testing.T) {
	bed := construction.Building{Kind: construction.Bed, Stuff: item.Wood, X: 3, Y: 4, Quality: 4, Damage: 9}
	if !Packable(bed) {
		t.Fatal("a bed packs")
	}
	for _, b := range []construction.Building{{Kind: construction.Wall}, {Kind: construction.Floor},
		{Kind: construction.Bed, Ruin: true}, {Kind: construction.SleeperCasket, Occupant: 1},
		{Kind: construction.SentryTurret, Hostile: true}, {Kind: construction.SentryTurret, Ancient: true, Hostile: true}} { // MAP-13
		if Packable(b) {
			t.Fatalf("%v must not pack", b.Kind)
		}
	}
	c := CrateOf(bed)
	if c.Kind != item.MinifiedBuilding || c.Qty != 1 || construction.Kind(c.Building) != construction.Bed ||
		c.Stuff != item.Wood || c.Quality != 4 || c.Wear != 9 || c.X != 3 || c.Y != 4 {
		t.Fatalf("crate %+v", c)
	}
	for k := construction.Kind(0); k.Valid(); k++ {
		if construction.Kind(uint8(k)) != k {
			t.Fatalf("construction kind %d does not fit Stack.Building", k)
		}
	}
	if InstallWork(construction.Bed, item.Wood) < MinInstallWork {
		t.Fatal("an install takes at least the minimum")
	}
}

func TestMarkSets(t *testing.T) {
	var s []entity.EntityID
	for _, id := range []entity.EntityID{5, 2, 9, 2} {
		s = setMark(s, id, true)
	}
	if len(s) != 3 || s[0] != 2 || s[2] != 9 || !marked(s, 5) {
		t.Fatalf("set %v", s)
	}
	s = setMark(s, 5, false)
	if marked(s, 5) || len(s) != 2 {
		t.Fatalf("unmark %v", s)
	}
	if got := sortedSet([]entity.EntityID{0, 4, 1, 4}); len(got) != 2 || got[0] != 1 {
		t.Fatalf("sortedSet %v", got)
	}
	o := New(nil)
	o.Import(Save{Strip: []entity.EntityID{7, 3}, Open: []entity.EntityID{8}})
	if ex := o.Export(); len(ex.Strip) != 2 || ex.Strip[0] != 3 || ex.Open[0] != 8 {
		t.Fatalf("export %+v", ex)
	}
}
