package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal handling. Hunters also tame: an animal marked for taming holds
// still while a handler coaxes it, then is tamed (or not; a botched try
// can enrage it). Tame animals have names, live in animal pens when there
// are any (else around the colony), pass through the colony's doors and
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
		if p, _ := g.positions.Get(id); p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY {
			a.TameMark = on
			g.animals.Set(id, a)
		}
	}
}

// beingTamed: a handler has claimed this animal for a taming, milking or
// shearing job.
func (g *Game) beingTamed(id entity.EntityID) bool {
	owner, ok := g.reserve.EntityOwner(id)
	if !ok {
		return false
	}
	j, _ := g.jobs.Get(owner)
	return (j.Kind == job.Tame || j.Kind == job.Milk || j.Kind == job.Shear) && j.Target == id
}

// assignTame sends a hunter to the nearest animal marked for taming.
func (g *Game) assignTame(id entity.EntityID) bool {
	if !g.anyAnimal(func(_ entity.EntityID, a animal.State) bool { return a.TameMark && !a.Tame }) {
		return false // jobindex.go: nothing marked for taming
	}
	now := uint64(g.clock.Now())
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, aid := range g.animalIDs {
		a, ok := g.animals.Get(aid)
		if !ok || !a.TameMark || a.Tame || a.Enraged(now) || now < a.TameRetry || animal.DefOf(a.Species).TameChance == 0 {
			continue
		}
		if _, taken := g.reserve.EntityOwner(aid); taken {
			continue
		}
		p, _ := g.positions.Get(aid)
		if d := pathing.Dist(pos.X, pos.Y, p.X, p.Y); d < bestD || (d == bestD && aid < best) {
			best, bestD = aid, d
		}
	}
	if best == 0 {
		return false
	}
	p, _ := g.positions.Get(best)
	tx, ty, ok := g.besideSpot(id, p.X, p.Y)
	if !ok {
		return false
	}
	return g.startJob(id, job.Record{Kind: job.Tame, Target: best, TX: tx, TY: ty, WorkLeft: animal.TameWork})
}

func (g *Game) doTame(id entity.EntityID, j job.Record) {
	a, ok := g.animals.Get(j.Target)
	pos, _ := g.positions.Get(id)
	ap, _ := g.positions.Get(j.Target)
	switch {
	case !ok || a.Tame || !a.TameMark:
		j.State = job.Complete
		g.jobs.Set(id, j)
		return
	case weapon.Distance(pos.X, pos.Y, ap.X, ap.Y) > 1:
		j.State, j.Fail = job.Fail, job.TargetGone
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skAnimals, true) // skills.go
	if u == 0 {
		return
	}
	if j.WorkLeft -= u; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	def := animal.DefOf(a.Species)
	now := uint64(g.clock.Now())
	tamed := g.rng.IntN(100) < g.tameChance(id, def.TameChance) // skills.go
	if tamed {
		a.Tame, a.TameMark, a.Designated = true, false, false
		a.Name = petNames[g.rng.IntN(len(petNames))]
		a.Mode, a.Target, a.Attacker, a.Path, a.PathI = animal.Wander, 0, 0, nil, 0
		g.message(ap.X, ap.Y, "%s tamed a %s. Its name is %s.", g.name(id), a.Species.Label(), a.Name)
		g.logEvent(j.Target, logEntry{Kind: logTamedBy, Other: id, Name: g.name(id)})
		g.logEvent(id, logEntry{Kind: logTamed, Other: j.Target, Name: a.Name + " the " + a.Species.Label()})
	} else {
		a.TameRetry = now + animal.TameRetryTicks
		g.logEvent(j.Target, logEntry{Kind: logResistedTaming, Other: id, Name: g.name(id)})
		g.logEvent(id, logEntry{Kind: logFailedTame, Other: j.Target, Name: a.Species.Label()})
		if g.rng.IntN(100) < g.revengePct(def.TameRevenge) { // story.go: difficulty (NG-1)
			g.enrage(j.Target, &a, id, now)
		} else {
			g.message(ap.X, ap.Y, "%s failed to tame the %s.", g.name(id), a.Species.Label())
		}
	}
	g.animals.Set(j.Target, a)
	if tamed {
		g.onTamed(id, j.Target) // bonds.go: a chance of a bond with the tamer
	}
	j.State = job.Complete
	g.jobs.Set(id, j)
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
			g.strike(id, a.Target, def.Attack)
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

// wanderTame: tame animals keep to the pens (or near the colony).
func (g *Game) wanderTame(id entity.EntityID, a *animal.State, pos Position, def animal.Def, now uint64) {
	a.Target = 0
	if a.PathI < len(a.Path) || now < a.NextThink {
		return
	}
	a.NextThink = now + 240 + uint64(g.rng.IntN(480))
	if p, ok := g.nearestPen(pos); ok {
		if at, _ := g.zones.At(pos.X, pos.Y); at != p {
			gx, gy := g.penGoal(p)
			g.animalGoTo(id, a, gx, gy)
			return
		}
		r := def.WanderRadius
		b, _ := g.zones.Bounds(p)
		x := clampInt(pos.X+g.rng.IntN(2*r+1)-r, b.MinX, b.MaxX)
		y := clampInt(pos.Y+g.rng.IntN(2*r+1)-r, b.MinY, b.MaxY)
		if at, _ := g.zones.At(x, y); at == p { // pens need not be rectangles
			g.animalGoTo(id, a, x, y)
		}
		return
	}
	// no pen: stay near the colony
	hx, hy := g.home()
	r := def.WanderRadius
	if weapon.Distance(pos.X, pos.Y, hx, hy) > 12 {
		g.animalGoTo(id, a, hx+g.rng.IntN(7)-3, hy+g.rng.IntN(7)-3)
		return
	}
	g.animalGoTo(id, a, pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r)
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
// bed, else the map centre.
func (g *Game) home() (int, int) {
	for _, id := range g.pawnIDs {
		if bid := g.bedOf(id); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			return b.X, b.Y
		}
	}
	return g.grid.Width() / 2, g.grid.Height() / 2
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
	name := pluralLabel(s)
	if spawned == 1 {
		name = s.Label()
	}
	g.threat(ex, ey, "A manhunter pack of %d %s is charging the colony!", spawned, name)
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
