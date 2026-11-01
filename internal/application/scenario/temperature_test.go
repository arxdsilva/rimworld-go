package scenario

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
)

// Temperature probes (TEMP-6): purpose-built colonies through the worst
// of their climate, each with and without the gear that should save it,
// and a freezer. Real game, public hooks only, no storyteller (the heat
// wave is fired by hand). The runs take minutes: set RIMWORLD_PROBE=1 to
// run them (the -race suite skips them, like the year proof).

// tempGear is what a probe colony gets.
type tempGear struct {
	heaters, coolers, parkas bool
}

// tempColony is a bedroom (7x5 inside, four beds) and a storeroom (7x5,
// a stockpile with 200 survival meals, firewood and medicine) side by
// side on a bare 50 x 30 map, a wood-fired generator and a battery north
// of them on a conduit line, and four colonists, all made on day0. gear adds a heater in each room, a
// cooler in each room's north wall (hot side out), parkas and tuques.
func tempColony(t *testing.T, b biome.ID, seed uint64, gear tempGear, day0 int) *game.Game {
	t.Helper()
	g := game.New(game.Config{Seed: seed, Width: 50, Height: 30, Biome: b})
	// 06:00, before anything is built or born: rested colonists born at
	// dawn are tired by bedtime (born at midnight they collapse wherever
	// they are the first evening, before their sleep hours)
	g.SkipTo(day0, 6)
	g.SetGround(0, 0, 49, 29, "soil", "", "")
	gaps := map[[2]int]bool{}
	if gear.coolers {
		gaps[[2]int{16, 8}], gaps[[2]int{20, 8}] = true, true
	}
	for x := 10; x <= 26; x++ {
		for y := 8; y <= 14; y++ {
			wall := x == 10 || x == 18 || x == 26 || y == 8 || y == 14
			switch {
			case !wall || gaps[[2]int{x, y}]:
			case y == 14 && (x == 14 || x == 22):
				g.BuildNow("door", "wood", "south", x, y)
			default:
				g.BuildNow("wall", "granite blocks", "south", x, y)
			}
		}
	}
	build := func(kind, stuff string, x, y int) {
		if !g.BuildNow(kind, stuff, "south", x, y) {
			t.Fatalf("build %s at %d,%d", kind, x, y)
		}
	}
	build("wood-fired generator", "steel", 13, 4)
	g.FuelCampfire(13, 4)
	// a battery for the nights: haulers top the generator's 3.4-day tank
	// up only in work hours, so it carries the heaters through a gap
	build("battery", "steel", 19, 4)
	for x := 11; x <= 25; x++ {
		build("power conduit", "steel", x, 7)
	}
	for i := 0; i < 4; i++ {
		build("bed", "wood", 11+i*2, 12)
	}
	if gear.heaters {
		build("heater", "steel", 12, 9)
		build("heater", "steel", 24, 9)
	}
	if gear.coolers {
		build("cooler", "steel", 16, 8)
		build("cooler", "steel", 20, 8)
	}
	g.RoofRooms()
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 19, Y: 9, X2: 25, Y2: 13})
	put := func(k item.Kind, qty, x, y int) {
		if _, err := g.SpawnStack(k, qty, x, y); err != nil {
			t.Fatalf("stack %v at %d,%d: %v", k, x, y, err)
		}
	}
	for i := 0; i < 10; i++ {
		put(item.SurvivalMeal, 20, 19+i%7, 9+i/7)
	}
	for i := 0; i < 10; i++ {
		put(item.Wood, 75, 19+i%7, 12+i/7)
	}
	put(item.HerbalMedicine, 10, 25, 11)
	if gear.parkas {
		put(item.Parka, 4, 19, 11)
		put(item.Tuque, 4, 20, 11)
	}
	for i := 0; i < 4; i++ {
		if _, err := g.SpawnNamedPawn(11+i*2, 10, fmt.Sprintf("Probe %d", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

// tempRun is one probe's tally.
type tempRun struct {
	deaths       map[string]string // name -> "day N: cause"
	alive        int
	lowRoom      int // coldest bedroom air seen at the hour marks, tenths
	roomHours    int // hour marks counted in the bedroom range
	warmHours    int // of those, the hours at 15 C or more
	highRoom     int
	lowOut       int
	highOut      int
	worstHypo    int // permille, any colonist
	worstHeat    int
	frostbitten  int
	exposureDead int
}

// runTempColony steps g from day0 for days, logging each day and
// collecting deaths with their causes; days run 06:00 to 06:00. The room
// range skips the first six hours (the heaters and coolers bring the new
// rooms round).
func runTempColony(t *testing.T, g *game.Game, label string, day0, days int, onDay func(day int)) tempRun {
	t.Helper()
	run := tempRun{deaths: map[string]string{}, lowRoom: 1 << 20, highRoom: -1 << 20, lowOut: 1 << 20, highOut: -1 << 20}
	for d := day0; d < day0+days; d++ {
		if onDay != nil {
			onDay(d)
		}
		for h := 0; h < 24; h++ {
			g.Step(clock.TicksPerHour)
			bed, out := g.TemperatureAt(14, 11), g.TemperatureAt(40, 20)
			if d > day0 || h >= 6 {
				run.lowRoom, run.highRoom = min(run.lowRoom, bed), max(run.highRoom, bed)
				run.roomHours++
				if bed >= 150 {
					run.warmHours++
				}
			}
			run.lowOut, run.highOut = min(run.lowOut, out), max(run.highOut, out)
			for _, p := range g.Snapshot() {
				if p.Hostile {
					continue
				}
				run.worstHypo, run.worstHeat = max(run.worstHypo, p.Hypothermia), max(run.worstHeat, p.Heatstroke)
				if p.Status == "dead" && run.deaths[p.Name] == "" {
					run.deaths[p.Name] = fmt.Sprintf("day %d: %s", d, p.Reason)
					if strings.Contains(p.Reason, "hypothermia") || strings.Contains(p.Reason, "heatstroke") || strings.Contains(p.Reason, "froze") {
						run.exposureDead++
					}
				}
			}
		}
		var cols []string
		for _, p := range g.Snapshot() {
			if !p.Hostile {
				st := p.Status
				if p.Reason != "" {
					st += "(" + p.Reason + ")"
				}
				cols = append(cols, fmt.Sprintf("%s:%s/hypo%d/heat%d/frost%d@%.1fC", strings.TrimPrefix(p.Name, "Probe "), st, p.Hypothermia, p.Heatstroke, p.Frostbite, float64(p.Temp)/10))
			}
		}
		w := g.World()
		t.Logf("%s day %2d %-6s %-9s out %5.1fC bed %5.1fC store %5.1fC | %s", label, d, w.Season, w.ClimateEvent,
			float64(g.TemperatureAt(40, 20))/10, float64(g.TemperatureAt(14, 11))/10, float64(g.TemperatureAt(22, 11))/10, strings.Join(cols, " "))
	}
	for _, p := range g.Snapshot() {
		if !p.Hostile {
			if p.Status != "dead" {
				run.alive++
			}
			run.frostbitten += min(p.Frostbite, 1)
		}
	}
	names := make([]string, 0, len(run.deaths))
	for n := range run.deaths {
		names = append(names, n+" ("+run.deaths[n]+")")
	}
	sort.Strings(names)
	t.Logf("%s: alive %d/4, bedroom %.1f..%.1fC, outside %.1f..%.1fC, worst hypothermia %d‰ heatstroke %d‰, frostbitten %d, deaths %v",
		label, run.alive, float64(run.lowRoom)/10, float64(run.highRoom)/10, float64(run.lowOut)/10, float64(run.highOut)/10,
		run.worstHypo, run.worstHeat, run.frostbitten, names)
	return run
}

// TestTemperatureProbeTemperateWinter: a temperate colony with heaters,
// parkas and tuques lives through the depth of winter (days 38-52, the
// coldest at day 44, nights near -27 C); the same colony bare (no
// heaters, no warm clothes) is logged beside it.
func TestTemperatureProbeTemperateWinter(t *testing.T) {
	if os.Getenv("RIMWORLD_PROBE") == "" {
		t.Skip("set RIMWORLD_PROBE=1")
	}
	warm := runTempColony(t, tempColony(t, biome.TemperateForest, 21, tempGear{heaters: true, parkas: true}, 38), "winter/heated", 38, 15, nil)
	runTempColony(t, tempColony(t, biome.TemperateForest, 21, tempGear{}, 38), "winter/bare", 38, 15, nil)
	if warm.exposureDead > 0 || warm.alive < 4 {
		t.Errorf("the heated, dressed colony lost colonists to the winter: %v", warm.deaths)
	}
	// a door held open to -20 C air while colonists haul through it dips
	// the room for an hour (open doors exchange heat; the heaters bring it
	// back), so the heated room must stay warm nearly always and never
	// near freezing
	if warm.lowRoom < 50 {
		t.Errorf("the heated bedroom fell to %.1f C", float64(warm.lowRoom)/10)
	}
	if warm.warmHours*100 < warm.roomHours*95 {
		t.Errorf("the heated bedroom was at 15 C or more for only %d of %d hours", warm.warmHours, warm.roomHours)
	}
}

// TestTemperatureProbeDesertSummer: a desert colony with coolers lives
// through midsummer (days 7-21, the hottest at day 14) and a heat wave
// (+17 C, fired on day 12); the same colony without coolers is logged
// beside it.
func TestTemperatureProbeDesertSummer(t *testing.T) {
	if os.Getenv("RIMWORLD_PROBE") == "" {
		t.Skip("set RIMWORLD_PROBE=1")
	}
	wave := func(g *game.Game) func(int) {
		return func(day int) {
			if day == 12 && !g.FireIncident("heat_wave") {
				t.Error("the heat wave did not fire")
			}
		}
	}
	g := tempColony(t, biome.Desert, 22, tempGear{coolers: true}, 7)
	cool := runTempColony(t, g, "summer/cooled", 7, 15, wave(g))
	h := tempColony(t, biome.Desert, 22, tempGear{}, 7)
	runTempColony(t, h, "summer/bare", 7, 15, wave(h))
	if cool.exposureDead > 0 || cool.alive < 4 {
		t.Errorf("the cooled colony lost colonists to the summer: %v", cool.deaths)
	}
	if cool.highRoom > 360 {
		t.Errorf("the cooled bedroom rose to %.1f C", float64(cool.highRoom)/10)
	}
}

// TestTemperatureProbeFreezer: a 5x5 storeroom with a cooler set to
// -5 C keeps raw meat frozen through ten temperate summer days, while the
// same meat outside spoils. The generator is refuelled by hand every hour
// (no colonists).
func TestTemperatureProbeFreezer(t *testing.T) {
	if os.Getenv("RIMWORLD_PROBE") == "" {
		t.Skip("set RIMWORLD_PROBE=1")
	}
	g := game.New(game.Config{Seed: 23, Width: 30, Height: 20})
	g.SkipTo(20, 0)
	g.SetGround(0, 0, 29, 19, "soil", "", "")
	for x := 8; x <= 14; x++ {
		for y := 6; y <= 12; y++ {
			if (x == 8 || x == 14 || y == 6 || y == 12) && !(x == 11 && y == 6) {
				g.BuildNow("wall", "granite blocks", "south", x, y)
			}
		}
	}
	for _, b := range []struct {
		kind string
		x, y int
	}{{"cooler", 11, 6}, {"wood-fired generator", 11, 2}, {"power conduit", 10, 4}, {"power conduit", 11, 4}, {"power conduit", 12, 4}, {"power conduit", 11, 5}} {
		if !g.BuildNow(b.kind, "steel", "south", b.x, b.y) {
			t.Fatalf("build %s", b.kind)
		}
	}
	g.RoofRooms()
	for _, v := range g.World().Buildings {
		if v.Kind == "cooler" {
			g.Enqueue(command.Command{Kind: command.SetThermostat, Target: uint64(v.ID), Count: -50})
		}
	}
	in, err := g.SpawnStack(item.Meat, 30, 10, 9)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := g.SpawnStack(item.Meat, 30, 22, 9)
	low, high, outHigh := 1<<20, -1<<20, -1<<20
	for d := 20; d < 30; d++ {
		for h := 0; h < 24; h++ {
			g.FuelCampfire(11, 2) // a tank lasts under a day
			g.Step(clock.TicksPerHour)
			r := g.TemperatureAt(11, 9)
			if d > 20 {
				low, high = min(low, r), max(high, r)
			}
			outHigh = max(outHigh, g.TemperatureAt(22, 9))
		}
		fin, fout := "gone", "gone"
		for _, v := range g.World().Items {
			if v.ID == in {
				fin = fmt.Sprintf("%.1fd rot %d‰", v.FreshDays, v.RotRate)
			}
			if v.ID == out {
				fout = fmt.Sprintf("%.1fd rot %d‰", v.FreshDays, v.RotRate)
			}
		}
		t.Logf("freezer day %d out %5.1fC freezer %5.1fC | meat inside %s, outside %s", d, float64(g.TemperatureAt(22, 9))/10, float64(g.TemperatureAt(11, 9))/10, fin, fout)
	}
	t.Logf("freezer: air %.1f..%.1fC from day 21, outside up to %.1fC", float64(low)/10, float64(high)/10, float64(outHigh)/10)
	var inside *game.ItemView
	for _, v := range g.World().Items {
		if v.ID == in {
			inside = &v
		}
	}
	if inside == nil || inside.Rotten || inside.RotRate != 0 || high > 0 {
		t.Errorf("the freezer failed: meat %+v, air up to %.1f C", inside, float64(high)/10)
	}
}
