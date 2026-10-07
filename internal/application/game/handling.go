package game

import (
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

// pen is an animal pen zone (inclusive rectangle).
type pen struct{ X1, Y1, X2, Y2 int }

func (p pen) contains(x, y int) bool { return x >= p.X1 && x <= p.X2 && y >= p.Y1 && y <= p.Y2 }

const guardRadius = 10

var petNames = []string{"Biscuit", "Pepper", "Juniper", "Ash", "Clover", "Moss", "Tansy", "Rook", "Sorrel",
	"Bramble", "Fennel", "Hazel", "Ember", "Thistle", "Burr", "Wren"}

func (g *Game) penViews() [][4]int {
	out := make([][4]int, 0, len(g.pens))
	for _, p := range g.pens {
		out = append(out, [4]int{p.X1, p.Y1, p.X2, p.Y2})
	}
	return out
}

func (g *Game) addPen(x1, y1, x2, y2 int) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	g.pens = append(g.pens, pen{minX, minY, maxX, maxY})
}

// cancelPens drops pens overlapping the rectangle.
func (g *Game) cancelPens(x1, y1, x2, y2 int) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	kept := g.pens[:0]
	for _, p := range g.pens {
		if p.X2 < minX || p.X1 > maxX || p.Y2 < minY || p.Y1 > maxY {
			kept = append(kept, p)
		}
	}
	g.pens = kept
}

func (g *Game) markTame(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok || a.Tame && on {
			continue
		}
		if p, _ := g.positions.Get(id); p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY {
			a.TameMark = on
			g.animals.Set(id, a)
		}
	}
}

// beingTamed: a handler has claimed this animal for a taming job.
func (g *Game) beingTamed(id entity.EntityID) bool {
	owner, ok := g.reserve.EntityOwner(id)
	if !ok {
		return false
	}
	j, _ := g.jobs.Get(owner)
	return j.Kind == job.Tame && j.Target == id
}

// assignTame sends a hunter to the nearest animal marked for taming.
func (g *Game) assignTame(id entity.EntityID) bool {
	now := uint64(g.clock.Now())
	pos, _ := g.positions.Get(id)
	var best entity.EntityID
	bestD := 1 << 30
	for _, aid := range g.animalIDs {
		a, ok := g.animals.Get(aid)
		if !ok || !a.TameMark || a.Tame || a.Enraged(now) || now < a.TameRetry {
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
	if !g.workTick(id) {
		return
	}
	if j.WorkLeft--; j.WorkLeft > 0 {
		g.jobs.Set(id, j)
		return
	}
	def := animal.DefOf(a.Species)
	now := uint64(g.clock.Now())
	if g.rng.IntN(100) < def.TameChance {
		a.Tame, a.TameMark, a.Designated = true, false, false
		a.Name = petNames[g.rng.IntN(len(petNames))]
		a.Mode, a.Target, a.Attacker, a.Path, a.PathI = animal.Wander, 0, 0, nil, 0
		g.message(ap.X, ap.Y, "%s tamed a %s. Its name is %s.", g.name(id), a.Species, a.Name)
	} else {
		a.TameRetry = now + animal.TameRetryTicks
		if g.rng.IntN(100) < def.TameRevenge {
			g.enrage(j.Target, &a, id, now)
		} else {
			g.message(ap.X, ap.Y, "%s failed to tame the %s.", g.name(id), a.Species)
		}
	}
	g.animals.Set(j.Target, a)
	j.State = job.Complete
	g.jobs.Set(id, j)
}

// enrage turns a wild animal manhunter for a day, going for who.
func (g *Game) enrage(id entity.EntityID, a *animal.State, who entity.EntityID, now uint64) {
	if a.Tame || a.Enraged(now) {
		return
	}
	a.Manhunter = now + animal.ManhunterTicks
	a.Mode, a.Target, a.TameMark, a.FleeUntil = animal.Attack, who, false, 0
	a.Path, a.PathI = nil, 0
	p, _ := g.positions.Get(id)
	g.threat(p.X, p.Y, "A %s has gone manhunter!", a.Species)
}

// provoked: a wild animal hurt by a colonist may go manhunter.
func (g *Game) provoked(id entity.EntityID, a *animal.State, by entity.EntityID) {
	if a.Tame || a.Dead() || by == 0 || !g.isPawn(by) || g.isRaider(by) {
		return
	}
	now := uint64(g.clock.Now())
	if !a.Enraged(now) && g.rng.IntN(100) < animal.DefOf(a.Species).Revenge {
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
			a.Cooldown = weapon.DefOf(def.Attack).Cooldown
			g.animals.Set(id, *a)
			g.strike(id, a.Target, def.Attack)
			if cur, ok := g.animals.Get(id); ok {
				*a = cur // a kill updates this animal (hunger, target)
			}
		}
		return
	}
	if a.PathI >= len(a.Path) || weapon.Distance(a.GoalX, a.GoalY, tp.X, tp.Y) > 1 || (now+uint64(id))%30 == 0 {
		g.animalGoTo(id, a, tp.X, tp.Y)
	}
}

// guardTarget is the nearest threat a tame guard animal should fight: a
// standing raider, or a wild animal attacking the colony.
func (g *Game) guardTarget(id entity.EntityID, pos Position) entity.EntityID {
	best, bestD := entity.EntityID(0), guardRadius+1
	for _, rid := range g.raiderIDs {
		if !g.standingRaider(rid) {
			continue
		}
		p, _ := g.positions.Get(rid)
		if d := weapon.Distance(pos.X, pos.Y, p.X, p.Y); d < bestD {
			best, bestD = rid, d
		}
	}
	now := uint64(g.clock.Now())
	for _, aid := range g.animalIDs {
		o, ok := g.animals.Get(aid)
		if aid == id || !ok || o.Tame || o.Dead() || !(g.hostile(o) || o.Enraged(now)) {
			continue
		}
		p, _ := g.positions.Get(aid)
		if d := weapon.Distance(pos.X, pos.Y, p.X, p.Y); d < bestD {
			best, bestD = aid, d
		}
	}
	return best
}

// thinkTame: guards fight off threats, everyone else keeps to the pens
// (or near the colony) and shies away from predators.
func (g *Game) thinkTame(id entity.EntityID, a *animal.State, now uint64) {
	def := animal.DefOf(a.Species)
	pos, _ := g.positions.Get(id)
	if def.Predator {
		if a.Target != 0 && !g.combatAlive(a.Target) {
			a.Target = 0
		}
		if a.Target == 0 {
			a.Target = g.guardTarget(id, pos)
		}
		if a.Target != 0 {
			g.chase(id, a, now)
			return
		}
	} else if tp, threat := g.nearestPredator(id, pos, def.FleeRadius); threat {
		if a.Mode != animal.Flee || a.PathI >= len(a.Path) || (now+uint64(id))%40 == 0 {
			a.Mode = animal.Flee
			fx, fy := animal.FleeGoal(pos.X, pos.Y, tp.X, tp.Y, 6)
			g.animalGoTo(id, a, fx, fy)
		}
		return
	}
	if a.Mode != animal.Wander {
		a.Mode, a.Path, a.PathI, a.Target = animal.Wander, nil, 0, 0
	}
	if a.PathI < len(a.Path) || now < a.NextThink {
		return
	}
	a.NextThink = now + 240 + uint64(g.rng.IntN(480))
	if p, ok := g.nearestPen(pos); ok {
		if !p.contains(pos.X, pos.Y) {
			g.animalGoTo(id, a, (p.X1+p.X2)/2, (p.Y1+p.Y2)/2)
			return
		}
		r := def.WanderRadius
		x := clampInt(pos.X+g.rng.IntN(2*r+1)-r, p.X1, p.X2)
		y := clampInt(pos.Y+g.rng.IntN(2*r+1)-r, p.Y1, p.Y2)
		g.animalGoTo(id, a, x, y)
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
	best, bestD, found := Position{}, r+1, false
	for _, aid := range g.animalIDs {
		o, ok := g.animals.Get(aid)
		if aid == id || !ok || o.Tame || !animal.DefOf(o.Species).Predator {
			continue
		}
		p, _ := g.positions.Get(aid)
		if d := weapon.Distance(pos.X, pos.Y, p.X, p.Y); d < bestD {
			best, bestD, found = p, d, true
		}
	}
	return best, found
}

func (g *Game) nearestPen(pos Position) (pen, bool) {
	best, bestD, ok := pen{}, 1<<30, false
	for _, p := range g.pens {
		cx, cy := (p.X1+p.X2)/2, (p.Y1+p.Y2)/2
		d := weapon.Distance(pos.X, pos.Y, cx, cy)
		if p.contains(pos.X, pos.Y) {
			d = -1
		}
		if d < bestD {
			best, bestD, ok = p, d, true
		}
	}
	return best, ok
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

// manhunterPack sends n enraged dustbucks in from a map edge.
func (g *Game) manhunterPack(n int) {
	ex, ey, ok := g.edgeTile()
	if !ok {
		return
	}
	now := uint64(g.clock.Now())
	for i := 0; i < n; i++ {
		for try := 0; try < 10; try++ {
			x, y := ex+g.rng.IntN(5)-2, ey+g.rng.IntN(5)-2
			if !g.grid.InBounds(x, y) {
				continue
			}
			if id, err := g.spawnAnimal(animal.Dustbuck, x, y); err == nil {
				a, _ := g.animals.Get(id)
				a.Manhunter, a.Mode = now+animal.ManhunterTicks, animal.Attack
				g.animals.Set(id, a)
				break
			}
		}
	}
	g.threat(ex, ey, "A manhunter pack of %d dustbucks is charging the colony!", n)
}

// ManhunterPack is the dev hook for a manhunter pack.
func (g *Game) ManhunterPack(n int) { g.manhunterPack(n) }

// releaseAnimal turns a tame animal loose.
func (g *Game) releaseAnimal(id entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame {
		return
	}
	p, _ := g.positions.Get(id)
	g.message(p.X, p.Y, "%s the %s was released into the wild.", a.Name, a.Species)
	a.Tame, a.Name, a.Target, a.Mode = false, "", 0, animal.Wander
	g.animals.Set(id, a)
}

// TameAnimal makes an animal tame on the spot (dev scenarios, fixtures).
func (g *Game) TameAnimal(id entity.EntityID, name string) {
	if a, ok := g.animals.Get(id); ok {
		a.Tame, a.Name, a.TameMark, a.Manhunter = true, name, false, 0
		g.animals.Set(id, a)
	}
}
