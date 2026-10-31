package scenario

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/plant"
)

// Year proof (FOOD-6): a small purpose-built colony with roofed storage,
// a field and a fueled stove lives 60 days (a RimWorld year) on its own
// food. It starts with yearBuffer survival meals, about two weeks for its
// colonists, so living the year means eating what it grows and cooks.
// The run takes minutes: set RIMWORLD_PROBE=1 to run it (the -race suite
// skips it, like climate_test.go's year probe).
const (
	yearDays      = 60
	yearColonists = 4
	yearBuffer    = 100 // survival meals: 100 x 0.9 / (4 x 1.6 a day) = 14 days
)

// yearColony lays the colony out on a bare 60 x 50 map (no storyteller:
// the proof is about food, not raids): a row of walled, roofed rooms
// (a 5 x 5 bedroom and kitchen with dining, a campfire each for the
// winter, and a 9 x 9 storeroom with a stockpile, the firewood, parkas
// and tuques) and a 16 x 12 field of rice and potatoes south of them,
// half of it already growing.
func yearColony(t *testing.T) *game.Game {
	t.Helper()
	g := game.New(game.Config{Seed: 11, Width: 60, Height: 50})
	g.SetGround(0, 0, 59, 49, "soil", "", "")
	room := func(rx, ry, side int) { // walls around side x side, a door south
		for x := rx; x <= rx+side+1; x++ {
			for y := ry; y <= ry+side+1; y++ {
				switch {
				case x != rx && x != rx+side+1 && y != ry && y != ry+side+1:
				case x == rx+side/2+1 && y == ry+side+1:
					g.BuildNow("door", "wood", "south", x, y)
				default:
					g.BuildNow("wall", "granite blocks", "south", x, y)
				}
			}
		}
	}
	x0, y0 := 18, 8
	room(x0, y0, 5)
	room(x0+6, y0, 5)
	room(x0+12, y0, 9)
	for i := 0; i < yearColonists; i++ {
		g.BuildNow("bed", "wood", "south", x0+1+i%2*2, y0+1+i/2*3)
	}
	g.BuildNow("campfire", "wood", "south", x0+5, y0+3) // winter heat (banks down in warm air)
	kx := x0 + 6
	g.BuildNow("stove", "wood", "south", kx+1, y0+1)
	g.BuildNow("table", "wood", "south", kx+3, y0+3)
	g.BuildNow("chair", "wood", "south", kx+2, y0+3)
	g.BuildNow("chair", "wood", "south", kx+4, y0+3)
	g.BuildNow("chair", "wood", "south", kx+3, y0+4)
	g.BuildNow("campfire", "wood", "south", kx+5, y0+5)
	sx := x0 + 12
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: sx + 1, Y: y0 + 1, X2: sx + 9, Y2: y0 + 9})
	g.RoofRooms()
	put := func(k item.Kind, qty, x, y int) {
		if _, err := g.SpawnStack(k, qty, x, y); err != nil {
			t.Fatalf("stack %v at %d,%d: %v", k, x, y, err)
		}
	}
	for i := 0; i < yearBuffer/20; i++ {
		put(item.SurvivalMeal, 20, sx+1+i, y0+1)
	}
	for i := 0; i < 16; i++ { // the stove's and the winter's firewood
		put(item.Wood, 75, sx+1+i%8, y0+8+i/8)
	}
	put(item.HerbalMedicine, 10, sx+7, y0+1)
	put(item.Parka, yearColonists, sx+8, y0+1)
	put(item.Tuque, yearColonists, sx+9, y0+1)
	g.Enqueue(command.Command{Kind: command.AddBill, X: kx + 1, Y: y0 + 1, Recipe: uint8(recipe.SimpleMeal), Mode: uint8(recipe.UntilHave), Count: 12})
	fx, fy := 14, 20
	rice, _ := plant.Parse("rice")
	potato, _ := plant.Parse("potato")
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: fx, Y: fy, X2: fx + 15, Y2: fy + 5, Build: uint8(rice)})
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: fx, Y: fy + 6, X2: fx + 15, Y2: fy + 11, Build: uint8(potato)})
	for y := fy; y < fy+12; y += 2 {
		for x := fx; x < fx+16; x++ {
			species := "rice"
			if y >= fy+6 {
				species = "potato"
			}
			_, _ = g.SpawnCrop(species, x, y, (x*7+y*13)%80)
		}
	}
	for i := 0; i < yearColonists; i++ {
		if _, err := g.SpawnNamedPawn(x0+2+i*3, y0+12, fmt.Sprintf("Year %d", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func TestYearProofColonyFeedsItself(t *testing.T) {
	if os.Getenv("RIMWORLD_PROBE") == "" {
		t.Skip("set RIMWORLD_PROBE=1")
	}
	g := yearColony(t)
	seen := map[string]uint64{} // newest log tick read per colonist
	ate := map[string]int{}     // food kind -> sittings
	deaths := map[string]string{}
	starved := 0
	for day := 1; day <= yearDays; day++ {
		for h := 0; h < 24; h++ {
			g.Step(clock.TicksPerHour)
			for _, p := range g.Snapshot() {
				if p.Hostile {
					continue
				}
				for _, l := range p.Log { // newest first
					if l.Tick <= seen[p.Name] {
						break
					}
					if k, ok := strings.CutPrefix(l.Text, "Ate "); ok {
						ate[k]++
					}
				}
				if len(p.Log) > 0 {
					seen[p.Name] = max(seen[p.Name], p.Log[0].Tick)
				}
				if p.Status == "dead" && deaths[p.Name] == "" {
					deaths[p.Name] = fmt.Sprintf("day %d: %s", day, p.Reason)
					if p.Reason == "malnutrition" || p.Reason == "starvation" || strings.Contains(p.Reason, "food") {
						starved++
					}
				}
			}
		}
		if day%10 == 0 {
			alive, malnourished := 0, 0
			for _, p := range g.Snapshot() {
				if !p.Hostile && p.Status != "dead" {
					alive++
					if p.Malnutrition > 0 {
						malnourished++
					}
				}
			}
			w := g.World()
			t.Logf("day %2d %-6s alive %d malnourished %d food %4d (%s) food days %d.%d", day, w.Season, alive, malnourished, w.Food, kinds(w.FoodByKind), w.FoodDays/10, w.FoodDays%10)
		}
	}
	t.Logf("eaten (sittings): %s", kinds(ate))
	t.Logf("deaths: %v", deaths)
	alive := 0
	for _, p := range g.Snapshot() {
		if !p.Hostile && p.Status != "dead" {
			alive++
		}
	}
	if starved > 0 {
		t.Errorf("%d colonists died of hunger: %v", starved, deaths)
	}
	if alive == 0 {
		t.Fatalf("the colony died out: %v", deaths)
	}
	own := 0
	for k, n := range ate {
		if k != item.SurvivalMeal.String() {
			own += n
		}
	}
	if own <= ate[item.SurvivalMeal.String()] {
		t.Errorf("ate %d sittings of its own food against %d survival meals: it should live off its production", own, ate[item.SurvivalMeal.String()])
	}
}

// kinds prints a kind -> count map in name order.
func kinds(m map[string]int) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var b strings.Builder
	for _, k := range ks {
		fmt.Fprintf(&b, "%s=%d ", k, m[k])
	}
	return strings.TrimSpace(b.String())
}
