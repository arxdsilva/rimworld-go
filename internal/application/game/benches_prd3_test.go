package game

import (
	"bytes"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
)

// PRD-3: the new benches in the whole game.

// TestForgeRefuelNoFlap: haulers fill a wood-fired forge only once it is
// below half (refuelBelow-style hysteresis) and fill it to the top; an
// idle forge burns nothing, so a full or half-full one is never topped
// up again and again.
func TestForgeRefuelNoFlap(t *testing.T) {
	t.Parallel()
	g := fuelGame(t)
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	g.SetNeeds(id, needs.Full, needs.Full)
	forge := placeFuel(t, g, construction.FueledSmithy, 8, 4)
	if _, err := g.SpawnStack(item.Wood, 30, 2, 6); err != nil {
		t.Fatal(err)
	}
	spec, ok := fuelSpec(construction.FueledSmithy)
	if !ok || spec.CapTicks != 5*TicksPerWood || spec.BurnPermil != 0 {
		t.Fatalf("forge fuel spec %+v %v", spec, ok)
	}
	g.setFuel(forge, spec.Below+TicksPerWood/2) // just over half: left alone
	if g.assignRefuel(id) {
		t.Fatal("refuelled a forge over half full")
	}
	before := g.fuel[forge]
	g.Step(3 * climateInterval)
	if g.fuel[forge] != before {
		t.Fatalf("an idle forge burnt fuel: %d -> %d", before, g.fuel[forge])
	}
	g.setFuel(forge, spec.Below-1) // under half: filled to the top
	for i := 0; i < 2000 && g.fuel[forge] < spec.CapTicks-TicksPerWood; i++ {
		g.Step(1)
	}
	if g.fuel[forge] < spec.CapTicks-TicksPerWood || g.fuel[forge] > spec.CapTicks {
		t.Fatalf("forge fuel %d after refuelling, cap %d", g.fuel[forge], spec.CapTicks)
	}
	g.setFuel(forge, spec.CapTicks-TicksPerWood) // used a little: no trip back
	if g.assignRefuel(id) {
		t.Fatal("topped a nearly full forge up again")
	}
}

// TestForgeSmithsAndBurns: a colonist forges a mace at a fuelled forge,
// the forge burns a log per WorkPerWood work, and the bench's view shows
// its work speed.
func TestForgeSmithsAndBurns(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	jumpTo(g, 10, 12)
	smith, _ := g.SpawnNamedPawn(1, 1, "Ada")
	forge := buildNow(t, g, construction.FueledSmithy, item.Steel, construction.South, 5, 2)
	g.SpawnStack(item.Steel, 10, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.MakeMace), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	if len(g.craft.Bills.Of(forge)) != 1 {
		t.Fatal("the forge takes no mace bill")
	}
	if v := g.craft.Bills.AppendViews(nil, forge)[0]; v.Status != BillNoFuel {
		t.Fatalf("an empty forge: %q", v.Status)
	}
	full := 5 * TicksPerWood
	g.setFuel(forge, full)
	for i := 0; i < 6000 && g.craft.Bills.Of(forge)[0].Done == 0; i++ {
		if i%100 == 0 {
			g.SetNeeds(smith, needs.Full, needs.Full)
		}
		g.Step(1)
	}
	if g.craft.Bills.Of(forge)[0].Done != 1 {
		t.Fatal("no mace forged")
	}
	burnt := full - g.fuel[forge]
	work := recipe.DefOf(recipe.MakeMace).Work
	if want := work * TicksPerWood / craft.WorkPerWood; burnt < want/2 || burnt > want*2 {
		t.Fatalf("burnt %d fuel ticks for %d work, want about %d", burnt, work, want)
	}
	var bv BuildingView
	for _, b := range g.World().Buildings {
		if b.ID == forge {
			bv = b
		}
	}
	if bv.WorkSpeed <= 0 || bv.WorkSpeed > 1000 || !bv.Fuel {
		t.Fatalf("forge view speed %d fuel %v", bv.WorkSpeed, bv.Fuel)
	}
}

// TestSculptureInGame: a sculpting bench bill sets a sculpture down
// beside the bench with a title, its beauty from the furniture table,
// and the art survives a save.
func TestSculptureInGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 20, Height: 12})
	jumpTo(g, 10, 12)
	artist, _ := g.SpawnNamedPawn(1, 1, "Ada")
	bench := buildNow(t, g, construction.ArtBench, item.Wood, construction.South, 8, 4)
	g.SpawnStack(item.Wood, 20, 1, 8)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 8, Y: 4, Recipe: uint8(recipe.SculptSmall), Mode: uint8(recipe.DoCount), Count: 1})
	g.Step(1)
	for i := 0; i < 20000 && g.craft.Bills.Of(bench)[0].Done == 0; i++ {
		if i%100 == 0 {
			g.SetNeeds(artist, needs.Full, needs.Full)
		}
		g.Step(1)
	}
	if g.craft.Bills.Of(bench)[0].Done != 1 {
		t.Fatal("no sculpture carved")
	}
	var sv BuildingView
	for _, b := range g.World().Buildings {
		if b.Kind == "small sculpture" {
			sv = b
		}
	}
	if sv.ID == 0 || sv.Epitaph == "" || sv.Quality == "" || sv.Beauty <= 0 || sv.Stuff != "wood" {
		t.Fatalf("sculpture view %+v", sv)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "art"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := h.bldgs.Get(sv.ID)
	if got := h.craft.Art.Title(sv.ID, b); got != sv.Epitaph {
		t.Fatalf("title after load %q, want %q", got, sv.Epitaph)
	}
	if d := h.craft.Art.Describe(sv.ID, b); !strings.HasSuffix(d, "Carved by Ada.") {
		t.Fatalf("description after load %q", d)
	}
}

// TestBillActionWords: game.Action* and craft.Action* say the same.
func TestBillActionWords(t *testing.T) {
	t.Parallel()
	if craft.ActionCraft != ActionCraft || craft.ActionCook != ActionCook || craft.ActionTailor != ActionTailor ||
		craft.ActionButcher != ActionButcher || craft.ActionCremate != ActionCremate {
		t.Fatal("bill action words differ")
	}
}

// TestWaveDPoweredKinds: every powered wave D kind is a consumer on the
// grid and reads powered in the view once connected (the renderer's
// powered look comes from PowerRole/Powered).
func TestWaveDPoweredKinds(t *testing.T) {
	t.Parallel()
	for _, k := range []construction.Kind{construction.ElectricSmithy, construction.MachiningTable, construction.FabricationBench,
		construction.ElectricSmelter, construction.BiofuelRefinery, construction.Autocannon, construction.UraniumSlugTurret,
		construction.Autodoor} {
		g := New(Config{Seed: 1, Width: 16, Height: 10})
		id := g.placeGenerated(construction.Building{Kind: k, Stuff: item.Steel, X: 5, Y: 2})
		if id == 0 {
			t.Fatalf("could not place %s", k)
		}
		onPower(t, g, 5, 2, 5, 5)
		var bv BuildingView
		for _, b := range g.World().Buildings {
			if b.ID == id {
				bv = b
			}
		}
		if bv.PowerRole != "consumer" || !bv.Powered {
			t.Errorf("%s: role %q powered %v (%s)", k, bv.PowerRole, bv.Powered, bv.NoPowerReason)
		}
	}
}
