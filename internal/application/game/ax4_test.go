package game

import (
	"testing"

	"rimworld-go/internal/application/game/haul"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// AX-4: power and industry end to end in the game.

// frameOf is the frame at (x, y) on layer l (ok false: none).
func frameOf(g *Game, x, y int, l construction.Layer) (entity.EntityID, construction.Frame, bool) {
	fid := g.layerFrameAt(x, y, l)
	fr, ok := g.frames.Get(fid)
	return fid, fr, ok
}

// A blueprint's secondary materials (Def.Extra) are hauled after its
// material and components, shown on the frame view, and block the build
// until they are in; cancelling drops them back on the map.
func TestFrameTakesSecondaryMaterials(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SpawnStack(item.GraniteBlocks, 40, 2, 8)
	g.SpawnStack(item.Components, 5, 3, 8)
	g.Enqueue(command.Command{Kind: command.DesignateBuild, X: 8, Y: 3, Build: uint8(construction.ElectricCrematorium), Stuff: uint8(item.GraniteBlocks)})
	g.Step(1)
	fid, fr, ok := frameOf(g, 8, 3, construction.LayerObject)
	if !ok || fr.ExtraNeed(0) != 2 {
		t.Fatalf("no crematorium blueprint needing 2 steel: %+v", fr)
	}
	for i := 0; i < 3000; i++ {
		if i%200 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
		if fr, _ = g.frames.Get(fid); fr.Wood >= fr.Need && fr.Comp >= fr.CompNeed {
			break
		}
	}
	if fr.Wood < fr.Need || fr.Comp < fr.CompNeed || fr.Ready() {
		t.Fatalf("material and components in, steel still missing: %+v ready=%v", fr, fr.Ready())
	}
	var fv FrameView
	for _, v := range g.World().Frames {
		if v.ID == fid {
			fv = v
		}
	}
	if fv.AX.Extra[0] != "steel" || fv.AX.Need[0] != 2 || fv.AX.ExtraGot[0] != 0 || fv.Status != "Needs 2 steel" {
		t.Fatalf("frame view: %+v status %q", fv.AX, fv.Status)
	}
	g.SpawnStack(item.Steel, 10, 4, 8)
	for i := 0; i < 2000 && !fr.Ready(); i++ {
		g.Step(1)
		fr, _ = g.frames.Get(fid)
	}
	if fr.ExtraGot[0] != 2 || !fr.Ready() {
		t.Fatalf("steel not delivered: %+v", fr)
	}
	if n := g.haul.incoming[haul.ExtraKey(fid, 0)]; n != 0 {
		t.Fatalf("incoming left on the steel slot: %d", n)
	}
	g.cancelFrame(fid)
	steel := 0
	g.stacks.All(func(_ entity.EntityID, st item.Stack) {
		if st.Kind == item.Steel {
			steel += st.Qty
		}
	})
	if steel != 10 {
		t.Fatalf("cancel: %d steel on the map, want all 10 back", steel)
	}
}

// A waterproof conduit goes over shallow water, a plain one does not;
// neither over deep water.
func TestWaterproofConduitOverShallowWater(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	fillTerrain(g, 3, 1, 4, 1, terrain.ShallowWater)
	g.setTerrain(5, 1, terrain.DeepWater)
	if ok, r := g.CanPlaceAt("power conduit", "steel", "south", 3, 1); ok || r != ReasonWater {
		t.Fatalf("plain conduit on water: %v %q", ok, r)
	}
	if ok, r := g.CanPlaceAt("waterproof conduit", "steel", "south", 3, 1); !ok {
		t.Fatalf("waterproof conduit on shallow water refused: %q", r)
	}
	if ok, _ := g.CanPlaceAt("waterproof conduit", "steel", "south", 5, 1); ok {
		t.Fatal("waterproof conduit on deep water")
	}
	a := buildNow(t, g, construction.WaterproofConduit, item.Steel, construction.South, 3, 1)
	buildNow(t, g, construction.WaterproofConduit, item.Steel, construction.South, 4, 1)
	gen := buildNow(t, g, construction.ChemfuelGenerator, item.Steel, construction.South, 1, 1)
	g.setFuel(gen, g.fuelCap(gen))
	g.balancePower(uint64(g.clock.Now()))
	if g.genOutput(gen) != chemfuelGenW {
		t.Fatalf("chemfuel generator: %d W", g.genOutput(gen))
	}
	if g.netOf(a) == 0 || g.netOf(a) != g.netOf(gen) {
		t.Fatal("the waterproof conduit carries the generator's power")
	}
}

// The chemfuel generator burns 4.5 chemfuel a day while running and a
// hauler refuels it with chemfuel only.
func TestChemfuelGeneratorBurnsAndRefuels(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	gen := buildNow(t, g, construction.ChemfuelGenerator, item.Steel, construction.South, 6, 2)
	g.setFuel(gen, chemfuelTicks*10)
	before := g.fuel[gen]
	for range 240 { // one day of climate intervals
		g.burnFuel()
	}
	if used := before - g.fuel[gen]; used < chemfuelTicks*4 || used > chemfuelTicks*5 {
		t.Fatalf("burnt %d ticks of fuel in a day, want 4.5 chemfuel (%d)", used, chemfuelTicks*9/2)
	}
	if g.fuelRoom(gen, item.Wood) == 0 {
		t.Fatal("room for fuel")
	}
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SpawnStack(item.Wood, 50, 2, 5)
	if g.assignRefuel(ada) {
		t.Fatal("a chemfuel generator takes no wood")
	}
	g.SpawnStack(item.Chemfuel, 40, 3, 5)
	if !g.assignRefuel(ada) {
		t.Fatal("no refuel with chemfuel about")
	}
	j, _ := g.jobs.Get(ada)
	if st, _ := g.stacks.Get(j.Target); st.Kind != item.Chemfuel {
		t.Fatalf("refuel brings %s", st.Kind)
	}
}

// A watermill's wheel goes in the river's moving water; it makes 1100 W.
func TestWatermillOnTheRiver(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 14})
	fillTerrain(g, 0, 8, 19, 9, terrain.ShallowWater)
	flow := make([]uint8, 20*14)
	for x := range 20 {
		flow[8*20+x], flow[9*20+x] = 3, 3
	}
	if ok, r := g.CanPlaceAt("watermill generator", "wood", "south", 4, 2); ok || r == "" {
		t.Fatal("no river: refused")
	}
	if ok, _ := g.CanPlaceAt("watermill generator", "wood", "south", 4, 3); ok {
		t.Fatal("still water is no river")
	}
	g.br.ind.SetFlow(flow, 20, 14)
	if ok, r := g.CanPlaceAt("watermill generator", "wood", "south", 4, 3); !ok {
		t.Fatalf("wheel in the river refused: %q", r)
	}
	if ok, _ := g.CanPlaceAt("watermill generator", "wood", "south", 4, 4); ok {
		t.Fatal("housing on the water")
	}
	mill := buildNow(t, g, construction.Watermill, item.Wood, construction.South, 4, 3)
	if w := g.genOutput(mill); w != 1100 {
		t.Fatalf("watermill: %d W", w)
	}
	for _, bv := range g.World().Buildings {
		if bv.ID == mill && bv.AX.Flow != 1000 {
			t.Fatalf("flow view %d", bv.AX.Flow)
		}
	}
	// a save keeps the river
	g2 := saveLoad(t, g)
	if w := g2.genOutput(mill); w != 1100 {
		t.Fatalf("after load: %d W", w)
	}
}

// A ground scan's find lays deep ore; a miner works a powered deep drill
// over it and brings the ore up; the scanner is a researcher's job.
func TestDeepDrillBringsUpOre(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 24, Height: 16})
	if !g.br.ind.Find(industryHost{g}) {
		t.Fatal("no find")
	}
	w := g.World()
	if len(w.AX.DeepOre) == 0 || w.AX.DeepOreVer == 0 {
		t.Fatal("the world view lists no deep ore")
	}
	d := w.AX.DeepOre[0]
	ore, _ := item.ParseKind(d.Ore)
	dx, dy := d.X, max(0, d.Y-1)
	if dy+3 >= 16 {
		dy = 16 - 4
	}
	drill := buildNow(t, g, construction.DeepDrill, item.Steel, construction.South, dx, dy)
	gx := dx + 2
	if gx+1 >= 24 {
		gx = dx - 3
	}
	gen := onPower(t, g, dx, dy, gx, dy)
	if !g.powered(drill) {
		t.Fatal("drill unpowered")
	}
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	for _, wt := range []work.Type{work.Haul, work.Construct, work.Clean} {
		g.SetWork(ada, wt, false)
	}
	got := 0
	for i := 0; i < 30000 && got == 0; i++ {
		if i%500 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
		if i == 2000 {
			if j, _ := g.jobs.Get(ada); j.Kind != job.OperateDrill {
				t.Fatalf("the miner is not drilling: %v", j.Kind)
			}
		}
		g.stacks.All(func(_ entity.EntityID, st item.Stack) {
			if st.Kind == ore {
				got += st.Qty
			}
		})
	}
	if got == 0 {
		t.Fatalf("no %s brought up", ore)
	}
	if g.genOutput(gen) == 0 {
		t.Fatal("the generator ran dry")
	}
}

// No flap (AGENTS.md hysteresis rule): a drill that loses its power fails
// its operator's sitting and is put off for postBlockedTicks, so the
// operator does not hop back on and off while the power flickers.
func TestDrillPowerLossDoesNotFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	drill := buildNow(t, g, construction.DeepDrill, item.Steel, construction.South, 5, 2)
	gen := onPower(t, g, 5, 2, 8, 2)
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	for _, wt := range []work.Type{work.Haul, work.Construct, work.Clean} {
		g.SetWork(ada, wt, false)
	}
	for i := 0; i < 600; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.OperateDrill && j.State == job.Do {
			break
		}
	}
	if j, _ := g.jobs.Get(ada); j.Kind != job.OperateDrill || j.State != job.Do {
		t.Fatalf("not drilling: %v %v", j.Kind, j.State)
	}
	g.setFuel(gen, 0) // the power goes
	g.balancePower(uint64(g.clock.Now()))
	starts := 0
	for i := 0; i < 400; i++ {
		if i == 5 { // and comes straight back
			g.setFuel(gen, g.fuelCap(gen))
			g.balancePower(uint64(g.clock.Now()))
		}
		prev, _ := g.jobs.Get(ada)
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.OperateDrill && prev.Kind != job.OperateDrill {
			starts++
		}
	}
	if starts != 0 || !g.postponedFor(ada, job.OperateDrill, drill, 0, 0) {
		t.Fatalf("the operator flapped back onto the drill %d times (postponed %v)", starts, g.postponedFor(ada, job.OperateDrill, drill, 0, 0))
	}
}

// The electric crematorium burns a corpse on power, without wood, in the
// wiki's 180 ticks of work.
func TestElectricCrematoriumBurnsCorpses(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	crem := buildNow(t, g, construction.ElectricCrematorium, item.GraniteBlocks, construction.South, 6, 2)
	onPower(t, g, 6, 2, 9, 2)
	if len(g.craft.Bills.Of(crem)) != 1 {
		t.Fatal("no default cremation bill")
	}
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	g.SpawnStack(item.CorpseHuman, 1, 3, 8)
	corpses := func() int {
		n := 0
		g.stacks.All(func(_ entity.EntityID, st item.Stack) {
			if st.Kind == item.CorpseHuman {
				n++
			}
		})
		return n
	}
	for i := 0; i < 3000 && corpses() > 0; i++ {
		if i%300 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Bill && j.Dest == crem && j.State == job.Do && j.Carry != 0 && j.WorkLeft > 180 {
			t.Fatalf("crematorium work %d, want 180", j.WorkLeft)
		}
	}
	if corpses() > 0 {
		j, _ := g.jobs.Get(ada)
		t.Fatalf("the corpse was not cremated: %+v %v", j.Kind, g.craft.Bills.AppendViews(nil, crem))
	}
}

// A powered moisture pump dries the marsh around it over the days.
func TestMoisturePumpDriesMarsh(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	fillTerrain(g, 2, 2, 12, 10, terrain.Marsh)
	pump := buildNow(t, g, construction.MoisturePump, item.Steel, construction.South, 6, 6)
	onPower(t, g, 6, 6, 4, 8)
	if !g.powered(pump) {
		t.Fatal("pump unpowered")
	}
	ip := g.br.ind.Pumps(industryHost{g})
	ip.Advance(2 * 87 * 60000 / 10) // 17.4 days: radius 2
	if g.terr.At(8, 6) != terrain.Soil || g.terr.At(9, 6) != terrain.Marsh {
		t.Fatalf("radius 2: %v %v", g.terr.At(8, 6), g.terr.At(9, 6))
	}
	for _, bv := range g.World().Buildings {
		if bv.ID == pump && bv.AX.Radius != 2 {
			t.Fatalf("radius view %d", bv.AX.Radius)
		}
	}
}
