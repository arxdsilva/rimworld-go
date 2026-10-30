package game

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/terrain"
)

func TestSapperDigClearsOre(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	if !g.SetGround(4, 4, 5, 4, "rock", "granite", "gold") || !g.SetGround(4, 6, 4, 6, "rock_smooth", "marble", "") {
		t.Fatal("set ground")
	}
	g.raid.Rock.Dig(4, 4, raid.RockHP)
	if g.TerrainAt(4, 4) != "stone" || g.geo.OreAt(4, 4) != geology.NoOre || g.stoneAt(4, 4) != geology.Granite {
		t.Fatalf("dug ore = %s ore %v stone %v", g.TerrainAt(4, 4), g.geo.OreAt(4, 4), g.stoneAt(4, 4))
	}
	if g.geo.OreAt(5, 4) != geology.Gold {
		t.Fatal("the neighbouring ore stays")
	}
	g.raid.Rock.Dig(4, 6, raid.RockHP)
	if g.TerrainAt(4, 6) != "stone" || !g.grid.Walkable(4, 6) {
		t.Fatalf("sappers dig smoothed rock too: %s", g.TerrainAt(4, 6))
	}
	// any change of ground clears ore: it only lives in rock
	g.setTerrain(5, 4, terrain.Soil)
	if g.geo.OreAt(5, 4) != geology.NoOre {
		t.Fatal("ore left under soil")
	}
}

func TestStructVerBumps(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := postWorker(t, g, 1, 1, "Ada", work.Construct)
	check := func(what string, f func()) {
		t.Helper()
		v := g.World().StructVer
		f()
		if w := g.World().StructVer; w == v || w != g.structVer {
			t.Fatalf("%s: struct version %d -> %d", what, v, w)
		}
	}
	check("terrain", func() { g.setTerrain(9, 7, terrain.Rock) })
	check("build", func() { g.BuildNow("wall", "wood", "south", 5, 5) })
	check("deconstruct", func() {
		applyNow(g, command.Command{Kind: command.DesignateDeconstruct, X: 5, Y: 5, X2: 5, Y2: 5})
		stepFed(g, TicksPerCell*20+200, id)
		if _, ok := building(g, 5, 5, "wall"); ok {
			t.Fatal("wall not deconstructed")
		}
	})
	check("destroyed", func() {
		g.BuildNow("wall", "wood", "south", 7, 5)
		v := g.World().StructVer
		g.damageBuilding(g.layerBldgAt(7, 5, construction.LayerObject), 1<<20)
		if g.World().StructVer == v {
			t.Fatal("destroying a wall keeps the version")
		}
	})
	v := g.World().StructVer
	stepFed(g, 30, id)
	if g.World().StructVer != v {
		t.Fatal("the version moves with nothing changed")
	}
}

func TestWorldViewStoneLayersShared(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 8, Height: 6})
	if !g.SetGround(2, 1, 3, 1, "rock", "slate", "steel") || !g.SetGround(2, 2, 2, 2, "stone", "", "") {
		t.Fatal("set ground")
	}
	a, b := g.World(), g.World()
	if len(a.StoneType) != 8*6 || len(a.Ore) != 8*6 {
		t.Fatalf("layer sizes %d %d", len(a.StoneType), len(a.Ore))
	}
	if &a.StoneType[0] != &b.StoneType[0] || &a.Ore[0] != &b.Ore[0] {
		t.Fatal("unchanged layers must be shared between views")
	}
	names, ores := StoneNames(), OreNames()
	at := func(w WorldView, x, y int) (string, string) {
		return names[w.StoneType[y*w.TerrainW+x]], ores[w.Ore[y*w.TerrainW+x]]
	}
	if s, o := at(a, 2, 1); s != "slate" || o != "steel" {
		t.Fatalf("2,1 = %s %s", s, o)
	}
	if s, _ := at(a, 2, 2); s != "granite" {
		t.Fatalf("stone ground without geology reads as granite, got %q", s)
	}
	if s, o := at(a, 0, 0); s != "" || o != "" {
		t.Fatalf("soil = %q %q", s, o)
	}
	g.setTerrain(2, 1, terrain.Stone) // mined: ore gone, stone stays
	c := g.World()
	if &c.Ore[0] == &a.Ore[0] {
		t.Fatal("a change must rebuild, not overwrite, the shared layers")
	}
	if s, o := at(c, 2, 1); s != "slate" || o != "" {
		t.Fatalf("after mining 2,1 = %s %s", s, o)
	}
	if s, o := at(a, 2, 1); s != "slate" || o != "steel" {
		t.Fatal("an older view changed under its reader")
	}
}

func TestWorldGeologyDeterministic(t *testing.T) {
	t.Parallel()
	a := NewColony(Config{Seed: 5, Width: 48, Height: 40}).World()
	b := NewColony(Config{Seed: 5, Width: 48, Height: 40}).World()
	if !reflect.DeepEqual(a.StoneType, b.StoneType) || !reflect.DeepEqual(a.Ore, b.Ore) {
		t.Fatal("same seed, different geology")
	}
	for i, c := range a.Terrain {
		t1 := terrain.Type(c)
		natural := terrain.IsRockWall(t1) || t1 == terrain.Stone || t1 == terrain.StoneSmooth
		if natural != (a.StoneType[i] != 0) {
			t.Fatalf("cell %d (%s) stone %d", i, t1, a.StoneType[i])
		}
		if a.Ore[i] != 0 && t1 != terrain.Rock {
			t.Fatalf("ore outside rock at %d", i)
		}
	}
}

func TestStonecutterBillCutsChunks(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.StonecutterTable, construction.South, 5, 2)
	g.SpawnStack(item.GraniteChunk, 2, 1, 4)
	g.SpawnStack(item.MarbleChunk, 1, 2, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.CutGranite), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	if bl := g.World().Buildings[0].Bills; len(bl) != 1 || bl[0].Label != "Cut granite blocks" {
		t.Fatalf("bills = %+v", bl)
	}
	for range 20 {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	if g.stockOf(item.GraniteBlocks) != recipe.CutYield || g.stockOf(item.GraniteChunk) != 1 || g.stockOf(item.MarbleChunk) != 1 {
		t.Fatalf("blocks %d chunks %d marble %d", g.stockOf(item.GraniteBlocks), g.stockOf(item.GraniteChunk), g.stockOf(item.MarbleChunk))
	}
	if w := g.World(); w.Stone != recipe.CutYield {
		t.Fatalf("stone total %d", w.Stone)
	}
	byName := map[string]RecipeView{}
	for _, r := range Recipes() {
		byName[r.Name] = r
	}
	if r := byName["cut_granite"]; r.Bench != "stonecutter table" || r.Input != "1 granite chunk" || r.Output != "granite blocks" {
		t.Fatalf("cut recipe view %+v", r)
	}
	if r := byName["knife"]; r.Input != "3 wood, metal or stone blocks" { // PRD-2: a stuffed knife
		t.Fatalf("knife recipe view %+v", r)
	}
}

func TestKnifeTakesAnyStoneBlocks(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.Workbench, construction.South, 5, 2)
	g.SpawnStack(item.SlateBlocks, 3, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.Knife), Mode: uint8(recipe.DoCount), Count: 1})
	for range 20 {
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(TicksPerCell * 10)
	}
	if made := g.stockOf(item.Knife) == 1 || g.equip[id] == item.Knife; !made || g.stockOf(item.SlateBlocks) != 0 {
		t.Fatalf("knife %d slate %d", g.stockOf(item.Knife), g.stockOf(item.SlateBlocks))
	}
}

func TestStoneGoodsInStockpilesAndWealth(t *testing.T) {
	t.Parallel()
	stone := zone.Zone{Kind: zone.KindStockpile, Stone: true} // PRD-4: only storage zones take things
	for _, k := range []item.Kind{item.GraniteChunk, item.MarbleBlocks, item.Stone} {
		if !allows(stone, k) || allows(zone.Zone{Kind: zone.KindStockpile, Wood: true}, k) {
			t.Fatalf("%s: stone filter", k)
		}
	}
	for _, k := range []item.Kind{item.Steel, item.Silver, item.Gold} {
		if !allows(zone.Zone{Kind: zone.KindStockpile}, k) {
			t.Fatalf("%s: metals are always welcome", k)
		}
	}
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	g.BuildNow("wall", "wood", "south", 2, 2)
	wood := g.Wealth()
	h := New(Config{Seed: 1, Width: 10, Height: 6})
	h.BuildNow("wall", "sandstone blocks", "south", 2, 2)
	if h.Wealth() <= wood {
		t.Fatalf("a stone wall is worth more: %d vs %d", h.Wealth(), wood)
	}
}

func TestSupplyReasonNamesTheStuff(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.placeFrameStuff(construction.Wall, item.MarbleBlocks, construction.South, 5, 3)
	var got OrderOption
	for _, o := range g.OrdersAt(id, 5, 3) {
		if o.Cmd.Order == command.OrderSupply {
			got = o
		}
	}
	if !got.Disabled || got.Reason != ReasonNoStuff+"marble blocks" {
		t.Fatalf("supply order %+v", got)
	}
}

func TestSoilLabel(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ ground, stone, ore, want string }{
		{"rock", "granite", "", "Granite rock wall"},
		{"rock_smooth", "granite", "", "Smoothed granite wall"},
		{"stone", "granite", "", "Rough granite ground"},
		{"stone_smooth", "granite", "", "Smooth granite floor"},
		{"rock", "granite", "steel", "Compacted steel (in granite)"},
		{"rock", "marble", "gold", "Gold ore (in marble)"},
		{"rock", "sandstone", "silver", "Silver ore (in sandstone)"},
		{"stone", "slate", "", "Rough slate ground"},
		{"rock", "", "", "Rock wall"},
		{"stone", "", "", "Rough stone"},
		{"rock_smooth", "", "", "Smoothed rock wall"},
		{"soil", "", "", "Soil"},
		{"rich_soil", "granite", "", "Rich soil"},
		{"bogus", "", "", ""},
	} {
		if got := SoilLabel(c.ground, c.stone, c.ore); got != c.want {
			t.Errorf("SoilLabel(%q, %q, %q) = %q, want %q", c.ground, c.stone, c.ore, got, c.want)
		}
	}
	smooth := map[string]bool{}
	for _, s := range Soils() {
		smooth[s.Name] = s.Smoothable
	}
	if !smooth["rock"] || !smooth["stone"] || smooth["rock_smooth"] || smooth["stone_smooth"] || smooth["soil"] {
		t.Fatalf("smoothable grounds %v", smooth)
	}
}

func TestStuffNamesAndAlias(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]item.Kind{"stone": item.GraniteBlocks, "granite blocks": item.GraniteBlocks,
		"slate blocks": item.SlateBlocks, "steel": item.Steel, "wood": item.Wood} {
		if id, ok := StuffID(name); !ok || item.Kind(id) != want {
			t.Errorf("StuffID(%q) = %d %v", name, id, ok)
		}
	}
	for _, name := range []string{"granite chunk", "gold", "berries"} {
		if _, ok := StuffID(name); ok {
			t.Errorf("%q is not a building material", name)
		}
	}
}
