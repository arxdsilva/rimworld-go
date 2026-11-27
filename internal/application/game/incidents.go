// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"strings"

	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/farm"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/application/game/visit"
	"rimworld-go/internal/application/game/windfall"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/pkg/mathx"
	"rimworld-go/pkg/msgfmt"
)

// Nature events (SIM-025, plan 3.2-3.3). Every two to five days, at a
// random hour, the storyteller rolls one event among those the catalogue
// (domain/incident) allows now and the map can host: a herd crossing the
// land, a stray pet joining, farm animals wandering in, a wild animal
// taking a liking to the colony, or timbergnaws come to gnaw the trees;
// from day 15 a disease, weighted by the difficulty's disease frequency
// (disease.go, MED-04).
// Nothing is rolled while a threat is on the map; the roll waits an hour
// and tries again. The "mad animal" is a threat and is rolled by the
// threat storyteller (raids.go). All randomness is g.rng (the power events
// use their own stream, power_events.go); the schedule and
// the cooldowns are saved (Incidents) and the event animals' state with
// the wildlife (wildlifeState).

const (
	// a herd crossing: crossMin..crossMax animals, on maps wider than
	// crossMinWidth, leaving within crossDrift tiles of the entry's line.
	crossMin      = 6
	crossMax      = 14
	crossMinWidth = 30
	crossDrift    = 10
	// farm animals: farmMin..farmMax of one species.
	farmMin = 2
	farmMax = 4
	// likingRadius: a self-taming animal is this close to home.
	likingRadius = 30
	// timbergnaws: gnawMin..gnawMax arrive when the map has gnawTrees
	// living trees, gnawFood percent fed; they leave after gnawDays or
	// once no tree stands within gnawReach of any of them.
	gnawMin   = 3
	gnawMax   = 7
	gnawTrees = 10
	gnawFood  = 30
	gnawDays  = 4
	gnawReach = 40
	// madRadius: a mad animal is this close to home.
	madRadius = 40
	// natureRetry: a threat is on the map; the nature roll waits this long.
	natureRetry = clock.TicksPerHour
	// gnawTreeGrown: a tree this grown (percent) feeds a timbergnaw (as
	// in the food search).
	gnawTreeGrown = 50
)

// Species of the joining events (all ours to choose).
var (
	straySpecies = []animal.Species{animal.Husky, animal.Labrador, animal.Cat, animal.YorkshireTerrier, animal.Horse, animal.Donkey}
	farmSpecies  = []animal.Species{animal.Cow, animal.Goat, animal.Sheep, animal.Pig, animal.Chicken, animal.Turkey, animal.Alpaca, animal.Yak}
)

// gnawEvent is the timbergnaw visit in progress (saved with the wildlife).
// Chew[i] is the living tree IDs[i] is eating (0: none), to count the
// felled ones; Gone: they were sent off.
type gnawEvent struct {
	Since  uint64
	Felled int
	IDs    []entity.EntityID
	Chew   []entity.EntityID
	Gone   bool
}

// phaseIncidents runs once a tick after the animals: it follows departing
// herds and timbergnaws and, at the scheduled tick, rolls a nature event.
func (g *Game) phaseIncidents() {
	now := uint64(g.clock.Now())
	g.trackFlare(now) // power_events.go: the solar flare's end (PLAY-021); a nil check without power
	// MAP-0: the climate offset (ng.climateOff) and trade are the colony's,
	// on the first map's pass.
	if primary(g) {
		g.climateTick(now) // weather_ng1.go (TEMP-2): the climate event's offset and end
		g.tradeTick(now)   // trade.go (PLAY-024): requested caravans, the ship, pods, sessions; returns at once without trade
	}
	g.fireQueue(now) // incident_queue.go (EAI-030): queued events due now
	if !g.wild {
		return
	}
	g.trackDepartures() // population.go
	g.trackGnaw(now)
	st := &g.incidents
	switch {
	case st.NextNature == 0:
		g.scheduleNature(now)
	case now < st.NextNature:
	case g.threatActive(now):
		st.NextNature = now + natureRetry
	default:
		g.rollNature(now)
		g.scheduleNature(now)
	}
}

// scheduleNature sets the next nature roll 2-5 days on, at a random hour.
func (g *Game) scheduleNature(now uint64) {
	gap := incident.NatureGapMin + g.rng.IntN(incident.NatureGapMax-incident.NatureGapMin+1)
	g.incidents.NextNature = incident.NextNatureAt(now, gap, g.rng.IntN(clock.HoursPerDay))
}

// threatActive: a hostile is on the map (a standing raider, a manhunter or
// hive insect, an animal attacking a colonist).
func (g *Game) threatActive(now uint64) bool {
	if standing, _ := g.Raiders(); standing > 0 {
		return true
	}
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && (a.Enraged(now) || g.hostile(a)) {
			return true
		}
	}
	return false
}

// rollNature picks and fires one nature event.
func (g *Game) rollNature(now uint64) {
	if id, picked := g.pickNature(now); picked && g.fireIncident(id, -1, now) {
		g.incidentFired(id, now)
	}
}

// incidentFired records that event id (the plugin's own ID for a plugin
// incident: its cooldown is its own) happened, and a plugin incident
// announces itself under its own label (PLG-026).
func (g *Game) incidentFired(id incident.ID, now uint64) {
	g.incidents.Fired(id, g.rel(now))
	if d := g.sk.content.Incidents.DefOf(id); d.Like != d.ID {
		hx, hy := g.home()
		if src := g.sk.content.SourceOf(incident.Class, d.Key); src.AddedBy != "" { // PLG-072: name the plugin
			g.notice(hx, hy, 0, "%s (added by %s)", d.Name, src.AddedBy)
		} else {
			g.notice(hx, hy, 0, "%s", d.Name)
		}
	}
}

// patchedNote is " (changed by A, B)" for a core incident a plugin patched
// in c (PLG-072), "" for the game's own untouched ones.
func patchedNote(c Content, id incident.ID) string {
	src := c.SourceOf(incident.Class, c.Incidents.Key(id))
	if len(src.PatchedBy) == 0 {
		return ""
	}
	return " (changed by " + strings.Join(src.PatchedBy, ", ") + ")"
}

// pickNature picks one nature event (one g.rng draw when any is
// eligible); the map's checks are asked once per event and remembered for
// the roll (they draw no randomness).
func (g *Game) pickNature(now uint64) (incident.ID, bool) {
	book := g.sk.content.Incidents // plugins' incidents too (PLG-026)
	var fit [incident.MaxBook]int8 // 0 unknown, 1 fits, 2 does not
	ok := func(id incident.ID) bool {
		if fit[id] == 0 {
			fit[id] = 2
			if !g.incidentDisabled(book.DefOf(id).Key) && g.incidentFits(id, now) { // setup.go: scenario-disabled (NG-1)
				fit[id] = 1
			}
		}
		return fit[id] == 1
	}
	rn := g.rel(now)      // setup.go: the catalogue's days count from the landing (now itself on a legacy colony)
	odds := natureOdds(g) // weather_ng1.go: disease frequency (MED-04), Extreme weather (TEMP-2)
	odds.Points, odds.Winter = g.queuePoints(now), g.season(now) == "Winter"
	total := book.Total(incident.Nature, g.incidents, rn, g.biome, odds, ok)
	if total <= 0 {
		return 0, false
	}
	if bias := g.storyProfile().PositiveBias; g.hasSetup() && bias > 0 && g.rng.IntN(100) < bias {
		// EAI-033: the storyteller's positive bias draws among the helpful
		// events (a setup colony's extra draw; a legacy colony draws none)
		odds.Helpful = true
		if t := book.Total(incident.Nature, g.incidents, rn, g.biome, odds, ok); t > 0 {
			total = t
		} else {
			odds.Helpful = false
		}
	}
	return book.Pick(incident.Nature, g.incidents, rn, g.biome, odds, g.rng.IntN(total), ok)
}

// incidentFits is the map's situational check for event id. MAP-0: each
// map rolls its own nature events; the colony's events (gatherings,
// visitors, traders, colony-wide climate events) fit only on the first map.
func (g *Game) incidentFits(id incident.ID, now uint64) bool {
	id = g.sk.content.Incidents.Like(id) // a plugin incident runs its like's checks (PLG-026)
	switch id {
	case incident.Party, incident.VisitorGroup, incident.TravelerGroup, incident.WandererJoins, incident.TraderCaravan, incident.OrbitalTrader,
		incident.HeatWave, incident.ColdSnap, incident.VolcanicWinter, incident.OreMeteor, incident.SupplyDrop, incident.SkySong:
		if !primary(g) {
			return false
		}
	}
	switch id {
	case incident.HerdCrossing:
		return g.grid.Width() > crossMinWidth && g.hasWildSpecies(crosser)
	case incident.StrayJoins:
		return len(g.fitting(straySpecies)) > 0
	case incident.FarmAnimals:
		return len(g.fitting(farmSpecies)) > 0
	case incident.TakenALiking:
		return len(g.likingCandidates(now)) > 0
	case incident.TimbergnawGnawing:
		return len(g.wl.Gnaw.IDs) == 0 && g.livingTrees(gnawTrees) >= gnawTrees
	case incident.MadAnimal:
		return len(g.madCandidates(now)) > 0
	case incident.Party:
		return g.partyFits(now) // gathering.go (M5)
	case incident.VisitorGroup, incident.TravelerGroup:
		return g.visitorsFit(now) // visitors.go (M9)
	case incident.TransportPodCrash:
		_, _, _, _, ok := crashSite(g)
		return g.standingColonists() > 0 && ok
	case incident.WandererJoins:
		return g.wandererFits(now) // visitors.go (M9)
	case incident.SolarFlare, incident.ShortCircuit:
		return g.powerEventFits(id) // power_events.go (PLAY-021): false with no power devices
	case incident.Flu, incident.Plague, incident.Malaria, incident.SleepingSickness, incident.GutWorms, incident.MuscleParasites:
		return g.diseaseFits(id) // disease.go (MED-04)
	case incident.TraderCaravan:
		return g.caravanFits(now) // trade_caravan.go (PLAY-024)
	case incident.OrbitalTrader:
		return g.orbitalFits(now) // trade_orbital.go (PLAY-024)
	case incident.CrashedShipPart: // raid/shippart.go (DEF-4): one at a time, from 400 raid points
		return g.raid.Ships.Parts() == 0 && g.raidPoints(int(now/clock.TicksPerDay)) >= g.sk.content.Incidents.DefOf(incident.CrashedShipPart).MinPoints
	case incident.RivalRaid:
		return primary(g) && g.standingColonists() > 0
	case incident.PredatorPack, incident.ManhunterPack, incident.HiveSwarm, incident.Raid: // a setup colony's storyteller's, by its flags (story.go)
		return g.hasSetup() && g.threatOK(id)
	case incident.RefugeeChased:
		return g.refugeeFits() // incident_queue.go (EAI-031)
	case incident.CropBlight: // farm (FOOD-2): a healthy zone crop to strike
		_, ok := farm.Origin(plantField{g}, now)
		return ok
	case incident.Eclipse, incident.ToxicFallout: // FOOD-6: one of each at a time (StartSky)
		return true
	case incident.HeatWave, incident.ColdSnap, incident.VolcanicWinter: // TEMP-2: the season's temperature
		return climateFits(g, id, now) // weather_ng1.go
	case incident.OreMeteor, incident.SupplyDrop, incident.SkySong: // #13 windfalls (game/windfall)
		return windfallOf(g).Fits(id, now)
	}
	return false
}

// fireIncident makes event id happen; false when it could not (no tile,
// no animal). A threat spends its def's share (Def.Spend; the fired
// event's own def) of pts threat points, of today's when pts < 0.
func (g *Game) fireIncident(id incident.ID, pts int, now uint64) (fired bool) {
	if id != incident.RivalRaid && id != incident.Raid {
		defer scriptFired(g, id, &fired) // retain the original event key; raids notify on arrival
	}
	if scriptReplaced(g, id, now) { // scripts.go: the plugin's run_incident took its place
		return true
	}
	d := g.sk.content.Incidents.DefOf(id) // a plugin incident's own def, its own share
	if pts < 0 && d.Category == incident.Threat {
		pts = g.queuePoints(now) // a dev hook or the queue: today's threat points
	}
	pts, id = d.Spend(pts), g.sk.content.Incidents.Like(id) // a plugin incident runs its like's behaviour (PLG-026)
	switch id {
	case incident.Party:
		return g.throwParty(now) // gathering.go (M5)
	case incident.VisitorGroup:
		return visitorIncidentArrive(g, now)
	case incident.TravelerGroup:
		return visitsOf(g).ArriveKind(visit.KindTraveler, now) // visitors.go (M9)
	case incident.TransportPodCrash:
		return crashPod(g, now)
	case incident.WandererJoins:
		return g.wandererJoins(now) // visitors.go (M9)
	case incident.HerdCrossing:
		return g.herdCrossing()
	case incident.StrayJoins:
		return g.strayJoins()
	case incident.FarmAnimals:
		return g.farmAnimals()
	case incident.TakenALiking:
		return g.takenALiking(now)
	case incident.TimbergnawGnawing:
		return g.timbergnaws(now)
	case incident.MadAnimal:
		return g.madAnimal(now, true)
	case incident.SolarFlare:
		return g.startSolarFlare(now) // power_events.go (PLAY-021)
	case incident.ShortCircuit:
		return g.shortCircuit(now) // power_events.go (PLAY-021)
	case incident.Flu, incident.Plague, incident.Malaria, incident.SleepingSickness, incident.GutWorms, incident.MuscleParasites:
		return g.diseaseOutbreak(id, now) // disease.go (MED-04)
	case incident.TraderCaravan:
		return g.caravanArrive(now) // trade_caravan.go (PLAY-024)
	case incident.OrbitalTrader:
		return g.orbitalArrive(now) // trade_orbital.go (PLAY-024)
	case incident.CrashedShipPart: // raid/shippart.go (DEF-4): the day's raid points; d's share (a plugin's own when it likes this)
		return g.raid.Ships.Crash(d.Spend(g.raidPoints(int(now/clock.TicksPerDay))), now)
	case incident.PredatorPack: // the story's threats (story.go): packs sized by the points (animal.PackSize, EAI-005)
		return g.raid.Threats.WolfPack(pts)
	case incident.ManhunterPack:
		rageOf(g).ManhunterPack(pts)
		return true
	case incident.HiveSwarm:
		return g.raid.Hives.Infest() // raid/hive.go (EAI-035): false with no hive spot
	case incident.Raid: // a ship part, mechanoids or people
		return storyMajor(g, pts, now) || g.storyRaid(g.relDay(now), pts)
	case incident.MechRaid:
		return g.raid.Threats.MechRaid(pts, now) > 0
	case incident.RivalRaid:
		return g.queuedRaid(0, now)
	case incident.RefugeeChased:
		return g.refugeeChased(now) // incident_queue.go (EAI-031)
	case incident.CropBlight: // farm (FOOD-2, wiki: Blight)
		o, n, ok := farm.Strike(plantField{g}, now)
		g.threat("Blight", o.X, o.Y, "Blight has struck the %s (%d plants). Cut the blighted plants before it spreads.", farm.CropField(o.Species), n)
		return ok
	case incident.Eclipse, incident.ToxicFallout, incident.HeatWave, incident.ColdSnap, incident.VolcanicWinter: // FOOD-6, TEMP-2 (incident/sky.go, climate.go)
		title, text, ok := g.incidents.StartTimed(id, now, g.rng.IntN)
		if hx, hy := g.home(); ok {
			g.post(MessageView{Tick: now, X: hx, Y: hy, Letter: true, Title: title, Kind: letters.Caution, Text: text + patchedNote(g.sk.content, id)})
		}
		return ok
	case incident.OreMeteor, incident.SupplyDrop, incident.SkySong: // #13 windfalls (game/windfall)
		return windfallOf(g).Fire(id, now)
	}
	return false
}

// FireIncident makes the event with key (incident.Def.Key) happen now,
// skipping the schedule, cooldown and day checks but not the map's own
// (dev hook and tests); it reports whether it happened.
func (g *Game) FireIncident(key string) bool {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	now := uint64(g.clock.Now())
	if d, ok := g.sk.content.Incidents.ByName(key); ok && g.incidentFits(d.ID, now) && g.fireIncident(d.ID, -1, now) {
		g.incidentFired(d.ID, now)
		return true
	}
	return false
}

// PluginIncident is a plugin's incident for the developer tools' list
// (PLG-026): the key FireIncident takes, its own label and the core
// incident whose behaviour it runs.
type PluginIncident struct {
	Key, Label, Like string
}

// PluginIncidents lists the run's plugin incidents in load order (empty
// with no plugins). Read only.
func (g *Game) PluginIncidents() []PluginIncident {
	var out []PluginIncident
	book := g.sk.content.Incidents
	for _, d := range book.All() {
		if d.Like != d.ID {
			out = append(out, PluginIncident{Key: d.Key, Label: d.Name, Like: book.DefOf(d.Like).Name})
		}
	}
	return out
}

// notice posts a message about creature focus (the UI's jump link
// follows it).
func (g *Game) notice(x, y int, focus entity.EntityID, format string, args ...any) {
	g.post(MessageView{Tick: uint64(g.clock.Now()), Msg: msgfmt.Of(format, args...), X: x, Y: y, Focus: focus})
}

// --- climate fit and species -----------------------------------------------

// climateFit: species s can live in the map's biome: it lives wild there,
// or its habitat suits the biome's climate (domestic and pet species).
func (g *Game) climateFit(s animal.Species) bool {
	if animal.Weight(s, g.biome) > 0 {
		return true
	}
	return animal.RosterOf(s).Habitat&animal.ClimateBand(biome.DefOf(g.biome).Warmth) != 0
}

// fitting is the climate-fit species of list.
func (g *Game) fitting(list []animal.Species) []animal.Species {
	var out []animal.Species
	for _, s := range list {
		if g.climateFit(s) {
			out = append(out, s)
		}
	}
	return out
}

// crosser: a herd crossing's species, a plant-eating herd animal no
// smaller than a hare.
func crosser(s animal.Species) bool {
	r := animal.RosterOf(s)
	return !isPredator(s) && animal.EcologyOf(s).Diet == animal.Herbivore && r.HerdMax >= 3 && r.Size >= animal.SizeS
}

// hasWildSpecies: the biome has wildlife passing keep this season (no
// randomness drawn, unlike pickSpecies).
func (g *Game) hasWildSpecies(keep func(animal.Species) bool) bool {
	season := g.season(uint64(g.clock.Now())) // setup.go: the map's own calendar
	for _, s := range animal.All() {
		if keep(s) && animal.WildWeight(s, g.biome, season) > 0 {
			return true
		}
	}
	return false
}

// freshPetName is a pet name no tame animal on the map has yet, when one
// is found in a few draws.
func (g *Game) freshPetName() string {
	used := map[string]bool{}
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && a.Tame {
			used[a.Name] = true
		}
	}
	name := petNames[g.rng.IntN(len(petNames))]
	for try := 0; try < 8 && used[name]; try++ {
		name = petNames[g.rng.IntN(len(petNames))]
	}
	return name
}

// joinColony makes animal id the colony's: tame, named, calm.
func (g *Game) joinColony(id entity.EntityID, a *animal.State, kind uint8) {
	a.Tame, a.TameMark, a.Designated, a.Leaving = true, false, false, false
	a.Name = g.freshPetName()
	a.Herd, a.Target, a.Attacker, a.HuntSince = 0, 0, 0, 0
	a.Mode, a.Path, a.PathI, a.Feed, a.FeedLeft = animal.Wander, nil, 0, animal.FeedTarget{}, 0
	a.NextThink, a.NextDecide = 0, uint64(g.clock.Now())
	g.logEvent(id, logEntry{Kind: kind})
}

// --- the events -------------------------------------------------------------

// herdCrossing brings 6-14 of one grazing species in at one edge, all
// walking to the opposite edge (within crossDrift of the entry's line).
// They are huntable and flee as any wild animal on the way, but are not
// counted against capacity (they are Leaving).
func (g *Game) herdCrossing() bool {
	s, ok := g.pickSpecies(crosser)
	if !ok {
		return false
	}
	ax, ay, bx, by, ok := g.crossingLine()
	if !ok {
		return false
	}
	ids := g.spawnHerd(s, ax, ay, crossMin+g.rng.IntN(crossMax-crossMin+1))
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		g.startLeaving(id, bx, by)
		g.logEvent(id, logEntry{Kind: evlog.Leaving})
	}
	g.notice(ax, ay, ids[0], "A herd of %d %s is crossing the land.", len(ids), animal.Plural(s))
	return true
}

// crossingLine is a walkable entry tile on one map edge and an exit on the
// opposite one, joined by a path.
func (g *Game) crossingLine() (ax, ay, bx, by int, ok bool) {
	w, h := g.grid.Width(), g.grid.Height()
	q := g.hostileMap()
	for try := 0; try < 8; try++ {
		drift := g.rng.IntN(2*crossDrift+1) - crossDrift
		switch g.rng.IntN(4) {
		case 0: // north to south
			ax, ay = g.rng.IntN(w), 0
			bx, by = mathx.Clamp(ax+drift, 0, w-1), h-1
		case 1: // south to north
			ax, ay = g.rng.IntN(w), h-1
			bx, by = mathx.Clamp(ax+drift, 0, w-1), 0
		case 2: // west to east
			ax, ay = 0, g.rng.IntN(h)
			bx, by = w-1, mathx.Clamp(ay+drift, 0, h-1)
		default: // east to west
			ax, ay = w-1, g.rng.IntN(h)
			bx, by = 0, mathx.Clamp(ay+drift, 0, h-1)
		}
		if !g.dryWalkable(ax, ay) || !g.dryWalkable(bx, by) {
			continue
		}
		if _, found := g.searcher().Find(q, ax, ay, bx, by); found {
			return ax, ay, bx, by, true
		}
	}
	return 0, 0, 0, 0, false
}

// strayJoins: one climate-fit pet or working animal walks in from an edge
// the colony can be reached from, already tame and named; it heads for a
// pen or home as every tame animal does.
func (g *Game) strayJoins() bool {
	list := g.fitting(straySpecies)
	if len(list) == 0 {
		return false
	}
	s := list[g.rng.IntN(len(list))]
	x, y, ok := g.edgeTile()
	if !ok {
		return false
	}
	id, ok := g.raid.Hives.SpawnNear(s, x, y)
	if !ok {
		return false
	}
	a, _ := g.animals.Get(id)
	a.Female = g.rng.IntN(2) == 0
	g.joinColony(id, &a, evlog.Joined)
	// A stray arrives as a colony animal and follows the same native
	// walking path to home as any other tame creature, including livestock
	// that would otherwise roam indefinitely along the entry edge.
	hx, hy := g.home()
	g.animalGoTo(id, &a, hx, hy)
	g.animals.Set(id, a)
	g.notice(x, y, id, "A stray %s, %s, has joined the colony.", s.Label(), a.Name)
	return true
}

// farmAnimals: 2-4 animals of one climate-fit farm species wander in tame,
// at least one of each sex.
func (g *Game) farmAnimals() bool {
	list := g.fitting(farmSpecies)
	if len(list) == 0 {
		return false
	}
	s := list[g.rng.IntN(len(list))]
	x, y, ok := g.edgeTile()
	if !ok {
		return false
	}
	n := farmMin + g.rng.IntN(farmMax-farmMin+1)
	var ids []entity.EntityID
	for i := 0; i < n; i++ {
		id, ok := g.raid.Hives.SpawnNear(s, x, y)
		if !ok {
			continue
		}
		a, _ := g.animals.Get(id)
		switch len(ids) {
		case 0:
			a.Female = true
		case 1:
			a.Female = false
		default:
			a.Female = g.rng.IntN(2) == 0
		}
		g.joinColony(id, &a, evlog.Joined)
		g.animals.Set(id, a)
		ids = append(ids, id)
	}
	switch len(ids) {
	case 0:
		return false
	case 1:
		g.notice(x, y, ids[0], "A %s has wandered in and stayed.", s.Label())
	default:
		g.notice(x, y, ids[0], "%d %s have wandered in and stayed.", len(ids), animal.Plural(s))
	}
	return true
}

// likingCandidates are the wild animals that may take a liking to the
// colony: calm, tameable non-predators near home, in id order.
func (g *Game) likingCandidates(now uint64) []entity.EntityID {
	var out []entity.EntityID
	hx, hy := g.home()
	g.within(hx, hy, likingRadius, ckAnimal, func(id entity.EntityID, _ Position) {
		a, ok := g.animals.Get(id)
		if !ok || a.Tame || a.Leaving || a.Enraged(now) || a.Mode == animal.Flee || a.Mode == animal.Attack {
			return
		}
		d := animal.DefOf(a.Species)
		if d.Predator || d.Legendary || d.Hive || d.TameChance == 0 || g.beingTamed(id) {
			return
		}
		out = append(out, id)
	})
	return out
}

// takenALiking: a wild animal near home tames itself.
func (g *Game) takenALiking(now uint64) bool {
	cands := g.likingCandidates(now)
	if len(cands) == 0 {
		return false
	}
	id := cands[g.rng.IntN(len(cands))]
	a, _ := g.animals.Get(id)
	g.joinColony(id, &a, evlog.Liking)
	g.animals.Set(id, a)
	p, _ := g.positions.Get(id)
	g.notice(p.X, p.Y, id, "A wild %s has taken a liking to the colony: meet %s.", a.Species.Label(), a.Name)
	return true
}

// gnawTree: a plant a timbergnaw would eat (as the food search has it).
func gnawTree(p plant.Wild) bool {
	return plant.DefOf(p.Species).Trunk && !p.Zone && p.Percent() >= gnawTreeGrown
}

// livingTrees counts the map's trees, stopping at enough.
func (g *Game) livingTrees(enough int) int {
	n := 0
	for _, id := range g.plantIDs {
		if p, ok := g.plants.Get(id); ok && plant.DefOf(p.Species).Trunk && !p.Zone {
			if n++; n >= enough {
				break
			}
		}
	}
	return n
}

// timbergnaws: 3-7 hungry timbergnaws come in at an edge to eat the trees.
func (g *Game) timbergnaws(now uint64) bool {
	x, y, ok := g.edgeTile()
	if !ok {
		return false
	}
	n := gnawMin + g.rng.IntN(gnawMax-gnawMin+1)
	g.wl.LastHerd++
	herd := g.wl.LastHerd
	ev := gnawEvent{Since: now}
	for i := 0; i < n; i++ {
		id, ok := g.raid.Hives.SpawnNear(animal.Timbergnaw, x, y)
		if !ok {
			continue
		}
		a, _ := g.animals.Get(id)
		a.Female = g.rng.IntN(2) == 0
		a.Food, a.Herd, a.NextDecide = animal.Full*gnawFood/100, herd, now
		g.animals.Set(id, a)
		ev.IDs = append(ev.IDs, id)
	}
	if len(ev.IDs) == 0 {
		return false
	}
	ev.Chew = make([]entity.EntityID, len(ev.IDs))
	g.wl.Gnaw = ev
	g.notice(x, y, ev.IDs[0], "%d timbergnaws have come to gnaw at the trees.", len(ev.IDs))
	return true
}

// trackGnaw follows the timbergnaws (a handful) every tick: a tree one was
// eating that is gone was felled; every hour they check whether to leave
// (after gnawDays, or with no tree left in reach).
func (g *Game) trackGnaw(now uint64) {
	gw := &g.wl.Gnaw
	if len(gw.IDs) == 0 {
		return
	}
	var counted []entity.EntityID
	ids, chew := gw.IDs[:0], gw.Chew[:0]
	for i, id := range gw.IDs {
		if pid := gw.Chew[i]; pid != 0 {
			if _, stands := g.plants.Get(pid); !stands && !slices.Contains(counted, pid) {
				counted = append(counted, pid)
				gw.Felled++
			}
		}
		a, ok := g.animals.Get(id)
		if !ok {
			continue // died or walked off
		}
		var on entity.EntityID
		if a.Mode == animal.Feed && a.Feed.Kind == animal.FeedTree {
			if _, stands := g.plants.Get(a.Feed.ID); stands {
				on = a.Feed.ID
			}
		}
		ids, chew = append(ids, id), append(chew, on)
	}
	gw.IDs, gw.Chew = ids, chew
	if len(gw.IDs) == 0 {
		*gw = gnawEvent{}
		return
	}
	if gw.Gone || now%clock.TicksPerHour != 0 {
		return
	}
	if now-gw.Since >= gnawDays*clock.TicksPerDay || !g.treeNearGnaws() {
		g.sendGnawsOff()
	}
}

// treeNearGnaws: some timbergnaw has a tree to eat within gnawReach.
func (g *Game) treeNearGnaws() bool {
	fx := g.food()
	for _, id := range g.wl.Gnaw.IDs {
		pos, ok := g.positions.Get(id)
		if !ok {
			continue
		}
		found := false
		fx.plants.Each(pos.X, pos.Y, gnawReach, func(pid entity.EntityID) bool {
			if p, ok := g.plants.Get(pid); ok && gnawTree(p) && weapon.Distance(pos.X, pos.Y, p.X, p.Y) <= gnawReach {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// sendGnawsOff walks the timbergnaws off at the edge nearest the first.
func (g *Game) sendGnawsOff() {
	gw := &g.wl.Gnaw
	gw.Gone = true
	lead, _ := g.positions.Get(gw.IDs[0])
	ex, ey := nearestEdge(lead, g.grid.Width(), g.grid.Height())
	if x, y, ok := g.combat.Tiles.WalkableNear(ex, ey, 4); ok {
		ex, ey = x, y
	}
	for _, id := range gw.IDs {
		if a, ok := g.animals.Get(id); ok && !a.Tame {
			g.startLeaving(id, ex, ey)
			g.logEvent(id, logEntry{Kind: evlog.Leaving})
		}
	}
	g.message(ex, ey, "The timbergnaws are leaving, %d trees felled.", gw.Felled)
}

// Timbergnaws is the timbergnaw visit for the alert: how many are still
// gnawing (wild, not leaving) and how many trees they have felled.
func (g *Game) Timbergnaws() (count, felled int) {
	for _, id := range g.wl.Gnaw.IDs {
		if a, ok := g.animals.Get(id); ok && !a.Tame && !a.Leaving {
			count++
		}
	}
	return count, g.wl.Gnaw.Felled
}

// madCandidates are the wild animals near home that may go mad, in id
// order.
func (g *Game) madCandidates(now uint64) []entity.EntityID {
	var out []entity.EntityID
	hx, hy := g.home()
	g.within(hx, hy, madRadius, ckAnimal, func(id entity.EntityID, _ Position) {
		a, ok := g.animals.Get(id)
		if !ok || a.Tame || a.Leaving || a.Enraged(now) {
			return
		}
		if d := animal.DefOf(a.Species); d.Hive || d.Legendary || d.Mech {
			return
		}
		out = append(out, id)
	})
	return out
}

// madAnimal (a threat): one wild animal near home goes manhunter. Unless
// forced, it keeps to the event's first day and cooldown; it reports
// whether one went mad.
func (g *Game) madAnimal(now uint64, force bool) bool {
	if !force && !incident.DefOf(incident.MadAnimal).Allowed(g.incidents, g.rel(now), g.biome) {
		return false
	}
	cands := g.madCandidates(now)
	if len(cands) == 0 {
		return false
	}
	id := cands[g.rng.IntN(len(cands))]
	a, _ := g.animals.Get(id)
	pos, _ := g.positions.Get(id)
	who, _, _ := g.nearestColonist(pos)
	rageOf(g).Enrage(id, &a, who, now) // the existing manhunter threat message
	g.animals.Set(id, a)
	g.incidents.Fired(incident.MadAnimal, g.rel(now))
	return true
}

// stepToxic is an hour of toxic buildup (FOOD-6, body/cond_toxic.go): in
// toxic fallout every pawn and animal under the open sky gains it (an
// animal at half the rate, hive insects and machines never), and it
// recedes on everyone sheltered. Animals are walked only while the
// fallout lasts; after it the poisoned ones are animal patients.
func stepToxic(g *Game, now uint64) {
	fallout, h, roll := g.incidents.Fallout(now), now/clock.TicksPerHour, g.stream(streamMedical).IntN
	var ids []entity.EntityID
	g.need.All(func(id entity.EntityID, _ needs.Pair) {
		if (fallout || body.ToxicBuildup(g.bodies[id]) > 0) && owns(g, id) && !worldPawnCandidate(g, id) { // MAP-0: on its own map's pass, under its own sky
			ids = append(ids, id)
		}
	})
	for _, id := range ids {
		v, ok := g.vital.Get(id)
		pos, _ := g.positions.Get(id)
		out := fallout && g.onMap(id) && !g.roofAt(pos.X, pos.Y).Roofed()
		if !ok || v.Status == health.Dead || !out && body.ToxicBuildup(g.bodies[id]) == 0 {
			continue
		}
		b := g.ensureBody(id, &v)
		if b == nil {
			continue
		}
		switch st := body.StepToxic(b, out, 0, h, roll); {
		case st.Dead:
			g.message(pos.X, pos.Y, "%s died of toxic buildup.", g.name(id))
			g.dieMedical(id, health.CauseDisease) // caravan_medical.go
			g.clearPatient(id)
		default:
			if st.Changed {
				g.bodyChanged(id, b, &v) // body.go: stats follow, joins the patients
				g.updateDowned(id, b, &v)
			}
			if st.Vomit && g.onMap(id) {
				g.addDirt(pos.Y*g.grid.Width()+pos.X, vomitDirt) // filth.go
			}
			if st.Dementia {
				g.developAging(id, body.CondDementia, int(body.Brain), now) // aging.go
			}
			if st.Carcinoma {
				g.developAging(id, body.CondCarcinoma, -1, now)
			}
		}
	}
	ids = g.animalPatientIDs()
	if fallout {
		ids = g.animalIDs
	}
	for _, id := range slices.Clone(ids) { // a death touches the stores
		a, ok := g.animals.Get(id)
		pos, _ := g.positions.Get(id)
		out := fallout && !g.roofAt(pos.X, pos.Y).Roofed()
		if !ok || a.Dead() || animal.IsMech(a.Species) || animal.DefOf(a.Species).Hive || !out && body.ToxicBuildup(a.Body) == 0 {
			continue
		}
		if a.Body == nil {
			a.Body = animal.NewBody(a.Species) // animal_health.go
		}
		if body.StepToxic(a.Body, out, body.ToxicResistAnimal, h, roll).Dead {
			g.animalMedicalDeath(id, &a)
			continue
		}
		animal.ReconcileBody(&a)
		g.checkAnimalDowned(id, &a, now)
		g.animals.Set(id, a)
		if a.Body != nil {
			g.markAnimalPatient(id)
		}
	}
}

// toxicRot rots a body that died with toxic buildup sev (permille) at
// once, with a chance equal to sev (FOOD-6, wiki: Toxic buildup, Toxic
// fallout). A carcass merged into others on its tile is left alone.
func toxicRot(g *Game, corpse entity.EntityID, sev int32) {
	if st, ok := g.stacks.Get(corpse); ok && st.Qty == 1 && sev > 0 && int32(g.stream(streamMedical).IntN(1000)) < sev {
		st.Rot = st.Shelf()
		g.stacks.Set(corpse, st)
	}
}

// crashSite is a read-only, deterministic landing preflight: the shell
// and victim's adjacent dry cells must both be free under open sky.
func crashSite(g *Game) (x, y, vx, vy int, ok bool) {
	hx, hy := g.home()
	x, y, ok = travel.Ring(hx, hy, 2*podRadius, func(x, y int) bool {
		if !g.podFree(x, y, nil) || g.roofAt(x, y).Roofed() {
			return false
		}
		if _, _, occupied := g.nearestWhere(x, y, 0, ckAll, nil); occupied {
			return false
		}
		vx, vy, ok = travel.Ring(x, y, 1, func(nx, ny int) bool {
			if nx == x && ny == y || !g.dryWalkable(nx, ny) || !cellStandable(g, nx, ny) || g.roofAt(nx, ny).Roofed() || g.fireNear(nx, ny, 0) {
				return false
			}
			_, _, occupied := g.nearestWhere(nx, ny, 0, ckAll, nil)
			return !occupied
		})
		return ok
	})
	return
}

func crashPod(g *Game, now uint64) bool {
	x, y, vx, vy, ok := crashSite(g)
	if !ok || !g.BuildNow(construction.DefOf(construction.PodShell).Name, item.Steel.String(), "south", x, y) {
		return false
	}
	ref := incident.CrashFaction(g.ng.factions, g.rng.IntN)
	f := g.factionOf(ref)
	var id entity.EntityID
	if f != nil && f.Relation == faction.Hostile {
		group := len(g.raidGroups)
		g.raidGroups = append(g.raidGroups, raid.Group{Faction: ref, Size: 1, Over: true})
		id = spawnRaider(g, 0, group, item.Club, vx, vy)
		delete(g.equip, id)
		delete(g.apparel, id)
	} else {
		id = g.spawnVisitor(ref, f, vx, vy, now)
	}
	// Real crash trauma must keep the victim down under the native recovery rule.
	g.hurtAs(id, health.DownedInjury+10, 0, body.Cut, health.CauseInjury)
	g.post(MessageView{Tick: now, X: x, Y: y, Focus: id, Letter: true, Title: "Transport pod crash", Kind: letters.Info, Msg: msgfmt.Of("A transport pod crashed. %s lies injured nearby.", g.name(id))})
	return true
}

// windfallHost is windfall.Host over the game (#13): a thin view, so no
// method is added to Game; windfallOf builds a Windfall per call on g.rng.
type windfallHost struct{ g *Game }

func windfallOf(g *Game) windfall.Windfall { return windfall.New(windfallHost{g}, g.rng) }

func (h windfallHost) Home() (int, int)                 { return h.g.home() }
func (h windfallHost) Colonists() int                   { return h.g.standingColonists() }
func (h windfallHost) Rock(x, y int, ore geology.Ore)   { mapsPop(h).LumpVein(x, y, ore) }
func (h windfallHost) SongEnds(now uint64) uint64       { return h.g.incidents.SongView(now) }
func (h windfallHost) StartSong(now, ticks uint64) bool { return h.g.incidents.StartSong(now, ticks) }
func (h windfallHost) Drop(k item.Kind, qty, x, y int) {
	h.g.dropStackOf(item.Stack{Kind: k, Qty: qty, Forbidden: true}, x, y)
}
func (h windfallHost) DropFree(x, y int, t [][2]int) bool {
	return h.g.podFree(x, y, t) && h.open(x, y)
}

// open: unzoned, unroofed, no item.
func (h windfallHost) open(x, y int) bool {
	_, z := h.g.zones.At(x, y)
	return !z && !h.g.roofAt(x, y).Roofed() && h.g.itemAt(x, y) == 0
}

// MeteorFree: 2 inside the edges, open dry ground (no plant, no blocker),
// unroofed, unzoned, no item, no building or frame on the cell or beside
// it (a rock beside a door would re-orient it), no creature within 1.
func (h windfallHost) MeteorFree(x, y int) bool {
	g := h.g
	if x < 2 || y < 2 || x >= g.grid.Width()-2 || y >= g.grid.Height()-2 || !mapsPop(h).LumpFree(x, y) || !h.open(x, y) {
		return false
	}
	for c := range 9 { // the cell and its 8 neighbours, every layer
		for l := range construction.Layer(numLayers) {
			if nx, ny := x+c%3-1, y+c/3-1; g.layerBldgAt(nx, ny, l) != 0 || g.layerFrameAt(nx, ny, l) != 0 {
				return false
			}
		}
	}
	_, _, near := g.nearestWhere(x, y, 1, ckAll, nil)
	return !near
}

// Cheer gives the colonists at home (no prisoners or guests) the sky song
// memory until the song ends.
func (h windfallHost) Cheer(now, until uint64) int {
	ids := h.g.homeColonists()
	for _, id := range ids {
		h.g.memories[id] = h.g.memories[id].AddLabeledUntil(mood.SkySong, 0, 0, "", now, until)
	}
	return len(ids)
}

func (h windfallHost) Letter(title string, x, y int, format string, args ...any) {
	h.g.post(MessageView{Tick: uint64(h.g.clock.Now()), X: x, Y: y, Letter: true, Title: title, Kind: letters.Good, Msg: msgfmt.Of(format, args...)})
}
