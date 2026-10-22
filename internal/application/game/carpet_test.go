package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
)

// carpetFrameAt places a carpet frame of the given carpet item.
func carpetFrameAt(g *Game, stuff item.Kind, x, y int) {
	g.placeFrameStuff(construction.Carpet, stuff, construction.South, x, y)
}

func TestStockpileTakesClothCarpets(t *testing.T) {
	t.Parallel()
	// a stockpile with every special filter off still takes plain goods
	plain := zone.Zone{Kind: zone.KindStockpile}
	for _, k := range append([]item.Kind{item.Cloth, item.Straw}, item.Carpets()...) {
		if !allows(plain, k) {
			t.Fatalf("stockpile refuses %s", k)
		}
	}
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 2, Y: 2, X2: 4, Y2: 4})
	g.Step(1)
	for _, k := range []item.Kind{item.Cloth, item.Straw, item.CarpetCloth, item.CarpetWood} {
		if !g.filterMatches(k) {
			t.Fatalf("no stockpile matches %s", k)
		}
		if !g.inMatchingStockpile(item.Stack{Kind: k, X: 3, Y: 3}) {
			t.Fatalf("%s at 3,3 is not stored", k)
		}
	}
}

func TestBuildablesListCarpetMaterials(t *testing.T) {
	t.Parallel()
	for _, b := range Buildables() {
		if len(b.Catches) != len(b.Stuffs) {
			t.Fatalf("%s: %d catches for %d stuffs", b.Kind, len(b.Catches), len(b.Stuffs))
		}
		if b.Kind != "carpet" {
			for _, c := range b.Catches {
				if c != 0 {
					t.Fatalf("%s catches dirt: %v", b.Kind, b.Catches)
				}
			}
			continue
		}
		if !b.Carpet || b.Wood != 1 || b.Stuffs[0] != "straw carpet" {
			t.Fatalf("carpet view = %+v", b)
		}
		want := map[string]int{"straw carpet": 45, "wood carpet": 30, "leather carpet": 55, "cloth carpet": 70}
		for i, s := range b.Stuffs {
			if b.Catches[i] != want[s] {
				t.Fatalf("%s catches %d%%, want %d%%", s, b.Catches[i], want[s])
			}
		}
		if len(b.Stuffs) != len(want) {
			t.Fatalf("stuffs = %v", b.Stuffs)
		}
	}
	for _, c := range item.Carpets() {
		if id, ok := StuffID(c.String()); !ok || item.Kind(id) != c {
			t.Fatalf("StuffID(%q) = %d %v", c.String(), id, ok)
		}
	}
	if _, ok := StuffID("cloth"); ok {
		t.Fatal("raw cloth is not a build material")
	}
}

func TestCarpetRecipesListed(t *testing.T) {
	t.Parallel()
	got := map[uint8]RecipeView{}
	for _, r := range Recipes() {
		got[r.ID] = r
	}
	for id, want := range map[recipe.ID][3]string{
		recipe.MakeStrawCarpet:  {"workbench", "10 straw", "straw carpet"},
		recipe.MakeWoodCarpet:   {"workbench", "6 wood", "wood carpet"},
		recipe.SewClothCarpet:   {"tailoring bench", "8 cloth", "cloth carpet"},
		recipe.SewLeatherCarpet: {"tailoring bench", "6 leather", "leather carpet"},
	} {
		r, ok := got[uint8(id)]
		if !ok || r.Bench != want[0] || r.Input != want[1] || r.Output != want[2] {
			t.Fatalf("recipe %d = %+v, want %v", id, r, want)
		}
	}
}

func TestCarpetOnFloorTile(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	placeBuilt(t, g, construction.Floor, construction.South, 5, 4)
	// no carpet item exists yet: the blueprint is still allowed, the frame waits
	if ok, r := g.CanPlaceAt("carpet", "straw carpet", "south", 5, 4); !ok {
		t.Fatalf("carpet on a floor: %q", r)
	}
	if ok, r := g.CanPlace("carpet", 5, 4); !ok {
		t.Fatalf("CanPlace carpet on a floor: %q", r)
	}
	if ok, r := g.CanPlaceStuff("carpet", "cloth carpet", 5, 4); !ok {
		t.Fatalf("CanPlaceStuff: %q", r)
	}
	carpetFrameAt(g, item.CarpetCloth, 5, 4)
	fid := g.layerFrameAt(5, 4, construction.LayerObject)
	fr, ok := g.frames.Get(fid)
	if !ok || fr.Kind != construction.Carpet || fr.Stuff != item.CarpetCloth || fr.Need != 1 || fr.Wood != 0 {
		t.Fatalf("frame = %+v ok=%v", fr, ok)
	}
	if g.layerBldgAt(5, 4, construction.LayerFloor) == 0 {
		t.Fatal("the floor stays under the carpet frame")
	}
	g.completeFrame(fid, fr)
	if g.layerBldgAt(5, 4, construction.LayerFloor) == 0 || g.layerBldgAt(5, 4, construction.LayerObject) == 0 {
		t.Fatal("floor and carpet share the tile")
	}
	if !g.grid.Walkable(5, 4) {
		t.Fatal("a carpet is walkable")
	}
	if ok, r := g.CanPlace("carpet", 5, 4); ok || r != ReasonOccupied {
		t.Fatalf("second carpet on the tile: %v %q", ok, r)
	}
	// ordinary ground works too, wrong material does not
	if ok, _ := g.CanPlaceAt("carpet", "straw carpet", "south", 7, 4); !ok {
		t.Fatal("carpet on bare ground")
	}
	if ok, r := g.CanPlaceAt("carpet", "wood", "south", 7, 4); ok || r != ReasonStuff {
		t.Fatalf("carpet from raw wood: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("wall", "straw carpet", "south", 7, 4); ok || r != ReasonStuff {
		t.Fatalf("wall from carpet: %v %q", ok, r)
	}
}

func TestCarpetNotOnDoor(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	placeBuilt(t, g, construction.Door, construction.South, 5, 4)
	if ok, r := g.CanPlaceAt("carpet", "straw carpet", "south", 5, 4); ok || r != ReasonOccupied {
		t.Fatalf("carpet on a door: %v %q", ok, r)
	}
	g.placeFrameStuff(construction.Door, item.Wood, construction.South, 7, 4)
	if ok, r := g.CanPlaceAt("carpet", "straw carpet", "south", 7, 4); ok || r != ReasonPlanned {
		t.Fatalf("carpet on a planned door: %v %q", ok, r)
	}
	placeBuilt(t, g, construction.Wall, construction.South, 9, 4)
	if ok, r := g.CanPlaceAt("carpet", "straw carpet", "south", 9, 4); ok || r != ReasonOccupied {
		t.Fatalf("carpet in a wall: %v %q", ok, r)
	}
	// the tile beside the door is fine
	if ok, r := g.CanPlaceAt("carpet", "straw carpet", "south", 5, 5); !ok {
		t.Fatalf("carpet beside a door: %q", r)
	}
}

func TestCarpetRemovalRefundsItsOwnItem(t *testing.T) {
	t.Parallel()
	for _, c := range item.Carpets() {
		g := New(Config{Seed: 1, Width: 8, Height: 6})
		carpetFrameAt(g, c, 3, 3)
		fid := g.layerFrameAt(3, 3, construction.LayerObject)
		fr, _ := g.frames.Get(fid)
		g.completeFrame(fid, fr)
		g.removeBuilding(fid)
		st, ok := stackOf(g, c)
		if !ok || st.Qty != 1 || st.X != 3 || st.Y != 3 {
			t.Fatalf("%s refund = %+v %v", c, st, ok)
		}
		if _, wood := stackOf(g, item.Wood); wood {
			t.Fatalf("%s refunded wood", c)
		}
	}
}

// Craft a straw carpet at a workbench, hang it as a blueprint, watch a
// colonist haul it into the frame and build it, then take it back down.
func TestCarpetCraftHaulInstallDeconstruct(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(item.Straw, 10, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.MakeStrawCarpet), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	run := func(done func() bool, what string) {
		t.Helper()
		for i := 0; i < 400 && !done(); i++ {
			g.SetNeeds(id, needs.Full, needs.Full)
			g.Step(TicksPerCell * 10)
		}
		if !done() {
			t.Fatalf("timed out: %s", what)
		}
	}
	run(func() bool { return g.stockOf(item.CarpetStraw) == 1 }, "straw carpet crafted")
	if g.stockOf(item.Straw) != 0 {
		t.Fatalf("straw left = %d, want 0 (10 consumed)", g.stockOf(item.Straw))
	}
	carpetFrameAt(g, item.CarpetStraw, 9, 5)
	if _, ok := g.frames.Get(g.layerFrameAt(9, 5, construction.LayerObject)); !ok {
		t.Fatal("no carpet frame")
	}
	run(func() bool { return g.layerBldgAt(9, 5, construction.LayerObject) != 0 }, "carpet built")
	if g.stockOf(item.CarpetStraw) != 0 {
		t.Fatalf("the item should be in the carpet now, stock=%d", g.stockOf(item.CarpetStraw))
	}
	var catch int
	for _, b := range g.World().Buildings {
		if b.Kind == "carpet" {
			catch = b.Catch
			if b.Stuff != "straw carpet" {
				t.Fatalf("stuff = %q", b.Stuff)
			}
		}
	}
	if catch != 45 {
		t.Fatalf("catch = %d, want 45", catch)
	}
	g.Enqueue(command.Command{Kind: command.DesignateDeconstruct, X: 9, Y: 5, X2: 9, Y2: 5})
	run(func() bool { return g.layerBldgAt(9, 5, construction.LayerObject) == 0 }, "carpet deconstructed")
	run(func() bool { return g.stockOf(item.CarpetStraw) == 1 }, "carpet item returned")
	if _, wood := stackOf(g, item.Wood); wood {
		t.Fatal("deconstructing a carpet must not give wood")
	}
}
