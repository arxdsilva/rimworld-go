package game

import (
	"fmt"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/plant"
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
	if id, picked := g.pickNature(now); picked && g.fireIncident(id, now) {
		g.incidents.Fired(id, g.rel(now))
	}
}

// pickNature picks one nature event (one g.rng draw when any is
// eligible); the map's checks are asked once per event and remembered for
// the roll (they draw no randomness).
func (g *Game) pickNature(now uint64) (incident.ID, bool) {
	var fit [incident.NumIDs]int8 // 0 unknown, 1 fits, 2 does not
	ok := func(id incident.ID) bool {
		if fit[id] == 0 {
			fit[id] = 2
			if !g.incidentDisabled(incident.DefOf(id).Key) && g.incidentFits(id, now) { // setup.go: scenario-disabled (NG-1)
				fit[id] = 1
			}
		}
		return fit[id] == 1
	}
	rn := g.rel(now)       // setup.go: the catalogue's days count from the landing (now itself on a legacy colony)
	freq := g.diseasePct() // disease.go (MED-04): the difficulty's disease frequency
	total := incident.TotalAt(incident.Nature, g.incidents, rn, g.biome, freq, ok)
	if total <= 0 {
		return 0, false
	}
	return incident.PickAt(incident.Nature, g.incidents, rn, g.biome, freq, g.rng.IntN(total), ok)
}

// incidentFits is the map's situational check for event id.
func (g *Game) incidentFits(id incident.ID, now uint64) bool {
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
	case incident.VisitorGroup:
		return g.visitorsFit(now) // visitors.go (M9)
	case incident.WandererJoins:
		return g.wandererFits(now) // visitors.go (M9)
	case incident.SolarFlare, incident.ShortCircuit:
		return g.powerEventFits(id) // power_events.go (PLAY-021): false with no power devices
	case incident.Flu, incident.Plague, incident.Malaria, incident.SleepingSickness, incident.GutWorms, incident.MuscleParasites:
		return g.diseaseFits(id) // disease.go (MED-04)
	}
	return false
}

// fireIncident makes event id happen; false when it could not (no tile,
// no animal).
func (g *Game) fireIncident(id incident.ID, now uint64) bool {
	switch id {
	case incident.Party:
		return g.throwParty(now) // gathering.go (M5)
	case incident.VisitorGroup:
		return g.visitorsArrive(now) // visitors.go (M9)
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
	}
	return false
}

// FireIncident makes the event with key (incident.Def.Key) happen now,
// skipping the schedule, cooldown and day checks but not the map's own
// (dev hook and tests); it reports whether it happened.
func (g *Game) FireIncident(key string) bool {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	now := uint64(g.clock.Now())
	for _, d := range incident.All() {
		if d.Key == key && g.incidentFits(d.ID, now) && g.fireIncident(d.ID, now) {
			g.incidents.Fired(d.ID, g.rel(now))
			return true
		}
	}
	return false
}

// notice posts a message about creature focus (the UI's jump link
// follows it).
func (g *Game) notice(x, y int, focus entity.EntityID, format string, args ...any) {
	g.post(MessageView{Tick: uint64(g.clock.Now()), Text: fmt.Sprintf(format, args...), X: x, Y: y, Focus: focus})
}

// --- climate fit and species -----------------------------------------------

// Bands of a biome's yearly mean (tenths of a degree C) for climate fit.
const (
	coldMeanMax = 20  // at or below: a cold land
	hotMeanMin  = 200 // at or above: a hot land
)

// climateBand is the habitat a yearly mean temperature calls for.
func climateBand(mean int) animal.Habitat {
	switch {
	case mean <= coldMeanMax:
		return animal.HabCold
	case mean >= hotMeanMin:
		return animal.HabHot
	}
	return animal.HabTemperate
}

// climateFit: species s can live in the map's biome: it lives wild there,
// or its habitat suits the biome's climate (domestic and pet species).
func (g *Game) climateFit(s animal.Species) bool {
	if animal.Weight(s, g.biome) > 0 {
		return true
	}
	return animal.RosterOf(s).Habitat&climateBand(biome.DefOf(g.biome).Climate.YearMean) != 0
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
		if keep(s) && wildWeight(s, g.biome, season) > 0 {
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
		g.logEvent(id, logEntry{Kind: logLeaving})
	}
	g.notice(ax, ay, ids[0], "A herd of %d %s is crossing the land.", len(ids), pluralLabel(s))
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
			bx, by = clampInt(ax+drift, 0, w-1), h-1
		case 1: // south to north
			ax, ay = g.rng.IntN(w), h-1
			bx, by = clampInt(ax+drift, 0, w-1), 0
		case 2: // west to east
			ax, ay = 0, g.rng.IntN(h)
			bx, by = w-1, clampInt(ay+drift, 0, h-1)
		default: // east to west
			ax, ay = w-1, g.rng.IntN(h)
			bx, by = 0, clampInt(ay+drift, 0, h-1)
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
	id, ok := g.spawnNear(s, x, y)
	if !ok {
		return false
	}
	a, _ := g.animals.Get(id)
	a.Female = g.rng.IntN(2) == 0
	g.joinColony(id, &a, logJoined)
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
		id, ok := g.spawnNear(s, x, y)
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
		g.joinColony(id, &a, logJoined)
		g.animals.Set(id, a)
		ids = append(ids, id)
	}
	switch len(ids) {
	case 0:
		return false
	case 1:
		g.notice(x, y, ids[0], "A %s has wandered in and stayed.", s.Label())
	default:
		g.notice(x, y, ids[0], "%d %s have wandered in and stayed.", len(ids), pluralLabel(s))
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
	g.joinColony(id, &a, logLiking)
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
		id, ok := g.spawnNear(animal.Timbergnaw, x, y)
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
			if _, stands := g.plants.Get(pid); !stands && !containsID(counted, pid) {
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
		fx.plants.each(pos.X, pos.Y, gnawReach, func(pid entity.EntityID) bool {
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
	if x, y, ok := g.walkableNear(ex, ey, 4); ok {
		ex, ey = x, y
	}
	for _, id := range gw.IDs {
		if a, ok := g.animals.Get(id); ok && !a.Tame {
			g.startLeaving(id, ex, ey)
			g.logEvent(id, logEntry{Kind: logLeaving})
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

func containsID(ids []entity.EntityID, id entity.EntityID) bool {
	for _, o := range ids {
		if o == id {
			return true
		}
	}
	return false
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
		if d := animal.DefOf(a.Species); d.Hive || d.Legendary {
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
	g.enrage(id, &a, who, now) // the existing manhunter threat message
	g.animals.Set(id, a)
	g.incidents.Fired(incident.MadAnimal, g.rel(now))
	return true
}
