package game

import (
	"fmt"

	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal handling. Hunters also tame: an animal marked for taming holds
// still while a handler coaxes it, then is tamed (or not; a botched try
// can enrage it). Tame animals have names; livestock live in pens (pens.go)
// and the rest around the colony; they pass through the colony's doors and
// never spring its traps; tame ridgewolves guard the colony against
// raiders and hostile animals. Wild animals hurt by colonists may go
// manhunter, hunting every colonist for a day; the storyteller also sends
// whole manhunter packs.

const (
	guardRadius = 10
	// pathRetry: after a failed path search an animal waits this long
	// before searching again.
	pathRetry = 90
)

var petNames = []string{"Biscuit", "Pepper", "Juniper", "Ash", "Clover", "Moss", "Tansy", "Rook", "Sorrel",
	"Bramble", "Fennel", "Hazel", "Ember", "Thistle", "Burr", "Wren"}

func (g *Game) markTame(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok || on && (a.Tame || animal.DefOf(a.Species).TameChance == 0) {
			continue // hive insects are never tamed
		}
		if p, _ := g.positions.Get(id); p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY && !g.fogged(p.X, p.Y) {
			a.TameMark = on
			g.animals.Set(id, a)
		}
	}
}

// beingTamed: a handler has claimed this animal for a taming, milking,
// shearing, training, slaughter or feeding job, or a doctor for tending it.
func (g *Game) beingTamed(id entity.EntityID) bool {
	owner, ok := g.reserve.EntityOwner(id)
	if !ok {
		return false
	}
	j, _ := g.jobs.Get(owner)
	switch j.Kind {
	case job.Tame, job.Milk, job.Shear, job.Train, job.TendAnimal, job.Rope:
		return j.Target == id
	}
	return herd.Holds(j, id) // FOOD-3: a slaughter or a feeding holds it too
}

// herdOf is the herd (game/herd, FOOD-3: taming, milking and shearing,
// slaughter, auto-slaughter, feeding downed animals, hunters' kills),
// built on first use over the settings saved in the wildlife state (a
// load replaces both).
func (g *Game) herdOf() *herd.Herd {
	if g.wl.herd == nil {
		g.wl.herd = herd.New(herdHost{g}, &g.wl.Herd)
	}
	return g.wl.herd
}

// herdHost implements herd.Host over the game.
type herdHost struct{ g *Game }

func (h herdHost) Now() uint64                    { return uint64(h.g.clock.Now()) }
func (h herdHost) Roll(n int) int                 { return h.g.rng.IntN(n) }
func (h herdHost) Message(x, y int, text string)  { h.g.message(x, y, "%s", text) }
func (h herdHost) Name(id entity.EntityID) string { return h.g.name(id) }
func (h herdHost) Animals(id entity.EntityID) int { return int(h.g.skillOf(id, skill.Animals).Level) }
func (h herdHost) AnimalIDs() []entity.EntityID   { return h.g.animalIDs }
func (h herdHost) Animal(id entity.EntityID) (animal.State, bool) {
	return h.g.animals.Get(id)
}
func (h herdHost) SetAnimal(id entity.EntityID, a animal.State) { h.g.animals.Set(id, a) }
func (h herdHost) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h herdHost) AnyAnimal(keep func(entity.EntityID, *animal.State) bool) bool {
	return h.g.anyAnimal(keep) // jobindex.go
}
func (h herdHost) Nearest(id entity.EntityID, keep func(entity.EntityID, *animal.State) bool) (entity.EntityID, int, int, bool) {
	g := h.g
	pos, _ := g.positions.Get(id)
	aid, p, ok := g.nearestWhere(pos.X, pos.Y, max(g.grid.Width(), g.grid.Height()), ckAnimal, func(o entity.EntityID, _ Position) bool {
		a := g.animals.Ref(o)
		return a != nil && keep(o, a)
	})
	return aid, p.X, p.Y, ok
}
func (h herdHost) Kill(aid, by entity.EntityID)                     { h.g.killAnimal(aid, by) }
func (h herdHost) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }
func (h herdHost) SetJob(id entity.EntityID, j job.Record)          { h.g.jobs.Set(id, j) }
func (h herdHost) Job(id entity.EntityID) (job.Record, bool)        { return h.g.jobs.Get(id) }
func (h herdHost) Owner(t entity.EntityID) (entity.EntityID, bool)  { return h.g.reserve.EntityOwner(t) }
func (h herdHost) Claim(id, t entity.EntityID) bool                 { return h.g.reserve.TryEntity(id, t) }
func (h herdHost) BesideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.besideSpot(id, x, y)
}
func (h herdHost) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, x, y)
}
func (h herdHost) Work(id entity.EntityID) int                  { return h.g.workProgress(id, skAnimals, true) }
func (h herdHost) Stack(sid entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(sid) }
func (h herdHost) Delete(sid entity.EntityID)                   { h.g.deleteItem(sid) }
func (h herdHost) HaulToStorage(id, sid entity.EntityID) bool {
	return h.g.haulToStockpile(id, h.pos(id), []entity.EntityID{sid}, 0)
}
func (h herdHost) pos(id entity.EntityID) Position   { p, _ := h.g.positions.Get(id); return p }
func (h herdHost) FedAnimal(id, aid entity.EntityID) { h.g.logPair(aid, id, logFedBy, logFed, 0, 0) }
func (h herdHost) Policy(aid entity.EntityID) func(item.Kind) bool {
	return h.g.sk.diet.Menu(aid, true).Has
}

// NearestFood is the nearest free, fresh food stack accept takes for id.
func (h herdHost) NearestFood(id entity.EntityID, accept func(item.Kind) bool) (entity.EntityID, int, int) {
	g := h.g
	return g.nearestFree(id, func(k item.Kind) bool { return k.IsFood() && accept(k) }) // orders.go
}

// Take: id, at stack sid, picks up qty of it, split off a bigger stack.
func (h herdHost) Take(id, sid entity.EntityID, qty int) (entity.EntityID, bool) {
	g := h.g
	st, ok := g.stacks.Get(sid)
	if !ok || st.Carrier != 0 || st.Qty <= 0 || st.Rotten() {
		return 0, false
	}
	if st.Qty <= qty {
		st.Carrier = id
		g.putStack(sid, st)
		return sid, true
	}
	st.Qty -= qty
	g.putStack(sid, st)
	carry := g.entities.Next()
	g.itemIDs = append(g.itemIDs, carry)
	st.Qty, st.Carrier = qty, id
	g.putStack(carry, st)
	return carry, true
}

// Drop spawns st at its tile, onto any stack it merges with.
func (h herdHost) Drop(st item.Stack) {
	g := h.g
	if sid, err := g.SpawnStack(st.Kind, st.Qty, st.X, st.Y); err == nil {
		s, _ := g.stacks.Get(sid)
		s.Animal = st.Animal
		g.mergeOrSet(sid, s)
	}
}

// Tamed: handler id tamed animal aid: it joins the colony, named, and may
// bond with its tamer.
func (h herdHost) Tamed(id, aid entity.EntityID) {
	g := h.g
	a, _ := g.animals.Get(aid)
	ap, _ := g.positions.Get(aid)
	a.Tame, a.TameMark, a.Designated = true, false, false
	a.Name = petNames[g.rng.IntN(len(petNames))]
	a.Mode, a.Target, a.Attacker, a.Path, a.PathI = animal.Wander, 0, 0, nil, 0
	g.message(ap.X, ap.Y, "%s tamed a %s. Its name is %s.", g.name(id), a.Species.Label(), a.Name)
	g.logEvent(aid, logEntry{Kind: logTamedBy, Other: id, Name: g.name(id)})
	g.logEvent(id, logEntry{Kind: logTamed, Other: aid, Name: a.Name + " the " + a.Species.Label()})
	g.animals.Set(aid, a)
	g.onTamed(id, aid) // bonds.go: a chance of a bond with the tamer
}

// TameFailed: id failed to tame aid, which waits out its retry and may
// go manhunter at its species' wiki chance (story.go: the difficulty).
func (h herdHost) TameFailed(id, aid entity.EntityID) {
	g := h.g
	a, _ := g.animals.Get(aid)
	ap, _ := g.positions.Get(aid)
	now := uint64(g.clock.Now())
	a.TameRetry = now + animal.TameRetryTicks
	g.logEvent(aid, logEntry{Kind: logResistedTaming, Other: id, Name: g.name(id)})
	g.logEvent(id, logEntry{Kind: logFailedTame, Other: aid, Name: a.Species.Label()})
	if g.rng.IntN(100) < g.revengePct(animal.DefOf(a.Species).TameRevenge) {
		g.enrage(aid, &a, id, now)
	} else {
		g.message(ap.X, ap.Y, "%s failed to tame the %s.", g.name(id), a.Species.Label())
	}
	g.animals.Set(aid, a)
}

// enrage turns a wild animal manhunter for a day, going for who; a
// legendary's rage calls every legendary nearby (wildlife.go).
func (g *Game) enrage(id entity.EntityID, a *animal.State, who entity.EntityID, now uint64) {
	if a.Tame || a.Enraged(now) {
		return
	}
	a.Manhunter = now + animal.ManhunterTicks
	a.Mode, a.Target, a.TameMark, a.FleeUntil = animal.Attack, who, false, 0
	a.Path, a.PathI = nil, 0
	if a.Downed { // animal_health.go: enraged, but it cannot hunt until it stands
		a.Mode, a.Target = animal.Wander, 0
	}
	g.logEvent(id, logEntry{Kind: logManhunter, Other: who})
	p, _ := g.positions.Get(id)
	g.post(MessageView{Tick: now, Text: fmt.Sprintf("A %s has gone manhunter!", a.Species.Label()),
		X: p.X, Y: p.Y, Danger: true, Focus: id})
	if animal.DefOf(a.Species).Legendary {
		g.rallyLegendaries(id, who, now)
	}
}

// provoked: a wild animal hurt by a colonist may go manhunter.
func (g *Game) provoked(id entity.EntityID, a *animal.State, by entity.EntityID) {
	if a.Tame || a.Dead() || by == 0 || !g.isPawn(by) || g.isRaider(by) {
		return
	}
	now := uint64(g.clock.Now())
	if !a.Enraged(now) && g.rng.IntN(100) < g.revengePct(animal.DefOf(a.Species).Revenge) { // story.go: difficulty (NG-1)
		g.enrage(id, a, by, now)
	}
}

// thinkManhunter hunts the nearest standing colonist anywhere on the map.
// It reports false once the rage has passed.
func (g *Game) thinkManhunter(id entity.EntityID, a *animal.State, now uint64) bool {
	if !a.Enraged(now) {
		if a.Manhunter != 0 {
			a.Manhunter, a.Target, a.Mode = 0, 0, animal.Wander
		}
		return false
	}
	if a.Target == 0 || !g.standingPawn(a.Target) || g.isRaider(a.Target) {
		pos, _ := g.positions.Get(id)
		a.Target, _, _ = g.nearestColonist(pos)
	}
	if a.Target == 0 {
		return false // nobody left standing: calm down for now
	}
	g.chase(id, a, now)
	return true
}

// chase closes on a.Target and bites on cooldown.
func (g *Game) chase(id entity.EntityID, a *animal.State, now uint64) {
	def := animal.DefOf(a.Species)
	pos, _ := g.positions.Get(id)
	a.Mode = animal.Attack
	tp, _ := g.positions.Get(a.Target)
	if weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) <= 1 {
		a.Path, a.PathI = nil, 0
		if a.Cooldown == 0 {
			a.Cooldown = def.Cooldown
			if a.HuntSince != 0 {
				a.HuntSince = now // a strike: the hunt is still on (T5)
			}
			g.animals.Set(id, *a)
			g.combat.Strikes.Strike(id, a.Target, def.Attack)
			if cur, ok := g.animals.Get(id); ok {
				*a = cur // a kill updates this animal (hunger, target)
			}
		}
		return
	}
	if now < a.RetryAt && a.PathI >= len(a.Path) {
		return // backing off after a failed path search
	}
	if a.PathI >= len(a.Path) || weapon.Distance(a.GoalX, a.GoalY, tp.X, tp.Y) > 1 || (now+uint64(id))%30 == 0 {
		g.animalGoTo(id, a, tp.X, tp.Y)
	}
}

// guardTarget is the nearest threat a tame guard animal should fight: a
// standing raider, or a wild animal attacking the colony.
func (g *Game) guardTarget(id entity.EntityID, pos Position) entity.EntityID {
	now := uint64(g.clock.Now())
	best, _, _ := g.nearestWhere(pos.X, pos.Y, guardRadius, ckRaider|ckAnimal, func(o entity.EntityID, _ Position) bool {
		if g.isRaider(o) {
			return g.standingRaider(o)
		}
		a, ok := g.animals.Get(o)
		return o != id && ok && !a.Tame && !a.Dead() && (g.hostile(a) || a.Enraged(now))
	})
	return best
}

// guardCheck: a tame guard (a predator) fights off threats near it,
// looking for one every animal.ThreatEvery ticks. It reports true while it
// fights.
func (g *Game) guardCheck(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	if a.Target != 0 && !g.combatAlive(a.Target) {
		a.Target = 0
	}
	if a.Target == 0 && (now+uint64(id))%animal.ThreatEvery == 0 {
		a.Target = g.guardTarget(id, pos)
	}
	if a.Target == 0 {
		if a.Mode == animal.Attack {
			a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
		}
		return false
	}
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0 // a sleeping guard wakes
	g.chase(id, a, now)
	return true
}

// guardBreaksHunt: a tame predator out hunting (PLAY-017) drops the hunt
// for a threat its guard duty would fight, looked for on guardCheck's
// cadence; guardCheck then takes the threat on.
func (g *Game) guardBreaksHunt(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	if (now+uint64(id))%animal.ThreatEvery != 0 {
		return
	}
	if t := g.guardTarget(id, pos); t != 0 {
		g.endFight(a) // animal_needs.go
		a.Target = t
	}
}

// wanderTame: livestock keep to their pen, or roam when they have none
// (pens.go); other tame animals stay near the colony, inside their
// allowed area (areas.go inArea).
func (g *Game) wanderTame(id entity.EntityID, a *animal.State, pos Position, def animal.Def, now uint64) {
	a.Target = 0
	if a.PathI < len(a.Path) || now < a.NextThink {
		return
	}
	a.NextThink = now + 240 + uint64(g.rng.IntN(480))
	if penned(a) {
		r := penRoamRadius
		_, c := g.homePen(id, a, pos)
		if c != nil {
			r = def.WanderRadius
		}
		x, y := pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r
		x, y = clampInt(x, 0, g.grid.Width()-1), clampInt(y, 0, g.grid.Height()-1)
		if c != nil && !g.inPenCells(c, x, y) { // pens need not be rectangles
			return
		}
		if c == nil {
			// roaming: never towards a wild predator in sight (it would
			// only run again: no flee/roam flapping)
			if pp, near := g.nearestPredator(id, pos, r+def.FleeRadius+animal.FleeRelease); near &&
				cellDistSq(Position{X: x, Y: y}, pp) < cellDistSq(pos, pp) {
				return
			}
		}
		g.animalGoTo(id, a, x, y)
		return
	}
	// stay near the colony, inside the allowed area
	hx, hy := g.home()
	r := def.WanderRadius
	var x, y int
	if weapon.Distance(pos.X, pos.Y, hx, hy) > 12 {
		x, y = hx+g.rng.IntN(7)-3, hy+g.rng.IntN(7)-3
	} else {
		x, y = pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r
	}
	if g.inArea(id, x, y) {
		g.animalGoTo(id, a, x, y)
	}
}

// combatAlive: a raider still standing or a living animal.
func (g *Game) combatAlive(id entity.EntityID) bool {
	if o, ok := g.animals.Get(id); ok {
		return !o.Dead()
	}
	return g.standingRaider(id)
}

// nearestPredator: the closest wild predator within r (tame grazers do
// not fear people or the colony's own wolves).
func (g *Game) nearestPredator(id entity.EntityID, pos Position, r int) (Position, bool) {
	_, p, ok := g.nearestWhere(pos.X, pos.Y, r, ckAnimal, func(o entity.EntityID, _ Position) bool {
		a, ok := g.animals.Get(o)
		return o != id && ok && !a.Tame && animal.DefOf(a.Species).Predator
	})
	return p, ok
}

// home is roughly where the colony lives: its first standing colonist's
// bed, else the landing spot (the map centre for legacy colonies).
func (g *Game) home() (int, int) {
	for _, id := range g.pawnIDs {
		if bid := g.bedOf(id); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			return b.X, b.Y
		}
	}
	return g.landingSpot() // setup.go
}

// bedOf is the bed a colonist has claimed (0: none).
func (g *Game) bedOf(id entity.EntityID) entity.EntityID {
	for _, bid := range g.bldgIDs {
		if b, ok := g.bldgs.Get(bid); ok && b.Kind.Sleeps() && b.SlotOf(uint64(id)) >= 0 {
			return bid
		}
	}
	return 0
}

// manhunterPack sends n enraged animals of one season-fitting grazer or
// territorial species in from a map edge (half as many of a large one).
func (g *Game) manhunterPack(n int) {
	s, ok := g.pickSpecies(manhunterSpecies)
	if !ok {
		return
	}
	ex, ey, ok := g.edgeTile()
	if !ok {
		return
	}
	if animal.RosterOf(s).Size >= animal.SizeL {
		n = (n + 1) / 2
	}
	now := uint64(g.clock.Now())
	spawned := 0
	for i := 0; i < n; i++ {
		for try := 0; try < 10; try++ {
			x, y := ex+g.rng.IntN(5)-2, ey+g.rng.IntN(5)-2
			if !g.grid.InBounds(x, y) {
				continue
			}
			if id, err := g.spawnAnimal(s, x, y); err == nil {
				a, _ := g.animals.Get(id)
				a.Manhunter, a.Mode = now+animal.ManhunterTicks, animal.Attack
				g.animals.Set(id, a)
				spawned++
				break
			}
		}
	}
	if spawned == 0 {
		return
	}
	name := animal.Plural(s)
	if spawned == 1 {
		name = s.Label()
	}
	g.threat("Manhunter pack", ex, ey, "A manhunter pack of %d %s is charging the colony!", spawned, name)
}

// manhunterSpecies: who forms manhunter packs (grazers and territorial
// animals).
func manhunterSpecies(s animal.Species) bool {
	b := animal.RosterOf(s).Behaviour
	return b == animal.Grazer || b == animal.Territorial
}

// ManhunterPack is the dev hook for a manhunter pack.
func (g *Game) ManhunterPack(n int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.manhunterPack(n)
}

// releaseAnimal turns a tame animal loose.
func (g *Game) releaseAnimal(id entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame {
		return
	}
	p, _ := g.positions.Get(id)
	g.message(p.X, p.Y, "%s the %s was released into the wild.", a.Name, a.Species.Label())
	g.unbondReleased(id, &a) // bonds.go: the bonded colonist's regret
	a.Tame, a.Name, a.Target, a.Mode = false, "", 0, animal.Wander
	g.animals.Set(id, a)
}

// TameAnimal makes an animal tame on the spot (dev scenarios, fixtures).
func (g *Game) TameAnimal(id entity.EntityID, name string) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if a, ok := g.animals.Get(id); ok {
		a.Tame, a.Name, a.TameMark, a.Manhunter = true, name, false, 0
		g.animals.Set(id, a)
	}
}
