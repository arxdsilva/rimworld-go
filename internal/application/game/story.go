package game

import (
	"slices"
	"strings"

	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/storyteller"
)

// The storyteller of a setup colony (NG-1 §5.2, §5.3). Owned by SETUP.
// A legacy colony never reaches this file: raids.go keeps today's
// scheduler for it. Here the chosen storyteller's Plan decides when the
// next threat comes and how strong (the legacy profile makes exactly the
// pre-NG-1 draws), pickThreat's die is filtered by the difficulty, the
// scenario's disabled incidents and whether any hostile faction can raid,
// and raid points come from difficulty.ThreatPoints with adaptation.
// Colony time (rel, relDay) counts from the landing day.

const (
	// scheduledHour is when a scenario's scheduled incidents happen.
	scheduledHour = 10
	// raiderNameSalt keeps generated raider names apart from colonists'.
	raiderNameSalt = 0x7261696465 // "raide"
	// maxRaidersCap caps the raid size on the biggest maps.
	maxRaidersCap = 24
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
	g.storyThreat(g.rng.IntN(threatRolls), day, winter, pct, now)
	if second {
		g.storyThreat(g.rng.IntN(threatRolls), day, winter, pct, now)
	}
	g.setPlan(storyteller.Plan(g.storyProfile(), &g.ng.story, g.rel(now), g.rng))
}

// storyThreat fires the threat a roll of the die gives, if any is allowed.
func (g *Game) storyThreat(roll, day int, winter bool, pct int, now uint64) {
	k, ok := g.allowedThreat(roll, day, winter)
	if !ok {
		return
	}
	switch k {
	case threatPack:
		g.wolfPack(2 + g.rng.IntN(2))
	case threatManhunters:
		g.manhunterPack(3 + g.rng.IntN(3))
	case threatHive:
		g.hiveSwarm()
	case threatMad:
		if !g.madAnimal(now, false) && g.threatOK(threatRaid) { // incidents.go
			g.storyRaid(day, pct)
		}
	default:
		g.storyRaid(day, pct)
	}
}

// allowedThreat is pickThreat filtered for the colony: a threat the
// difficulty or scenario rules out (or a raid with no hostile faction to
// send it) becomes a pack, manhunters or a hive swarm by the same die,
// whichever is allowed first; ok is false when nothing is.
func (g *Game) allowedThreat(roll, day int, winter bool) (threatKind, bool) {
	k := pickThreat(roll, day, winter)
	if g.threatOK(k) {
		return k, true
	}
	alts := [...]threatKind{threatPack, threatManhunters, threatHive}
	for i := range alts {
		a := alts[(roll+i)%len(alts)]
		if a == threatHive && day < hiveFirstDay {
			continue
		}
		if g.threatOK(a) {
			return a, true
		}
	}
	return 0, false
}

// threatOK: the difficulty, the scenario and the factions allow threat k.
func (g *Game) threatOK(k threatKind) bool {
	s := g.ng.setup.Difficulty
	switch k {
	case threatRaid:
		return s.BigThreats && !g.incidentDisabled("raid") && g.hostileRaider()
	case threatHive:
		return s.BigThreats && !g.incidentDisabled("hive_swarm")
	case threatPack:
		return s.PredatorsHuntColonists && !g.incidentDisabled("predator_pack")
	case threatManhunters:
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
	years := float64(g.rel(uint64(g.clock.Now()))) / float64(clock.TicksPerYear)
	return difficulty.ThreatPoints(g.ng.setup.Difficulty, g.Wealth(), g.standingColonists(), years, int(g.ng.adapt.Value))
}

// maxRaiders is the largest raid: 8 on a legacy colony; a setup colony's
// grows with the map (8 + area/40000, at most 24).
func (g *Game) maxRaiders() int {
	if !g.hasSetup() {
		return maxScaledRaiders
	}
	return min(maxScaledRaiders+g.grid.Width()*g.grid.Height()/40000, maxRaidersCap)
}

// storyRaid sends a raid from a hostile faction (unattributed when none),
// sized by threat points times pct and the faction's own multiplier, and
// armed by its tech level.
func (g *Game) storyRaid(day, pct int) {
	fi := faction.PickRaider(g.ng.factions, g.rng)
	pts := g.threatPoints() * pct / 100
	rifles := true
	if fi >= 0 {
		d := faction.DefOf(g.ng.factions[fi].Type)
		if d.PointsPct > 0 {
			pts = pts * d.PointsPct / 100
		}
		rifles = d.Tech != faction.Neolithic
	}
	kit := g.raidKitOf(pts, rifles)
	g.raidFrom(kit, g.pickPlan(day), fi+1)
}

// raiderName is a generated name for a faction's raider (no g.rng draw).
func (g *Game) raiderName(id entity.EntityID, f *faction.Faction) string {
	tag := backstory.TagIndustrial
	if faction.DefOf(f.Type).Tech == faction.Neolithic {
		tag = backstory.TagNeolithic
	}
	p := pawngen.Generate(g.ng.setup.Seed^raiderNameSalt, int(id), 0, pawngen.Rules{Tech: tag})
	if p.Nick != "" {
		return p.Nick
	}
	return p.First
}

// edgeDir names the map edge nearest (x, y): "north", "south", ...
func (g *Game) edgeDir(x, y int) string {
	w, h := g.grid.Width(), g.grid.Height()
	best, dir := y, "north"
	if d := h - 1 - y; d < best {
		best, dir = d, "south"
	}
	if x < best {
		best, dir = x, "west"
	}
	if d := w - 1 - x; d < best {
		dir = "east"
	}
	return dir
}

// raidMessage is the arrival message of a faction's raid.
func (g *Game) raidMessage(ex, ey, n int, plan raidPlan, f *faction.Faction) {
	noun, verb := "raiders", "are"
	if n == 1 {
		noun, verb = "raider", "is"
	}
	from := g.edgeDir(ex, ey)
	switch plan {
	case planSiege:
		g.threat("Siege", ex, ey, "Raid! %d %s of %s %s coming from the %s. They look ready to dig in for a siege.", n, noun, f.Name, verb, from)
	case planSappers:
		g.threat("Sappers", ex, ey, "Sappers! %d %s of %s %s tunnelling in through the walls from the %s.", n, noun, f.Name, verb, from)
	default:
		g.threat("Raid", ex, ey, "Raid! %d %s of %s %s coming from the %s.", n, noun, f.Name, verb, from)
	}
}

// --- the colony's hourly and daily upkeep -----------------------------------

// setupTick runs every tick for a setup colony (phaseThreats): the
// arrival, then hourly losses and raid outcomes, the day rollover at
// midnight and the scenario's scheduled incidents.
func (g *Game) setupTick(now uint64) {
	g.arrivalTick(now) // arrival.go
	if now%clock.TicksPerHour != 0 {
		return
	}
	g.checkLosses(now)
	g.checkRaidsOver(now) // factions.go
	switch now % clock.TicksPerDay {
	case 0:
		g.dayRollover(now)
	case scheduledHour * clock.TicksPerHour:
		g.scheduledIncidents(now)
	}
}

// checkLosses notes colonists who died since the last check: the
// storyteller eases the pending threat (OnLoss) and adaptation counts
// the loss at the day rollover.
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

// dayRollover: adaptation, faction goodwill drift, extreme weather.
func (g *Game) dayRollover(now uint64) {
	g.ng.adapt.Day(g.ng.setup.Difficulty, g.ng.lost)
	g.ng.lost = false
	g.factionsDay()   // factions.go
	g.extremeDay(now) // weather_ng1.go
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
		g.storyRaid(g.relDay(now), 100)
	case "predator_pack":
		g.wolfPack(2 + g.rng.IntN(2))
	case "manhunter_pack":
		g.manhunterPack(3 + g.rng.IntN(3))
	case "hive_swarm":
		g.hiveSwarm()
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

// infectionPct scales a wound's infection chance by the difficulty
// (clamped 0..100); a legacy colony's is unchanged.
func (g *Game) infectionPct(base int) int {
	if !g.hasSetup() {
		return base
	}
	return min(max(base*g.ng.setup.Difficulty.Infection/100, 0), 100)
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
