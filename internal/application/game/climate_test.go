package game

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
)

// jumpTo moves the clock to day (0-based) at hour without simulating.
func jumpTo(g *Game, day, hour int) {
	target := clock.Tick(day*clock.TicksPerDay + hour*clock.TicksPerHour)
	g.clock.Advance(int(target - g.clock.Now()))
}

func stackOf(g *Game, k item.Kind) (item.Stack, bool) {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k {
			return st, true
		}
	}
	return item.Stack{}, false
}

func TestMeatRotsAwayInSummer(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 6})
	jumpTo(g, 15, 0) // mid-summer
	if _, err := g.SpawnStack(item.Meat, 20, 2, 2); err != nil {
		t.Fatal(err)
	}
	g.Step(clock.TicksPerDay)
	st, ok := stackOf(g, item.Meat)
	if !ok || st.Rot == 0 {
		t.Fatalf("meat should be rotting: %+v %v", st, ok)
	}
	g.Step(2 * clock.TicksPerDay)
	if _, ok := stackOf(g, item.Meat); ok {
		t.Fatal("meat should have rotted away after three summer days")
	}
	w := g.World()
	if len(w.Messages) == 0 || !strings.Contains(w.Messages[len(w.Messages)-1].Text, "meat rotted away") {
		t.Fatalf("messages = %+v", w.Messages)
	}
}

func TestWinterFreezesFoodAndKillsCrops(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 6})
	jumpTo(g, 45, 0) // mid-winter
	if _, err := g.SpawnStack(item.Meat, 20, 2, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnPlantSpecies("corn", 4, 3, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnPlantSpecies("tree", 5, 3, 100); err != nil {
		t.Fatal(err)
	}
	g.Step(clock.TicksPerDay)
	if st, ok := stackOf(g, item.Meat); !ok || st.Rot != 0 {
		t.Fatalf("frozen meat should not rot: %+v %v", st, ok)
	}
	corn, tree := 0, 0
	for _, id := range g.plantIDs {
		p, _ := g.plants.Get(id)
		switch p.Species {
		case plant.Corn:
			corn++
		case plant.Tree:
			tree++
		}
	}
	if corn != 0 || tree != 1 {
		t.Fatalf("corn=%d tree=%d: frost kills crops, not trees", corn, tree)
	}
}

func TestNoSowingInWinter(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 6})
	if !g.sowable(3, 3) {
		t.Fatal("spring soil should be sowable")
	}
	jumpTo(g, 45, 12)
	if g.sowable(3, 3) {
		t.Fatal("frozen mid-winter soil should not be sown")
	}
}

func TestRottenCorpseIsNotButchered(t *testing.T) {
	g := New(Config{Seed: 1, Width: 8, Height: 6})
	if _, err := g.SpawnStack(item.CorpseAnimal, 1, 2, 2); err != nil {
		t.Fatal(err)
	}
	for _, id := range g.itemIDs {
		st, _ := g.stacks.Get(id)
		st.Rot = item.CorpseAnimal.ShelfTicks()
		g.stacks.Set(id, st)
	}
	if id, _ := g.ingredientFor(0, recipe.DefOf(recipe.ButcherAnimal).Input); id != 0 {
		t.Fatal("rotten corpse offered as butchering ingredient")
	}
}

// TestYearProbe runs a generated colony through a full year and logs how it
// fares. Set RIMWORLD_PROBE=1 to run.
func TestYearProbe(t *testing.T) {
	if os.Getenv("RIMWORLD_PROBE") == "" {
		t.Skip("set RIMWORLD_PROBE=1")
	}
	g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	// play it like a player would: a stockpile, berry picking, hunting and
	// a field of every crop
	cx, cy := g.Width()/2, g.Height()/2
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: cx + 1, Y: cy + 3, X2: cx + 4, Y2: cy + 5})
	g.Enqueue(command.Command{Kind: command.DesignateHarvest, X: 0, Y: 0, X2: g.Width() - 1, Y2: g.Height() - 1})
	g.Enqueue(command.Command{Kind: command.DesignateHunt, X: 0, Y: 0, X2: g.Width() - 1, Y2: g.Height() - 1})
	// a hut with three beds and a campfire, and wood cutting for the fire
	g.ClearPlants(cx-14, cy+3, cx-6, cy+9)
	hut(t, g, cx-14, cy+3, cx-7, cy+9)
	for i := 0; i < 3; i++ {
		if !g.BuildNow("bed", "wood", "south", cx-13+2*i, cy+4) {
			t.Fatalf("bed %d", i)
		}
	}
	if !g.BuildNow("campfire", "wood", "south", cx-10, cy+7) {
		t.Fatal("campfire")
	}
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: 0, Y: 0, X2: g.Width() - 1, Y2: cy - 8})
	for i, c := range []string{"potato", "corn", "wheat", "beans", "radish", "pumpkin", "lettuce"} {
		id, _ := CropID(c)
		x := cx - 9 + (i%4)*3
		y := cy - 6 + (i/4)*3
		g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: x, Y: y, X2: x + 2, Y2: y + 2, Build: id})
	}
	var seen uint64
	for day := 0; day < clock.DaysPerYear; day++ {
		g.Step(clock.TicksPerDay)
		w := g.World()
		for _, m := range w.Messages {
			if m.Tick > seen {
				t.Logf("  msg: %s", m.Text)
			}
		}
		if n := len(w.Messages); n > 0 {
			seen = w.Messages[n-1].Tick
		}
		alive := 0
		for _, p := range g.Snapshot() {
			if p.Status != "dead" {
				alive++
			}
		}
		hutT := float64(g.temperatureAt(cx-10, cy+5)) / 10
		var moods []string
		for _, p := range g.Snapshot() {
			moods = append(moods, fmt.Sprintf("%s:%d/%d%s", p.Name, p.Mood, p.Recreation, map[bool]string{true: "!" + p.MentalBreak, false: ""}[p.MentalBreak != ""]))
		}
		t.Logf("day %2d %-6s out %5.1fC hut %5.1fC wood %3d food %4d alive %d moods %v", day+1, w.Season, float64(w.OutdoorTemp)/10, hutT, w.Wood, w.Food, alive, moods)
		if os.Getenv("RIMWORLD_PROBE") == "raiders" {
			for _, rid := range g.raiderIDs {
				p, _ := g.positions.Get(rid)
				j, _ := g.jobs.Get(rid)
				v, _ := g.vital.Get(rid)
				r := g.raiders[rid]
				t.Logf("  raider %d at %v %v job %v/%v fail %v target %v tx %d,%d mode %v retry %d idle %d", rid, p, v.Status, j.Kind, j.State, j.Fail, j.Target, j.TX, j.TY, r.Mode, r.RetryAt, r.Idle)
			}
		}
	}
}
