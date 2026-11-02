package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/power"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// wikiBenches is every kind STK-BN gave its wiki footprint.
var wikiBenches = []construction.Kind{construction.Workbench, construction.Stove, construction.ButcherTable,
	construction.TailoringBench, construction.StonecutterTable, construction.BrewingBench, construction.DrugTable,
	construction.ElectricStove, construction.ElectricTailoringBench, construction.FueledSmithy, construction.ElectricSmithy,
	construction.MachiningTable, construction.ArtBench, construction.ElectricSmelter, construction.FabricationBench,
	construction.BiofuelRefinery, construction.RadioConsole}

// TestStkBenchSaveMigration (STK-BN, save v10): a bench whose wiki
// footprint still fits keeps its origin and facing and blocks its new
// cells; one whose new cells meet a wall or the map edge is taken down,
// refunded in full and announced; a blueprint is cancelled.
func TestStkBenchSaveMigration(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 30, Height: 20})
	keep := buildNow(t, g, construction.Stove, item.Wood, construction.East, 4, 4)
	radio := buildNow(t, g, construction.RadioConsole, item.Steel, construction.South, 4, 12)
	walled := buildNow(t, g, construction.Workbench, item.Wood, construction.South, 10, 4)
	buildNow(t, g, construction.Wall, item.Wood, construction.South, 14, 4)
	edge := buildNow(t, g, construction.MachiningTable, item.Steel, construction.South, 27, 8)
	g.placeFrameStuff(construction.TailoringBench, item.Wood, construction.South, 20, 14)
	if len(g.frameIDs) != 1 {
		t.Fatal("blueprint")
	}
	before := stackTotals(g)
	l := loadAsVersionWith(t, g, 9, func(s *saveState) {
		for i := range s.Bldgs { // where a 2x1 bench could stand before v10
			switch s.Bldgs[i].ID {
			case walled:
				s.Bldgs[i].V.X = 12 // its third cell is the wall
			case edge:
				s.Bldgs[i].V.X = 28 // its third cell is off the map
			}
		}
	})
	if len(l.frameIDs) != 0 {
		t.Fatal("a bench blueprint is cancelled: placed again at the new size")
	}
	for _, id := range []entity.EntityID{keep, radio} {
		b, ok := l.bldgs.Get(id)
		if !ok {
			t.Fatalf("bench %d fits: it stays", id)
		}
		cells, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		for _, c := range cells {
			if l.grid.Walkable(c[0], c[1]) {
				t.Fatalf("%s cell %v is not blocked", b.Kind, c)
			}
		}
		if !l.grid.Walkable(spot[0], spot[1]) {
			t.Fatalf("%s spot %v is blocked", b.Kind, spot)
		}
	}
	if b, _ := l.bldgs.Get(keep); b.X != 4 || b.Y != 4 || b.Facing != construction.East {
		t.Fatalf("the stove moved: %+v", b)
	}
	for _, id := range []entity.EntityID{walled, edge} {
		if _, ok := l.bldgs.Get(id); ok {
			t.Fatalf("bench %d no longer fits: it is taken down", id)
		}
	}
	after := stackTotals(l)
	if d := after[item.Wood] - before[item.Wood]; d != construction.WoodCost(construction.Workbench) {
		t.Fatalf("workbench refund %d wood", d)
	}
	if d := after[item.Steel] - before[item.Steel]; d != construction.WoodCost(construction.MachiningTable) {
		t.Fatalf("machining table refund %d steel", d)
	}
	if d := after[item.Components] - before[item.Components]; d != construction.ComponentCost(construction.MachiningTable) {
		t.Fatalf("machining table refund %d components", d)
	}
	msgs := 0
	for _, m := range l.World().Messages {
		if strings.Contains(m.Text, "no longer fits") && (strings.Contains(m.Text, "workbench") || strings.Contains(m.Text, "machining table")) {
			msgs++
		}
	}
	if msgs != 2 {
		t.Fatalf("%d refund messages, want 2", msgs)
	}
	l.Step(300) // and it keeps running
	if again := loadAsVersion(t, l, SaveVersion); len(again.bldgIDs) != len(l.bldgIDs) {
		t.Fatal("a v10 save is not migrated again")
	}
}

// stackTotals is every loose stack's quantity by kind.
func stackTotals(g *Game) map[item.Kind]int {
	out := map[item.Kind]int{}
	for _, id := range g.itemIDs {
		st, _ := g.stacks.Get(id)
		out[st.Kind] += st.Qty
	}
	return out
}

// TestPawnWorksWikiBenchFromMiddleFront (STK-BN): a crafter works every
// resized bench standing on the cell in front of its middle, in all four
// facings.
func TestPawnWorksWikiBenchFromMiddleFront(t *testing.T) {
	t.Parallel()
	for _, k := range wikiBenches {
		if k == construction.RadioConsole {
			continue // it runs no bills; its operator spot is in TestWikiBenchFootprints
		}
		r, ok := plainRecipe(k)
		if !ok {
			t.Fatalf("no bill to test the %s with", k)
		}
		for f := construction.South; f <= construction.West; f++ {
			t.Run(k.String()+"/"+f.String(), func(t *testing.T) {
				t.Parallel()
				workBenchOnce(t, k, f, r)
			})
		}
	}
}

// plainRecipe is a bill of k whose inputs inputKind can stand in for.
func plainRecipe(k construction.Kind) (recipe.Def, bool) {
	for _, r := range recipe.For(k) {
		if r.Legacy || r.BySpecies || r.Salvage {
			continue
		}
		plain := true
		for _, in := range recipeInputs(r) {
			plain = plain && in.Qty > 0 && !in.AnyDryable && !in.SmeltWear && !in.Wreck
		}
		if plain {
			return r, true
		}
	}
	return recipe.Def{}, false
}

func recipeInputs(r recipe.Def) []recipe.Input {
	if len(r.Inputs) == 0 {
		return []recipe.Input{r.Input}
	}
	return r.Inputs
}

// inputKind is an item kind in takes: berries for raw food, meat, a steel
// weapon for smelting, else its own kind.
func inputKind(in recipe.Input) item.Kind {
	switch {
	case in.AnyRaw || in.AnyVeg:
		return item.Berries
	case in.AnyMeat:
		return item.Meat
	case in.Smelt:
		for _, d := range recipe.All() {
			if d.Output.IsWeapon() && recipe.SmeltYield(item.Stack{Kind: d.Output, Qty: 1}) > 0 {
				return d.Output
			}
		}
	}
	return in.Kind
}

func workBenchOnce(t *testing.T, k construction.Kind, f construction.Facing, r recipe.Def) {
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	setSkill(g, ada, r.Learn, max(r.MinSkill, 10))
	bench := buildNow(t, g, k, construction.Stuffs(k)[0], f, 9, 8)
	if power.SpecOf(k).Role == power.Consumer {
		onPower(t, g, 9, 8, 12, 11)
	}
	if _, fueled := fuelSpec(k); fueled {
		g.setFuel(bench, g.fuelCap(bench))
	}
	for i, in := range recipeInputs(r) {
		g.SpawnStack(inputKind(in), in.Qty*2, 2+i, 5)
	}
	b, _ := g.bldgs.Get(bench)
	_, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
	g.Enqueue(command.Command{Kind: command.AddBill, X: b.X, Y: b.Y, Recipe: uint8(r.ID), Mode: uint8(recipe.DoCount), Count: 1})
	worked := 0
	for i := 0; i < 8000 && worked < 30; i++ {
		if i%200 == 0 {
			g.SetNeeds(ada, needs.Full, needs.Full)
		}
		g.Step(1)
		j, _ := g.jobs.Get(ada)
		if j.Kind != job.Bill || j.Dest != bench || j.State != job.Do || j.Carry == 0 { // working: the ingredients are laid out
			continue
		}
		if p, _ := g.positions.Get(ada); p.X != spot[0] || p.Y != spot[1] {
			t.Fatalf("works %s from %d,%d, want the middle-front %v", r.Name, p.X, p.Y, spot)
		}
		worked++
	}
	if worked == 0 {
		t.Fatalf("nobody worked %s at the %s", r.Name, k)
	}
}
