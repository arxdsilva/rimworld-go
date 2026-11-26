// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"maps"
	"reflect"
	"slices"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
)

// threatColony is a landed setup colony (Seasoned) whose only hostile
// faction is the clans.
func threatColony(t *testing.T) *Game {
	t.Helper()
	g := buildSetup(t, setupFor(t, "broken_hull", nil))
	stepUntilLanded(t, g)
	hostileOnly(g.ng.factions, slices.IndexFunc(g.ng.factions, func(f faction.Faction) bool { return f.Type == faction.Clans }))
	return g
}

// threatAt sets the clock to noon of colony day day and returns the tick.
func threatAt(g *Game, day int) uint64 {
	now := g.dayOff() + uint64(day)*clock.TicksPerDay + 12*clock.TicksPerHour
	g.clock = clock.NewAt(clock.Tick(now))
	return now
}

// threatFaces sweeps the setup colony's threat pick at colony day day
// over [0, Total), with the flags storyThreat asks (threatOK): each face's
// share and the total.
func threatFaces(g *Game, day int, winter bool) (map[incident.ID]int, int) {
	book := g.sk.content.Incidents
	rn := uint64(day) * clock.TicksPerDay
	o := incident.Odds{Disease: incident.FullFreq, Climate: incident.FullFreq, Points: g.threatPoints(), Winter: winter}
	n := book.Total(incident.Threat, g.incidents, rn, g.biome, o, g.threatOK)
	out := map[incident.ID]int{}
	for r := range n {
		if id, ok := book.Pick(incident.Threat, g.incidents, rn, g.biome, o, r, g.threatOK); ok {
			out[id]++
		}
	}
	return out, n
}

// threatBook gives g a core book whose threat defs f patched (a test
// forcing one face); the other books stay core.
func threatBook(g *Game, f func(d *incident.Def)) {
	defs := incident.Core().All()
	for i := range defs {
		if defs[i].Category == incident.Threat {
			f(&defs[i])
		}
	}
	g.sk.content.Incidents = incident.NewBook(defs, incident.Core().Keys())
}

// only keeps the threat faces in ids at their weights (the raid, the
// filler, then weighs what they leave of its Fill, here nothing) and
// zeroes the rest; the ship part and mechs come with no point minimum.
func only(ids ...incident.ID) func(d *incident.Def) {
	return func(d *incident.Def) {
		switch {
		case d.ID == incident.Raid && slices.Contains(ids, incident.Raid):
			d.Fill = incident.ThreatRolls
		case d.ID == incident.Raid: // a filler of nothing, and no split weight either
			d.Fill, d.Weight = 0, 0
			for _, id := range ids {
				d.Fill += incident.DefOf(id).Weight
			}
		case d.UnderRaid:
			d.MinPoints = 0
		case !slices.Contains(ids, d.ID):
			d.Weight = 0
		}
		d.Season = incident.AnySeason
	}
}

func rngState(t *testing.T, g *Game) []byte {
	t.Helper()
	b, err := g.rng.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// firedAt is whether some def fired at colony tick rn.
func firedAt(g *Game, rn uint64) bool {
	return slices.Contains(slices.Collect(maps.Values(g.incidents.Last)), rn)
}

// TestStoryThreatDeterministic (C1): a setup colony's threats are picked
// through the incident book, one g.rng draw a threat (none when nothing is
// allowed), and the same seed gives the same threats.
func TestStoryThreatDeterministic(t *testing.T) {
	t.Parallel()
	a, b := threatColony(t), threatColony(t)
	for _, g := range []*Game{a, b} {
		for i := range 12 {
			now := threatAt(g, 6+4*i)
			g.storyThreat(100, now)
			if !firedAt(g, g.rel(now)) {
				t.Fatalf("day %d: an allowed threat fired nothing: %v", 6+4*i, g.incidents.Last)
			}
		}
	}
	if !maps.Equal(a.incidents.Last, b.incidents.Last) || len(a.raiderIDs) != len(b.raiderIDs) || len(a.animalIDs) != len(b.animalIDs) ||
		!bytes.Equal(rngState(t, a), rngState(t, b)) {
		t.Fatalf("same seed, other threats: %v / %v, %d / %d raiders", a.incidents.Last, b.incidents.Last, len(a.raiderIDs), len(b.raiderIDs))
	}
	if _, ok := a.incidents.Last[incident.Raid]; !ok {
		t.Fatalf("twelve threats and no raid face: %v", a.incidents.Last)
	}
	// nothing allowed: no draw
	for _, g := range []*Game{a, b} {
		hostileOnly(g.ng.factions, -1)
		threatBook(g, only())
	}
	now := threatAt(a, 70)
	before, last := rngState(t, a), maps.Clone(a.incidents.Last)
	a.storyThreat(100, now)
	if !bytes.Equal(before, rngState(t, a)) || !maps.Equal(last, a.incidents.Last) {
		t.Fatal("a threat with nothing allowed drew or fired")
	}
	// one face of weight 1: the pick is one IntN(1), then the face's own draws
	threatBook(a, only(incident.MadAnimal))
	threatBook(b, only(incident.MadAnimal))
	now = threatAt(b, 70)
	b.rng.IntN(1)
	if b.fireIncident(incident.MadAnimal, 0, now) {
		b.incidentFired(incident.MadAnimal, now)
	}
	a.storyThreat(100, now)
	if !bytes.Equal(rngState(t, a), rngState(t, b)) || !maps.Equal(a.incidents.Last, b.incidents.Last) {
		t.Fatalf("the pick is not one draw: %v / %v", a.incidents.Last, b.incidents.Last)
	}
}

// TestThreatCooldownSaveRoundTrip (C2): every fired face, and the ship
// part or mechs a raid face became, record their own cooldown entry; a
// save keeps them and a save without them loads as never fired.
func TestThreatCooldownSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g := threatColony(t)
	threatBook(g, only(incident.Raid))
	for day := 20; day < 80 && (!hasLast(g, incident.CrashedShipPart) || !hasLast(g, incident.MechRaid)); day++ {
		now := threatAt(g, day)
		g.storyThreat(100, now)
		if g.incidents.Last[incident.Raid] != g.rel(now) {
			t.Fatalf("day %d: the raid face was not recorded: %v", day, g.incidents.Last)
		}
	}
	if !hasLast(g, incident.CrashedShipPart) || !hasLast(g, incident.MechRaid) {
		t.Fatalf("no ship part or mechs from the raid face: %v", g.incidents.Last)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	l, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(l.incidents.Last, g.incidents.Last) {
		t.Fatalf("loaded %v, want %v", l.incidents.Last, g.incidents.Last)
	}
	// a save from before the threat entries: the ship part never fired
	rn := g.incidents.Last[incident.CrashedShipPart] + clock.TicksPerDay
	ship := incident.DefOf(incident.CrashedShipPart)
	if ship.Allowed(g.incidents, rn, g.biome) {
		t.Fatal("the ship part cools down a day after it came")
	}
	for _, id := range []incident.ID{incident.Raid, incident.CrashedShipPart, incident.MechRaid} {
		delete(g.incidents.Last, id)
	}
	buf.Reset()
	if err := g.Save(&buf, "t"); err != nil {
		t.Fatal(err)
	}
	if l, _, err = Load(&buf); err != nil {
		t.Fatal(err)
	}
	if hasLast(l, incident.CrashedShipPart) || hasLast(l, incident.MechRaid) || !ship.Allowed(l.incidents, rn, l.biome) {
		t.Fatalf("an older save's ship part is cooling down: %v", l.incidents.Last)
	}
}

func hasLast(g *Game, id incident.ID) bool {
	_, ok := g.incidents.Last[id]
	return ok
}

// TestStoryThreatFallbacks (C4): a hive with no spot or a mad animal with
// no candidate is a people raid when a raid is allowed (nothing when it is
// not); a ship part drawn with fewer points spent than its minimum is a
// people raid.
func TestStoryThreatFallbacks(t *testing.T) {
	t.Parallel()
	g := threatColony(t)
	g.climate.YearMean = -600 // freezing: no hive spot (infest_test.go)
	raided := func(day int, id incident.ID, want bool) {
		t.Helper()
		g.incidents.Last = nil
		groups := len(g.raidGroups)
		now := threatAt(g, day)
		g.storyThreat(100, now)
		if hasLast(g, id) || (len(g.raidGroups) > groups) != want || hasLast(g, incident.Raid) != want {
			t.Fatalf("day %d %s: raid %v want %v, %v", day, incident.DefOf(id).Key, len(g.raidGroups) > groups, want, g.incidents.Last)
		}
	}
	threatBook(g, only(incident.HiveSwarm))
	raided(30, incident.HiveSwarm, true)
	hostileOnly(g.ng.factions, -1)
	raided(31, incident.HiveSwarm, false)
	hostileOnly(g.ng.factions, slices.IndexFunc(g.ng.factions, func(f faction.Faction) bool { return f.Type == faction.Clans }))
	for _, id := range g.animalIDs { // no wild animal to go mad
		setAnimal(g, id, func(a *animal.State) { a.Leaving = true })
	}
	threatBook(g, only(incident.MadAnimal))
	raided(32, incident.MadAnimal, true)
	// the ship part below its minimum of the points spent
	threatBook(g, func(d *incident.Def) {
		only(incident.Raid)(d)
		if d.ID == incident.Raid {
			d.Weight = 0 // the split always draws the ship part
		}
	})
	ship := g.sk.content.Incidents.DefOf(incident.CrashedShipPart)
	ship.MinPoints = 400
	defs := g.sk.content.Incidents.All()
	defs[incident.CrashedShipPart] = ship
	g.sk.content.Incidents = incident.NewBook(defs, incident.Core().Keys())
	pct := max(1, 399*100/max(g.threatPoints(), 1))
	if g.threatPoints()*pct/100 >= ship.MinPoints {
		t.Fatalf("pct %d of %d points", pct, g.threatPoints())
	}
	g.incidents.Last = nil
	groups, now := len(g.raidGroups), threatAt(g, 33)
	g.storyThreat(pct, now)
	if hasLast(g, incident.CrashedShipPart) || g.raid.Ships.Parts() != 0 || len(g.raidGroups) == groups {
		t.Fatalf("a ship part below its points: %v, %d parts", g.incidents.Last, g.raid.Ships.Parts())
	}
}

// TestThreatSpendsBudget (C5): a face spends its share of the pick's
// points (Spend of threat points times the plan's pct); the dev hook
// accepts the faces; a queued face spends the points it was queued with.
func TestThreatSpendsBudget(t *testing.T) {
	t.Parallel()
	a, b := threatColony(t), threatColony(t)
	for i, id := range []incident.ID{incident.PredatorPack, incident.ManhunterPack} {
		for _, g := range []*Game{a, b} {
			threatBook(g, func(d *incident.Def) {
				only(id)(d)
				if d.ID == id {
					d.PointsPct = 50
				}
			})
		}
		now := threatAt(a, 20+i)
		threatAt(b, 20+i)
		pct := 800 * 100 / max(a.threatPoints(), 1) // about 800 points carried, 400 spent: packs of different sizes
		a.storyThreat(pct, now)
		b.rng.IntN(incident.DefOf(id).Weight)
		pts := b.threatPoints() * pct / 100 * 50 / 100
		if id == incident.PredatorPack {
			b.raid.Threats.WolfPack(pts)
		} else {
			rageOf(b).ManhunterPack(pts)
		}
		b.incidentFired(id, now)
		if len(a.animalIDs) != len(b.animalIDs) || !bytes.Equal(rngState(t, a), rngState(t, b)) || !maps.Equal(a.incidents.Last, b.incidents.Last) {
			t.Fatalf("%s: %d animals, want %d at %d points", incident.DefOf(id).Key, len(a.animalIDs), len(b.animalIDs), pts)
		}
	}
	// a queued pack spends its queued points
	now := threatAt(a, 30)
	threatAt(b, 30)
	a.fireQueued(incident.Queued{ID: incident.PredatorPack, Points: 777}, now)
	b.raid.Threats.WolfPack(777)
	b.incidentFired(incident.PredatorPack, now)
	if len(a.animalIDs) != len(b.animalIDs) || !bytes.Equal(rngState(t, a), rngState(t, b)) || !maps.Equal(a.incidents.Last, b.incidents.Last) {
		t.Fatalf("queued pack: %d animals, want %d", len(a.animalIDs), len(b.animalIDs))
	}
	// the dev hook and a nature plugin's like reach the faces, behind the flags
	g := a
	g.sk.content.Incidents = incident.Core()
	for _, key := range []string{"raid", "predator_pack", "manhunter_pack", "hive_swarm", "mech_raid"} {
		d, _ := g.sk.content.Incidents.ByName(key)
		if !g.incidentFits(d.ID, now) {
			t.Fatalf("%s does not fit a setup colony", key)
		}
	}
	if !g.FireIncident("raid") || !hasLast(g, incident.Raid) {
		t.Fatal("the dev hook's raid did not come")
	}
	hostileOnly(g.ng.factions, -1)
	g.ng.setup.Difficulty.PredatorsHuntColonists = false
	if g.incidentFits(incident.Raid, now) || g.incidentFits(incident.PredatorPack, now) {
		t.Fatal("a raid with no hostile faction, or a pack the difficulty forbids, fits")
	}
}

// TestStoryDebugLastThreat (C8): the overlay's last threat is the ship
// part a raid face became, at its absolute tick.
func TestStoryDebugLastThreat(t *testing.T) {
	t.Parallel()
	g := threatColony(t)
	g.incidents.Last = nil
	if v := g.StoryDebug(); v.LastThreat != (incident.Def{}) || v.LastThreatAt != 0 {
		t.Fatalf("no threat yet: %+v at %d", v.LastThreat, v.LastThreatAt)
	}
	threatBook(g, func(d *incident.Def) {
		only(incident.Raid)(d)
		if d.ID == incident.Raid {
			d.Weight = 0
		}
	})
	now := threatAt(g, 25)
	g.storyThreat(100, now)
	v := g.StoryDebug()
	if g.raid.Ships.Parts() != 1 || v.LastThreat.ID != incident.CrashedShipPart || v.LastThreatAt != now || g.dayOff() == 0 {
		t.Fatalf("%d parts, last %s at %d, want %d (day offset %d)", g.raid.Ships.Parts(), v.LastThreat.Key, v.LastThreatAt, now, g.dayOff())
	}
}

// TestThreatScriptsReport (C9): on_incident hears a pack by its own key,
// and a raid face only as the people raid's core.rival_raid, once.
func TestThreatScriptsReport(t *testing.T) {
	t.Parallel()
	f := &fakeScripts{}
	g := New(Config{Seed: 3, Width: 32, Height: 32, Scripts: f})
	if err := g.LoadStartKit(); err != nil {
		t.Fatal(err)
	}
	if p := g.raidPoints(0); p >= incident.DefOf(incident.MechRaid).MinPoints {
		t.Fatalf("a bare colony at %d points: the raid face could be mechs", p)
	}
	incidents := func() []scripting.Incident {
		var out []scripting.Incident
		for _, ev := range f.events {
			if ev.Hook == scripting.OnIncident {
				out = append(out, ev.Incident)
			}
		}
		return out
	}
	f.events = nil
	if !g.FireIncident("predator_pack") || !slices.Equal(incidents(), []scripting.Incident{{Key: "core.predator_pack", Category: "threat"}}) {
		t.Fatalf("pack: %+v", incidents())
	}
	f.events = nil
	if !g.FireIncident("raid") || !slices.Equal(incidents(), []scripting.Incident{{Key: "core.rival_raid", Category: "threat"}}) {
		t.Fatalf("raid face: %+v", incidents())
	}
}
