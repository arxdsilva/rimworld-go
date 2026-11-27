// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"slices"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
)

// The threat die's faces (incident.Book.PickThreat over core's defs): a
// predator pack in winter, a raid outside winter from day 8, the mad
// animal's and the hive swarm's.
const (
	packRoll = 0
	raidRoll = 3
	madRoll  = 8 - 2
	hiveRoll = 8 - 1
)

// threatColony is a landed setup colony of seed (edit adjusts its Config,
// whose Setup is set) whose clock stands at noon of colony day day in
// season (the landing moved so: startDay is never 0), and that tick.
func threatColony(t *testing.T, seed uint64, season string, day int, edit func(*Config)) (*Game, uint64) {
	t.Helper()
	cfg := Config{Setup: setupFor(t, "ember_clan", func(s *Setup) { s.Seed = seed })}
	if edit != nil {
		edit(&cfg)
	}
	b := NewColonyBuilder(cfg)
	for done := false; !done; {
		_, _, done = b.Step(1 << 20)
	}
	g := b.Game()
	stepUntilLanded(t, g)
	for d := day + 1; d < day+121; d++ {
		if g.season(uint64(d)*clock.TicksPerDay) == season {
			now := uint64(d)*clock.TicksPerDay + clock.TicksPerDay/2
			g.ng.startDay = d - day
			g.clock = clock.NewAt(clock.Tick(now))
			return g, now
		}
	}
	t.Fatalf("no %s day", season)
	return nil, 0
}

// enrich makes g a rich Merciless colony: threat points past both the
// mechanoids' and the ship part's minimums.
func enrich(t *testing.T, g *Game) {
	t.Helper()
	g.ng.setup.Preset, g.ng.setup.Difficulty = difficulty.Merciless, difficulty.Of(difficulty.Merciless)
	if _, err := g.SpawnStack(item.Silver, 50000, 0, 0); err != nil {
		t.Fatal(err)
	}
	if p := g.threatPoints(); p < incident.DefOf(incident.CrashedShipPart).MinPoints {
		t.Fatalf("threat points %d", p)
	}
}

// threatBook is core's incidents with edit applied to the defs.
func threatBook(edit func(list []incident.Def)) incident.Book {
	core := incident.Core()
	list := core.All()
	edit(list)
	return incident.NewBook(list, core.Keys())
}

// The raid face's split, forced: people only, mechanoids almost surely,
// a ship part almost surely.
var (
	peopleBook = threatBook(func(l []incident.Def) { l[incident.CrashedShipPart].Weight, l[incident.MechRaid].Weight = 0, 0 })
	mechBook   = threatBook(func(l []incident.Def) { l[incident.CrashedShipPart].Weight, l[incident.MechRaid].Weight = 0, 1_000_000 })
	shipBook   = threatBook(func(l []incident.Def) { l[incident.CrashedShipPart].Weight = 1_000_000 })
)

func withBook(b incident.Book) func(*Config) {
	return func(c *Config) { c.Content.Incidents = b }
}

// threatCase is one storyteller threat of a known kind: the book, the
// season, the roll, a rich colony; the event recorded last and the
// on_incident key it reports.
type threatCase struct {
	name   string
	book   incident.Book
	winter bool
	roll   int
	rich   bool
	want   incident.ID
	key    string
}

var threatCases = []threatCase{
	{"pack", incident.Book{}, true, packRoll, false, incident.PredatorPack, "core.predator_pack"},
	{"people raid", peopleBook, false, raidRoll, false, incident.Raid, "core.rival_raid"},
	{"mech raid", mechBook, false, raidRoll, true, incident.MechRaid, "core.mech_raid"},
	{"ship part", shipBook, false, raidRoll, true, incident.CrashedShipPart, "core.crashed_ship_part"},
}

// threatCaseColony is c's colony at colony day 20, and that tick.
func threatCaseColony(t *testing.T, c threatCase, edit func(*Config)) (*Game, uint64) {
	t.Helper()
	season := "Summer"
	if c.winter {
		season = "Winter"
	}
	g, now := threatColony(t, 3, season, 20, func(cfg *Config) {
		cfg.Content.Incidents = c.book
		if edit != nil {
			edit(cfg)
		}
	})
	if c.rich {
		enrich(t, g)
	}
	return g, now
}

// The pin's rich colony reaches the raid face's mechanoid and ship part
// branches (so the pin holds them too), for every pinned seed.
func TestStoryThreatPinReachesSplit(t *testing.T) {
	t.Parallel()
	rich := pinThreatCases[len(pinThreatCases)-1]
	if !rich.rich {
		t.Fatal("the last pinned case is not the rich one")
	}
	for seed := uint64(1); seed <= 3; seed++ {
		if _, _, _, titles := pinThreatRun(t, seed, rich); titles["Mechanoid raid"] == 0 || titles["Crashed ship part"] == 0 {
			t.Fatalf("seed %d: letters %v", seed, titles)
		}
	}
}

// A storyteller threat is recorded in State.Last and is the overlay's
// last threat: a pack, a people raid, a mech raid (over the raid recorded
// with it) and a ship part (likewise).
func TestStoryThreatRecordsLast(t *testing.T) {
	t.Parallel()
	for _, c := range threatCases {
		g, now := threatCaseColony(t, c, nil)
		rn := g.rel(now)
		if _, _, ok := g.sk.content.Incidents.LastFired(g.incidents, incident.Threat); ok {
			t.Fatalf("%s: a threat before the test", c.name)
		}
		g.storyThreat(c.roll, 20, c.winter, 100, now)
		if at, ok := g.incidents.Last[c.want]; !ok || at != rn {
			t.Fatalf("%s: Last %v, want %d at %d", c.name, g.incidents.Last, c.want, rn)
		}
		if c.want != incident.PredatorPack && g.incidents.Last[incident.Raid] != rn {
			t.Fatalf("%s: the raid face not recorded: %v", c.name, g.incidents.Last)
		}
		if v := g.StoryDebug(); v.LastThreat.ID != c.want || v.LastThreatAt != rn || v.LastThreat.Key == "" {
			t.Fatalf("%s: overlay's last threat %v at %d", c.name, v.LastThreat.Key, v.LastThreatAt)
		}
	}
}

// Each storyteller threat reports one on_incident: a people raid as a
// rival raid on arrival, the others under their own keys.
func TestStoryThreatReportsOnce(t *testing.T) {
	t.Parallel()
	for _, c := range threatCases {
		f := &fakeScripts{}
		g, now := threatCaseColony(t, c, func(cfg *Config) { cfg.Scripts = f })
		f.events = nil
		g.storyThreat(c.roll, 20, c.winter, 100, now)
		var got []scripting.Incident
		for _, ev := range f.events {
			if ev.Hook == scripting.OnIncident {
				got = append(got, ev.Incident)
			}
		}
		if len(got) != 1 || got[0] != (scripting.Incident{Key: c.key, Category: "threat"}) {
			t.Fatalf("%s: on_incident %+v, want one %s", c.name, got, c.key)
		}
	}
}

// The last threat and its tick survive a save and load.
func TestStoryThreatSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g, now := threatColony(t, 3, "Winter", 20, nil)
	g.storyThreat(packRoll, 20, true, 100, now)
	var buf bytes.Buffer
	if err := g.Save(&buf, "threat"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	want, got := g.StoryDebug(), l.StoryDebug()
	if want.LastThreat.ID != incident.PredatorPack || got.LastThreat.ID != want.LastThreat.ID || got.LastThreatAt != want.LastThreatAt ||
		l.incidents.Last[incident.PredatorPack] != g.incidents.Last[incident.PredatorPack] {
		t.Fatalf("last threat %v at %d, want %v at %d", got.LastThreat.Key, got.LastThreatAt, want.LastThreat.Key, want.LastThreatAt)
	}
}

// Saves from before the threat events existed: one whose key table holds
// core's first 30 incidents loads and fires the next storyteller threat;
// one with a plugin incident at the old ID 30 keeps its cooldown by key,
// past the new threat events.
func TestOldSaveFiresThreats(t *testing.T) {
	t.Parallel()
	core := incident.Core()
	old := incident.NewBook(core.All()[:30], core.Keys()[:30])
	g, now := threatColony(t, 4, "Summer", 20, withBook(old))
	g.incidents.Fired(incident.MadAnimal, 5)
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if l.incidents.Last[incident.MadAnimal] != 5 {
		t.Fatalf("old record lost: %v", l.incidents.Last)
	}
	fired := 0
	for i := uint64(0); i < 5; i++ {
		at := now + i*clock.TicksPerDay
		l.clock = clock.NewAt(clock.Tick(at))
		l.fireStoryThreat(at)
		if _, when, ok := l.sk.content.Incidents.LastFired(l.incidents, incident.Threat); ok && when == l.rel(at) {
			fired++
		}
	}
	if fired == 0 {
		t.Fatal("the loaded old save fired no storyteller threat")
	}
	// a plugin incident saved at the old ID 30
	plugin := incident.Def{Key: "frontier.dark_noon", Name: "Dark noon", Category: incident.Nature, Weight: 5, Cooldown: 7, Like: incident.Eclipse}
	oldPlug := incident.NewBook(append(core.All()[:30], plugin), append(core.Keys()[:30], "frontier.dark_noon"))
	p := New(Config{Seed: 3, Width: 20, Height: 20, Content: Content{Incidents: oldPlug}})
	p.incidents.Fired(30, 1234)
	buf.Reset()
	if err := p.Save(&buf, "plugin"); err != nil {
		t.Fatal(err)
	}
	today := incidentBookWith(false, "frontier.dark_noon")
	id, _ := today.IDOf("frontier.dark_noon")
	lp, _, err := LoadContent(bytes.NewReader(buf.Bytes()), Content{Incidents: today})
	if err != nil {
		t.Fatal(err)
	}
	if _, hive := lp.incidents.Last[incident.HiveSwarm]; id < incident.NumIDs || lp.incidents.Last[id] != 1234 || hive {
		t.Fatalf("plugin at %d: Last %v", id, lp.incidents.Last)
	}
}

// The same seed fires the same storyteller threats.
func TestStoryThreatDeterministic(t *testing.T) {
	t.Parallel()
	run := func() []incident.ID {
		g, now := threatColony(t, 5, "Summer", 20, nil)
		var ids []incident.ID
		for i := uint64(0); i < 20; i++ {
			at := now + i*clock.TicksPerDay
			g.clock = clock.NewAt(clock.Tick(at))
			g.fireStoryThreat(at)
			if d, when, ok := g.sk.content.Incidents.LastFired(g.incidents, incident.Threat); ok && when == g.rel(at) {
				ids = append(ids, d.ID)
			}
		}
		return ids
	}
	a, b := run(), run()
	if len(a) < 10 || !slices.Equal(a, b) {
		t.Fatalf("threats %v and %v", a, b)
	}
}

// A mad animal face inside its cooldown sends a people raid when raids are
// allowed, and nothing when they are not (as the die always did).
func TestMadFaceCoolingFallsBack(t *testing.T) {
	t.Parallel()
	for _, raids := range []bool{true, false} {
		g, now := threatColony(t, 2, "Summer", 20, nil)
		if !raids {
			hostileOnly(g.ng.factions, -1)
		} else if !hostileRaider(g) {
			t.Fatal("no hostile raider faction")
		}
		cooling := g.rel(now) - clock.TicksPerDay // fired yesterday: 10 days' cooldown
		g.incidents.Fired(incident.MadAnimal, cooling)
		before := len(g.raidGroups)
		g.storyThreat(madRoll, 20, false, 100, now)
		if got := len(g.raidGroups) - before; raids && got != 1 || !raids && got != 0 {
			t.Fatalf("raids allowed %v: %d raid groups sent", raids, got)
		}
		if g.incidents.Last[incident.MadAnimal] != cooling {
			t.Fatalf("raids allowed %v: the cooling mad animal fired", raids)
		}
	}
}

// A fallback people raid is recorded as a raid: a hive swarm with no hive
// spot, a cooling mad animal while raids are allowed.
func TestFallbackRaidRecorded(t *testing.T) {
	t.Parallel()
	for _, roll := range []int{hiveRoll, madRoll} {
		g, now := threatColony(t, 2, "Summer", 20, nil)
		g.climate.YearMean = -600 // a freezing map: no hive spot
		g.incidents.Fired(incident.MadAnimal, g.rel(now)-clock.TicksPerDay)
		before := len(g.raidGroups)
		g.storyThreat(roll, 20, false, 100, now)
		if _, hive := g.incidents.Last[incident.HiveSwarm]; len(g.raidGroups) != before+1 || g.incidents.Last[incident.Raid] != g.rel(now) || hive {
			t.Fatalf("roll %d: %d raid groups, Last %v", roll, len(g.raidGroups)-before, g.incidents.Last)
		}
		if v := g.StoryDebug(); v.LastThreat.ID != incident.Raid {
			t.Fatalf("roll %d: overlay's last threat %s", roll, v.LastThreat.Key)
		}
	}
}

// A people raid of tp*pct/100 points at 100% is the raid of tp points at
// pct%: the storyteller's raid may be sized before it is sent.
func TestScaleRaidPointsIdentity(t *testing.T) {
	t.Parallel()
	for tp := 0; tp <= 3*difficulty.PointsCap; tp += 7 {
		for pct := 1; pct <= 300; pct++ {
			if a, b := difficulty.ScaleRaidPoints(tp*pct/100, 100), difficulty.ScaleRaidPoints(tp, pct); a != b {
				t.Fatalf("tp %d pct %d: %d != %d", tp, pct, a, b)
			}
		}
	}
}

// The ship part's gate reads the day's unscaled raid points: a raid face
// whose share is below the part's minimum still crashes one.
func TestShipPartGateUnscaled(t *testing.T) {
	t.Parallel()
	g, now := threatColony(t, 3, "Summer", 20, withBook(shipBook))
	enrich(t, g)
	tp := g.threatPoints()
	pct := 300 * 100 / tp // the face's points: about 300, under the part's 400
	if tp*pct/100 >= incident.DefOf(incident.CrashedShipPart).MinPoints {
		t.Fatalf("tp %d pct %d", tp, pct)
	}
	g.storyThreat(raidRoll, 20, false, pct, now)
	if g.raid.Ships.Parts() != 1 || g.incidents.Last[incident.CrashedShipPart] != g.rel(now) {
		t.Fatalf("tp %d at %d%%: %d ship parts", tp, pct, g.raid.Ships.Parts())
	}
}

// A threat def's share of the points reaches its executor: a predator pack
// at PointsPct 50 is the pack half the points buy; at 0 the full pack.
func TestThreatSpendReachesExecutor(t *testing.T) {
	t.Parallel()
	half := threatBook(func(l []incident.Def) { l[incident.PredatorPack].PointsPct = 50 })
	sizes := map[int]int{}
	for _, pct := range []int{50, 0} {
		b := incident.Book{}
		if pct == 50 {
			b = half
		}
		g, now := threatColony(t, 3, "Winter", 20, withBook(b))
		enrich(t, g)
		pts := g.threatPoints()
		before := len(g.animalIDs)
		g.storyThreat(packRoll, 20, true, 100, now)
		pack := g.animalIDs[before:]
		if len(pack) == 0 {
			t.Fatalf("pct %d: no pack", pct)
		}
		a, _ := g.animals.Get(pack[0])
		want := max(animal.PackSize(pts, a.Species), 2)
		if pct == 50 {
			want = max(animal.PackSize(pts/2, a.Species), 2)
		}
		if len(pack) != want {
			t.Fatalf("pct %d: a pack of %d at %d points, want %d", pct, len(pack), pts, want)
		}
		sizes[pct] = want
	}
	if sizes[50] >= sizes[0] {
		t.Fatalf("the half share did not shrink the pack: %v", sizes)
	}
}

// The dev hook fires a storyteller threat event of a setup colony at
// today's points, only while its flags allow it; a legacy colony keeps its
// own die and refuses them (as before they were events).
func TestThreatDevHookFits(t *testing.T) {
	t.Parallel()
	g, now := threatColony(t, 3, "Winter", 20, nil)
	before := len(g.animalIDs)
	if !g.FireIncident("predator_pack") || len(g.animalIDs) == before || g.incidents.Last[incident.PredatorPack] != g.rel(now) {
		t.Fatalf("the pack did not come: Last %v", g.incidents.Last)
	}
	g.ng.setup.Difficulty.PredatorsHuntColonists = false
	if g.FireIncident("predator_pack") {
		t.Fatal("a pack the difficulty rules out came")
	}
	legacy, _ := raidGame(t)
	for _, k := range []string{"predator_pack", "manhunter_pack", "hive_swarm", "raid"} {
		if legacy.FireIncident(k) {
			t.Fatalf("a legacy colony fired %s", k)
		}
	}
}

// A mechanoid raid of the raid face spends its own def's share (N2): at
// MechRaid.PointsPct 50 the group costs at most half the points.
func TestMechRaidSpendsItsShare(t *testing.T) {
	t.Parallel()
	half := threatBook(func(l []incident.Def) {
		l[incident.CrashedShipPart].Weight, l[incident.MechRaid].Weight, l[incident.MechRaid].PointsPct = 0, 1_000_000, 50
	})
	cost := map[int]int{}
	for _, pct := range []int{50, 0} {
		b := mechBook
		if pct == 50 {
			b = half
		}
		g, now := threatColony(t, 3, "Summer", 20, withBook(b))
		enrich(t, g)
		pts := g.threatPoints()
		before := len(g.animalIDs)
		g.storyThreat(raidRoll, 20, false, 100, now)
		if g.incidents.Last[incident.MechRaid] != g.rel(now) {
			t.Fatalf("pct %d: no mech raid", pct)
		}
		var kinds []animal.Species
		if s := g.raid.Mechs.Export(); s != nil {
			for _, p := range s.Pods {
				kinds = append(kinds, p.Mech)
			}
		}
		for _, id := range g.animalIDs[before:] {
			a, _ := g.animals.Get(id)
			kinds = append(kinds, a.Species)
		}
		for _, k := range kinds {
			m, _ := animal.MechOf(k)
			cost[pct] += m.CombatPower
		}
		if pct == 50 && cost[pct] > pts/2 {
			t.Fatalf("half share: the mechanoids cost %d of %d points", cost[pct], pts)
		}
	}
	if cost[50] >= cost[0] {
		t.Fatalf("the half share did not shrink the group: %v", cost)
	}
}
