// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"strings"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/ship"
	"rimworld-go/internal/application/game/wealth"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/pkg/msgfmt"
)

// The storyteller of a setup colony (NG-1 §5.2, §5.3). Owned by SETUP.
// A legacy colony never reaches this file: raids.go keeps today's
// scheduler for it. Here the chosen storyteller's Plan decides when the
// next threat comes and how strong (the legacy profile makes exactly the
// pre-NG-1 draws), raid.PickThreat's die is filtered by the difficulty, the
// scenario's disabled incidents and whether any hostile faction can raid,
// and raid points come from difficulty.ThreatPoints with adaptation.
// Colony time (rel, relDay) counts from the landing day.

const (
	// scheduledHour is when a scenario's scheduled incidents happen.
	scheduledHour = 10
	// raiderNameSalt keeps generated raider names apart from colonists'.
	raiderNameSalt = 0x7261696465 // "raide"
)

// storyProfile is the colony's storyteller (legacy if the key is unknown).
func (g *Game) storyProfile() storyteller.Profile {
	if g.hasSetup() {
		if p, ok := storyteller.ByKey(g.ng.setup.Storyteller); ok {
			return p
		}
	}
	p, _ := storyteller.ByKey("legacy")
	return p
}

// scheduleStory makes the storyteller's first plan: never before the
// storyteller's first day nor the difficulty's peaceful days.
func (g *Game) scheduleStory(now uint64) {
	g.threatsOn = true
	p := g.storyProfile()
	g.ng.story = storyteller.State{}
	d := storyteller.Plan(p, &g.ng.story, g.rel(now), g.rng)
	first := max(p.FirstThreatDay, g.ng.setup.Difficulty.PeacefulDays)
	if day := int(d.AtTick / clock.TicksPerDay); day < first {
		d.AtTick += uint64(first-day) * clock.TicksPerDay
	}
	g.setPlan(d)
}

// setPlan makes d (colony time) the pending threat.
func (g *Game) setPlan(d storyteller.Decision) {
	g.ng.plan = d
	g.nextThreat = d.AtTick + g.dayOff()
}

// fireStoryThreat fires the pending threat (two on a Dice double) and
// plans the next.
func (g *Game) fireStoryThreat(now uint64) {
	day := g.relDay(now)
	winter := g.season(now) == "Winter"
	pct := g.ng.plan.PointsPct
	if pct <= 0 {
		pct = 100
	}
	second := g.ng.plan.Second != storyteller.None
	g.storyThreat(g.rng.IntN(incident.ThreatRolls), day, winter, pct, now)
	if second {
		g.storyThreat(g.rng.IntN(incident.ThreatRolls), day, winter, pct, now)
	}
	g.setPlan(storyteller.Plan(g.storyProfile(), &g.ng.story, g.rel(now), g.rng))
}

// storyThreat fires the threat event a roll of the die shows (the book's
// PickThreat over the colony's flags, threatOK) at the threat points times
// pct and records it. One cooling down or unable to happen falls back as
// the die always did: a hive swarm, and a mad animal while raids are
// allowed, become a people raid (recorded as a raid).
func (g *Game) storyThreat(roll, day int, winter bool, pct int, now uint64) {
	book, pts, rn := g.sk.content.Incidents, g.threatPoints()*pct/100, g.rel(now)
	id, ok := book.PickThreat(rn, g.biome, incident.Odds{Points: pts, Winter: winter}, roll, g.threatOK)
	if !ok {
		return
	}
	if book.DefOf(id).Cooled(g.incidents, rn) && g.fireIncident(id, pts, now) {
		g.incidentFired(id, now)
	} else if (id == incident.HiveSwarm || id == incident.MadAnimal && g.threatOK(incident.Raid)) && g.storyRaid(day, pts) {
		g.incidentFired(incident.Raid, now)
	}
}

// storyMajor turns a raid face into a crashed ship part or a mechanoid raid
// of pts by the wiki's big-threat weights (raid/threats.go Major, DEF-4);
// false: a people raid comes instead.
func storyMajor(g *Game, pts int, now uint64) bool {
	humans := 0 // each hostile human faction weighs 1 as a raider (wiki: Events)
	for _, f := range g.ng.factions {
		if faction.Raider(f) {
			humans++
		}
	}
	switch g.raid.Threats.Major(pts, humans, g.sk.content.Incidents.DefOf) {
	case raid.MajorShipPart:
		return g.FireIncident("crashed_ship_part")
	case raid.MajorMechs: // through the incident framework: recorded and reported
		if g.fireIncident(incident.MechRaid, pts, now) {
			g.incidentFired(incident.MechRaid, now)
			return true
		}
	}
	return false
}

// threatOK: the difficulty, the scenario and the factions allow threat
// event id (the four the die's flags rule on; any other is allowed).
func (g *Game) threatOK(id incident.ID) bool {
	s := g.ng.setup.Difficulty
	switch id {
	case incident.Raid:
		return s.BigThreats && !g.incidentDisabled("raid") && hostileRaider(g)
	case incident.HiveSwarm:
		return s.BigThreats && !g.incidentDisabled("hive_swarm")
	case incident.PredatorPack:
		return s.PredatorsHuntColonists && !g.incidentDisabled("predator_pack")
	case incident.ManhunterPack:
		return !g.incidentDisabled("manhunter_pack")
	}
	return true
}

// standingColonists counts the colonists on their feet.
func (g *Game) standingColonists() int {
	n := 0
	for _, id := range g.pawnIDs {
		if g.standingPawn(id) {
			n++
		}
	}
	return n
}

// threatPoints is a setup colony's threat points (NG-1 §3.3).
func (g *Game) threatPoints() int {
	return weightedThreat(g, threatBase(g, false), g.rel(uint64(g.clock.Now())))
}

// threatBase returns the selected map's base (or the sum of assets for
// census reads). Quest sizing evaluates each scope independently.
func threatBase(g *Game, all bool) difficulty.ThreatBase {
	return threatCensusOf(g, all).ThreatBase
}

func threatCensusOf(g *Game, all bool) (total wealth.Census) {
	eachThreatScope(g, all, total.Add)
	return total
}

// eachThreatScope reads the actual native owners. Each map and caravan
// contributes once; travel cargo is separate from its former home.
func eachThreatScope(g *Game, all bool, use func(wealth.Census)) {
	cur := g.mapState
	defer func() { g.mapState = cur }()
	addPawn := func(c *wealth.Census, id entity.EntityID, asleep bool) {
		v, ok := g.vital.Get(id)
		if !ok || v.Status == health.Dead || g.prison.Is(id) {
			return
		}
		if p := g.personOf(id); p != nil && (p.Faction != 0 || p.Where != whereColony && p.Where != whereCaravan) {
			return
		}
		bio, known := g.personBio(id)
		c.AddPawn(bio, known, v, g.bodies[id], asleep)
	}
	home, road := map[uint32][]*stashRec{}, map[uint64][]*stashRec{}
	if w := g.worldSt; w != nil {
		for i := range w.stashed {
			r := &w.stashed[i]
			if r.Prisoner {
				continue
			}
			if w.ax9.Sleeper(r.Holder) == r.ID || w.ship != nil && w.ship.Sleeper(r.Holder) == r.ID {
				home[r.Map] = append(home[r.Map], r)
			} else if all {
				road[uint64(r.Holder)] = append(road[uint64(r.Holder)], r)
			}
		}
	}
	for _, m := range g.maps {
		if all && (g.worldSt != nil && !maps.IsColony(&g.worldSt.atlas, m.id)) || !all && m != cur {
			continue
		}
		g.mapState = m
		c := wealth.Census{ThreatBase: wealth.StorytellerBase(g, len(g.itemIDs), len(g.bldgIDs))}
		for _, id := range g.pawnIDs {
			if colonistHere(g, id) {
				addPawn(&c, id, false)
			}
		}
		for _, id := range g.animalIDs {
			if a, ok := g.animals.Get(id); ok {
				c.AnimalPower += animal.ThreatPower(a)
			}
		}
		for _, r := range home[m.id] {
			if r.Beast != nil {
				wealth.AddStashedAnimal(&c.ThreatBase, *r.Beast)
			} else {
				addPawn(&c, r.ID, true)
			}
		}
		use(c)
	}
	g.mapState = cur
	if !all || g.worldSt == nil {
		return
	}
	for _, cv := range g.worldSt.caravans {
		var c wealth.Census
		seen := map[entity.EntityID]bool{}
		for _, r := range road[cv.C.ID] {
			if r.Beast != nil {
				wealth.AddStashedAnimal(&c.ThreatBase, *r.Beast)
				seen[r.ID] = true
			} else {
				addPawn(&c, r.ID, false)
			}
		}
		g.worldSt.ax9.CargoThreat(cv.C.ID, g.sk.content.Items, &c.ThreatBase, seen)
		if c.Residents > 0 || c.Wealth > 0 || c.AnimalPower > 0 {
			use(c)
		}
	}
}

func weightedThreat(g *Game, b difficulty.ThreatBase, ticks uint64) int {
	s := g.ng.setup.Difficulty
	return difficulty.ThreatPointsWeighted(s, b, ticks, g.clock.Calendar().YearDays(), g.ng.adapt.Factor(s))
}

// storyRaid sends a raid of pts threat points from a hostile faction
// (unattributed when none), sized by the faction's own multiplier and armed
// by its tech level; false when no raider came.
func (g *Game) storyRaid(day, pts int) bool {
	fi := faction.PickRaider(g.ng.factions, g.rng)
	return factionRaid(g, fi, difficulty.ScaleRaidPoints(pts, 100), day)
}

// factionRaid sends faction fi's raid (index; -1: unattributed) at pts
// times the faction's multiplier, armed by its tech level.
func factionRaid(g *Game, fi, pts, day int) bool {
	tech := faction.Industrial
	if fi >= 0 {
		d := faction.DefOf(g.ng.factions[fi].Type)
		if d.PointsPct > 0 {
			pts = difficulty.ScaleRaidPoints(pts, d.PointsPct)
		}
		tech = d.Tech
	}
	kit := g.raidKitOf(pts, tech)
	return g.raidFrom(kit, g.raid.Threats.Plan(day), fi+1)
}

// raiderName is a generated name for a faction's raider (no g.rng draw).
func (g *Game) raiderName(id entity.EntityID, f *faction.Faction) string {
	return pawngen.Generate(g.worldSeed()^raiderNameSalt, int(id), 0, pawngen.Rules{Tech: techTag(f), Backstories: backstoriesOf(g), Traits: traitsOf(g)}).DisplayName()
}

// raidMessage is the arrival message of a faction's raid.
func (g *Game) raidMessage(ex, ey, n int, plan raid.Plan, f *faction.Faction) {
	title, format, noun, verb := raid.FactionWarning(plan, n)
	from := raid.EdgeDir(g.grid.Width(), g.grid.Height(), ex, ey)
	fields := letters.RaidFields(n, f.Name, "faction", from)
	threatWithFields(g, letters.RaidCode(plan), title, ex, ey, format, fields, n, noun, f.Name, verb, from)
}

// --- the colony's hourly and daily upkeep -----------------------------------

// storyHour is a setup colony's hourly story work in its slot, after each
// map's losses and repelled raids (phaseSystems: cadence.OffStory, on the
// first map's pass): quest timers, the day rollover in the day's first
// slot and the scenario's scheduled incidents.
func storyHour(g *Game, now uint64) {
	// Run on the existing colony-only story cadence, twice per game day.
	// The scheduled boundary, rather than its offset slot, marks elapsed time.
	elapsed := g.rel(now + cadence.OffStory)
	if elapsed >= clock.TicksPerDay/2 && elapsed%(clock.TicksPerDay/2) == 0 {
		g.ng.adapt.HalfDay(g.ng.setup.Difficulty, int(elapsed/clock.TicksPerDay))
	}
	questTick(g, now, false) // quest_host.go (QST-1): due timers and offers
	switch (now + cadence.OffStory) % clock.TicksPerDay {
	case 0:
		g.dayRollover(now)
	case scheduledHour * clock.TicksPerHour:
		g.scheduledIncidents(now)
	}
}

// checkLosses notes colonists who died since the last check: the
// storyteller eases the pending threat (OnLoss). Adaptation is charged
// at the actual health transition, not in this periodic scan.
func (g *Game) checkLosses(now uint64) {
	for _, id := range g.pawnIDs {
		if g.isStashed(id) {
			continue // world_stash.go: away from the map, not lost
		}
		v, ok := g.vital.Get(id)
		if !ok || v.Status != health.Dead || slices.Contains(g.ng.dead, id) {
			continue
		}
		g.ng.dead = append(g.ng.dead, id)
		g.ng.lost = true
		if storyteller.OnLoss(g.storyProfile(), &g.ng.story, &g.ng.plan, g.relDay(now)) {
			g.nextThreat = g.ng.plan.AtTick + g.dayOff()
		}
	}
}

// dayRollover handles the existing loss signal, goodwill and offers.
func (g *Game) dayRollover(now uint64) {
	g.ng.lost = false
	g.factionsDay()         // factions.go
	questTick(g, now, true) // quest_host.go (QST-1): the offer cadence
}

// scheduledIncidents fires the scenario's incidents due today.
func (g *Game) scheduledIncidents(now uint64) {
	day := g.relDay(now)
	for _, s := range g.ng.fx.Scheduled {
		if day == s.FirstDay || s.EveryDays > 0 && day > s.FirstDay && (day-s.FirstDay)%s.EveryDays == 0 {
			g.fireScheduled(s.Incident, now)
		}
	}
}

// fireScheduled makes a scheduled incident happen by its scenario key.
func (g *Game) fireScheduled(key string, now uint64) {
	switch key {
	case "raid":
		g.storyRaid(g.relDay(now), g.threatPoints())
	case "predator_pack": // sized by the day's threat points (animal.PackSize, EAI-005)
		g.raid.Threats.WolfPack(g.raidPoints(g.relDay(now)))
	case "manhunter_pack":
		rageOf(g).ManhunterPack(g.raidPoints(g.relDay(now)))
	case "hive_swarm":
		g.raid.Hives.Swarm()
	default:
		g.FireIncident(key) // incidents.go: nature events by key
	}
}

// --- commands -----------------------------------------------------------------

// cmdSetStoryteller switches the storyteller (from the next plan on; the
// pending threat stays). A legacy colony keeps its fixed pacing.
func (g *Game) cmdSetStoryteller(key string) {
	if !g.hasSetup() {
		return
	}
	if _, ok := storyteller.ByKey(key); ok {
		g.ng.setup.Storyteller = key
	}
}

// cmdSetDifficulty sets the difficulty: a preset key or
// "custom:"+difficulty.Encode(settings). Invalid values are ignored.
func (g *Game) cmdSetDifficulty(v string) {
	if !g.hasSetup() {
		return
	}
	if rest, ok := strings.CutPrefix(v, "custom:"); ok {
		s, err := difficulty.Decode(rest)
		if err != nil {
			return
		}
		g.ng.setup.Preset, g.ng.setup.Difficulty = difficulty.Custom, s
		g.rebuildAllStats() // skills.go: pain tolerance and mood follow the difficulty
		return
	}
	p, ok := difficulty.Parse(v)
	if !ok || p == difficulty.Custom {
		return
	}
	g.ng.setup.Preset, g.ng.setup.Difficulty = p, difficulty.Of(p)
	g.rebuildAllStats()
}

// --- difficulty hooks -------------------------------------------------------

// infectionChance scales a wound's infection chance (permille) by the
// difficulty, clamped 0..1000 (MED-02); a legacy colony's is unchanged.
func (g *Game) infectionChance(base int32) int32 {
	if !g.hasSetup() {
		return base
	}
	return min(max(base*int32(g.ng.setup.Difficulty.Infection)/100, 0), 1000)
}

// revengePct scales an animal's revenge chance by the difficulty.
func (g *Game) revengePct(base int) int {
	if !g.hasSetup() {
		return base
	}
	return min(max(base*g.ng.setup.Difficulty.AnimalRevenge/100, 0), 100)
}

// predatorsHuntColonists: wild predators may pick colonists as prey.
func (g *Game) predatorsHuntColonists() bool {
	return !g.hasSetup() || g.ng.setup.Difficulty.PredatorsHuntColonists
}

// shipOf is game/ship (AX-8: the escape ship, its reactor's raids, the
// ending), built on first use off the world state.
func shipOf(g *Game) *ship.State {
	if w := g.worldS(); w.ship == nil {
		w.ship = &ship.State{}
	}
	return g.worldSt.ship
}

type shipHost struct{ g *Game } // ship.Host over the game

func (h shipHost) StructVer() uint64              { return h.g.structVer }
func (h shipHost) BuildingIDs() []entity.EntityID { return h.g.bldgIDs }
func (h shipHost) Building(id entity.EntityID) (construction.Building, bool) {
	return h.g.bldgs.Get(id)
}
func (h shipHost) Lift(id entity.EntityID) { h.g.removeBuildingAs(id, false) }
func (h shipHost) Ready(id entity.EntityID) bool {
	return h.g.standingPawn(id) && slices.Contains(h.g.pawnIDs, id)
}
func (h shipHost) Name(id entity.EntityID) string { return h.g.name(id) }
func (h shipHost) Spot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.reachableSpot(id, x, y)
}
func (h shipHost) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }
func (h shipHost) SetJob(id entity.EntityID, j job.Record)          { h.g.jobs.Set(id, j) }
func (h shipHost) Board(id, casket entity.EntityID) bool            { return h.g.stashPawn(id, casket) }
func (h shipHost) Wake(id, casket entity.EntityID) bool {
	x, y, ok := 0, 0, false
	if b, has := h.g.bldgs.Get(casket); has {
		x, y, ok = h.g.casketExit(b, nil) // casket.go: a free cell beside it
	} else if i, found := slices.BinarySearchFunc(h.g.worldS().stashed, id, stashCmp); found {
		x, y, ok = h.g.combat.Tiles.WalkableNear(h.g.worldSt.stashed[i].X, h.g.worldSt.stashed[i].Y, 3) // where the casket stood
	}
	if ok = ok && h.g.unstashPawn(id, x, y); ok {
		cryptoSicken(h.g, id) // food_poison.go: the wiki's cryptosleep sickness, as the stasis casket's
	}
	return ok
}
func (h shipHost) Escaped(id entity.EntityID) {
	h.g.setWhere(id, whereEscaped)
	senseOf(h.g).Gear.Spill(id) // game/sense (CSE-036): left where it boarded
}
func (h shipHost) Now() uint64  { return uint64(h.g.clock.Now()) }
func (h shipHost) Seed() uint64 { return h.g.streams.seed }
func (h shipHost) Raid(mech bool) {
	g, now, day := h.g, uint64(h.g.clock.Now()), int(uint64(h.g.clock.Now())/clock.TicksPerDay)
	switch {
	case g.hasSetup() && !g.ng.setup.Difficulty.BigThreats: // the difficulty's no big threats holds
	case mech && g.raid.Threats.MechRaid(g.raidPoints(day), now) > 0:
	case g.hasSetup():
		g.storyRaid(g.relDay(now), g.threatPoints()) // the storyteller's raid, at its threat points
	default:
		g.scaledRaid(day)
	}
}
func (h shipHost) Letter(title string, x, y int, danger bool, format string, args ...any) {
	if danger {
		h.g.threat(title, x, y, format, args...)
		return
	}
	h.g.post(MessageView{Tick: uint64(h.g.clock.Now()), Msg: msgfmt.Of(format, args...), X: x, Y: y, Letter: true, Title: title, Kind: letters.Good})
}
func (h shipHost) Message(x, y int, format string, args ...any) { h.g.message(x, y, format, args...) }
func (h shipHost) Changed()                                     { h.g.viewGen++ }

// DevWakeShip moves the reactors' countdowns and the next raid on by ticks (dev mode, journeys).
func DevWakeShip(g *Game, ticks uint64) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	shipOf(g).Rewind(ticks)
}

// ShipCasketFree and LayInShip are game/travel's Ships role (AX-8): carrying into a ship casket.
func (h travelHost) ShipCasketFree(id entity.EntityID) bool {
	return shipOf(h.g).CasketFree(shipHost{h.g}, id)
}
func (h travelHost) LayInShip(p, c entity.EntityID) bool {
	return shipOf(h.g).LayIn(shipHost{h.g}, p, c)
}

// WealthHistory is the current map's daily wealth samples, oldest first
// (game/wealth, EAI-014: the History window's graph).
func (g *Game) WealthHistory() []wealth.Sample { return slices.Clone(g.wealth.History()) }

// WealthOf is game/wealth's Source over the current map (EAI-014): the
// colony's worth in parts, in a fixed order (the goods on the map, the
// buildings, the tame animals, then what its colonists carry as one
// part), summing parts from..to-1; parts is how many there are now.
func (g *Game) WealthOf(from, to int) (sum, parts int) {
	ni, nb, na := len(g.itemIDs), len(g.bldgIDs), len(g.animalIDs)
	parts = ni + nb + na + 1
	from, to = max(from, 0), min(to, parts)
	for i := from; i < to; i++ {
		sum += wealthPart(g, i, ni, nb, na)
	}
	return sum, parts
}

// wealthPart is part i's worth (ni goods, nb buildings, na animals, then
// the gear).
func wealthPart(g *Game, i, ni, nb, na int) int {
	switch {
	case i < ni:
		if st, ok := g.stacks.Get(g.itemIDs[i]); ok {
			return g.sk.content.Items.StackValue(st) * st.Qty // physical condition and patched price
		}
	case i < ni+nb:
		if b, ok := g.bldgs.Get(g.bldgIDs[i-ni]); ok && !notColonyPiece(b) { // ruins and vents are no one's wealth
			return wealth.BuildingValue(b)
		}
	case i < ni+nb+na:
		if a, ok := g.animals.Get(g.animalIDs[i-ni-nb]); ok && a.Tame {
			return animal.DefOf(a.Species).MaxHP * 2
		}
	default:
		return senseOf(g).Gear.Wealth() // game/sense (CSE-036): what its colonists carry
	}
	return 0
}

// StoryDebugView is the storyteller's state for the developer overlay
// (EAI-0): the current map's queued events, the tick the next threat is
// due, today's threat points, the wealth, and the last threat fired
// (Book.LastFired; zero when none) at its colony-time tick (unlike NextThreat).
type StoryDebugView struct {
	Queue        []incident.Queued
	NextThreat   uint64
	Points       int
	Wealth       int
	LastThreat   incident.Def
	LastThreatAt uint64
}

// StoryDebug is the storyteller's state now (read only).
func (g *Game) StoryDebug() StoryDebugView {
	v := StoryDebugView{Queue: slices.Clone(g.incidents.Queue), NextThreat: g.nextThreat,
		Points: g.raidPoints(int(uint64(g.clock.Now()) / clock.TicksPerDay)), Wealth: g.wealth.Value()}
	v.LastThreat, v.LastThreatAt, _ = g.sk.content.Incidents.LastFired(g.incidents, incident.Threat)
	return v
}

// adaptationMember is a living player resident or travelling member.
// Core has no distinct quest-lodger or baby roster; ordinary visitors
// and prisoners are not treated as player colonists.
func adaptationMember(g *Game, id entity.EntityID) bool {
	v, ok := g.vital.Get(id)
	if !ok || v.Status == health.Dead || g.prison.Is(id) {
		return false
	}
	if p := g.personOf(id); p != nil {
		return p.Faction == 0 && (p.Where == whereColony || p.Where == whereCaravan)
	}
	return g.isPawn(id)
}

func adaptationLoss(g *Game, id entity.EntityID, cause health.Cause, death bool) {
	if !g.hasSetup() || cause == health.CauseExecution || cause == health.CauseEuthanasia || !adaptationMember(g, id) {
		return
	}
	v, _ := g.vital.Get(id)
	if !death && v.Status != health.Alive {
		return
	}
	// Stashing removes the resident from its map roster; the lists are
	// disjoint. Count only live eligible members at the event boundary.
	population, present := 0, false
	count := func(pid entity.EntityID) {
		if adaptationMember(g, pid) {
			population++
			present = present || pid == id
		}
	}
	for _, m := range g.maps {
		for _, pid := range m.pawnIDs {
			count(pid)
		}
	}
	if w := g.worldSt; w != nil {
		for _, r := range w.stashed {
			if r.Beast == nil && !r.Prisoner {
				count(r.ID)
			}
		}
	}
	if !present {
		return
	}
	g.ng.adapt.Lose(population, death, g.relDay(uint64(g.clock.Now())))
}

func canonicalAdaptation(a difficulty.Adapter, day int) difficulty.Adapter {
	a.Migrate(day)
	return a
}
