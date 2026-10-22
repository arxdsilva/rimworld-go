package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// AnimalView is one living wild animal.
type AnimalView struct {
	ID entity.EntityID
	// Species is the species key ("arctic_fox": the art key); Label its
	// display name ("arctic fox"); Attack its natural weapon and attack
	// frame set ("bite", "claw", "kick", "gore", "headbutt", "peck");
	// Female picks a dimorphic species' female art; Size its body
	// length in tiles (SIM-019).
	Species    string
	Label      string
	Attack     string
	Female     bool
	Size       float64
	X, Y       int
	Health     int // percent
	Mode       string
	Designated bool
	Hostile    bool // attacking a colonist
	StepTicks  int  // duration of the current step in ticks (for lerp)
	// Tame animals belong to the colony and have a Name; TameMark: marked
	// for taming; Manhunter: enraged, hunting colonists.
	Tame      bool
	Name      string
	TameMark  bool
	Manhunter bool
	// Motion, as on PawnView.
	StepTick     uint64
	NextX, NextY int
	HasNext      bool
	BlockedBy    entity.EntityID
	// Action, as on PawnView: "bite", "kick", "graze", "flee" or "".
	Action             string
	AttackCycle        float64
	HitTick            uint64
	HitFromX, HitFromY int
	HitDamage          int
	// Log (LOG-001) is what happened to the animal, newest first.
	Log []LogLineView
}

// Wildlife at colony start (wildlife.go has the draw and the daily top-up).
const (
	startHerdsMin   = 3
	startHerdsMax   = 4
	wildStartRadius = 12
	wolfStartRadius = 16
	dawnHour        = 6
)

// SpawnAnimal places an animal of the named species (fixtures, tools).
func (g *Game) SpawnAnimal(species string, x, y int) (entity.EntityID, error) {
	s, ok := animal.ParseSpecies(species)
	if !ok {
		return 0, fmt.Errorf("spawn animal: unknown species %q", species)
	}
	return g.spawnAnimal(s, x, y)
}

func (g *Game) spawnAnimal(s animal.Species, x, y int) (entity.EntityID, error) {
	if !g.grid.Walkable(x, y) {
		return 0, fmt.Errorf("spawn animal at (%d, %d): tile not walkable", x, y)
	}
	id := g.entities.Next()
	g.animalIDs = append(g.animalIDs, id)
	g.positions.Set(id, Position{X: x, Y: y})
	g.animals.Set(id, animal.State{Species: s, NextThink: uint64(g.clock.Now()) + uint64(g.rng.IntN(300))})
	return id, nil
}

// randomTileAway finds a walkable tile at least minR (Chebyshev) from
// the map centre; ok is false after a bounded number of tries.
func (g *Game) randomTileAway(minR int) (int, int, bool) {
	cx, cy := g.grid.Width()/2, g.grid.Height()/2
	for try := 0; try < 400; try++ {
		x, y := g.rng.IntN(g.grid.Width()), g.rng.IntN(g.grid.Height())
		if weapon.Distance(x, y, cx, cy) >= minR && g.grid.Walkable(x, y) {
			return x, y, true
		}
	}
	return 0, 0, false
}

func (g *Game) phaseAnimals() {
	now := uint64(g.clock.Now())
	g.traces = projectile.Prune(g.traces, now)
	g.pruneBursts(now)
	g.migrate(now)
	for _, id := range append([]entity.EntityID(nil), g.animalIDs...) {
		a, ok := g.animals.Get(id)
		if !ok {
			continue
		}
		if a.Cooldown > 0 {
			a.Cooldown--
		}
		if animal.DefOf(a.Species).Predator {
			a.Hunger++
		}
		g.thinkAnimal(id, &a, now)
		if _, still := g.animals.Get(id); !still {
			continue
		}
		g.animals.Set(id, a) // it steps in phaseMovement
	}
}

func (g *Game) thinkAnimal(id entity.EntityID, a *animal.State, now uint64) {
	def := animal.DefOf(a.Species)
	pos, _ := g.positions.Get(id)
	if g.thinkManhunter(id, a, now) {
		return
	}
	if g.beingTamed(id) { // holds still for the handler
		a.Path, a.PathI, a.Mode = nil, 0, animal.Wander
		return
	}
	if a.Tame {
		g.thinkTame(id, a, now)
		return
	}
	switch {
	case def.Skittish:
		r := def.FleeRadius
		if a.Mode == animal.Flee {
			r += animal.FleeRelease // run well clear before grazing again
		}
		var tp Position
		threat := false
		if r > 0 { // tame-natured animals only run once hurt
			tp, threat = g.nearestThreat(id, pos, r)
		}
		if now < a.FleeUntil {
			if p, ok := g.positions.Get(a.Attacker); ok {
				tp, threat = p, true
			}
		}
		if threat {
			lost := a.PathI >= len(a.Path) && now >= a.RetryAt
			if a.Mode != animal.Flee || lost || (now+uint64(id))%40 == 0 {
				a.Mode = animal.Flee
				fx, fy := animal.FleeGoal(pos.X, pos.Y, tp.X, tp.Y, 8)
				g.animalGoTo(id, a, fx, fy)
			}
			return
		}
		if a.Mode == animal.Flee && a.PathI < len(a.Path) {
			return // finish the run
		}
	case def.Predator:
		if a.Target != 0 && !g.validPrey(id, a.Target) {
			a.Target = 0
		}
		if a.Target == 0 && a.Attacker != 0 && g.validPrey(id, a.Attacker) {
			a.Target = a.Attacker
		}
		if a.Target == 0 && a.Hungry() {
			a.Target = g.findPrey(id, pos, def)
		}
		if a.Target != 0 {
			g.chase(id, a, now)
			return
		}
		a.Attacker = 0
	}
	if a.Mode != animal.Wander {
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	}
	if a.PathI >= len(a.Path) && now >= a.NextThink {
		r := def.WanderRadius
		g.animalGoTo(id, a, pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r)
		a.NextThink = now + 240 + uint64(g.rng.IntN(480))
	}
}

// nearestThreat: the closest standing colonist or predator within r.
func (g *Game) nearestThreat(id entity.EntityID, pos Position, r int) (Position, bool) {
	best, bestD, found := Position{}, r+1, false
	consider := func(p Position) {
		if d := weapon.Distance(pos.X, pos.Y, p.X, p.Y); d < bestD {
			best, bestD, found = p, d, true
		}
	}
	for _, pid := range g.pawnIDs {
		if g.standingPawn(pid) {
			p, _ := g.positions.Get(pid)
			consider(p)
		}
	}
	for _, aid := range g.animalIDs {
		if aid == id {
			continue
		}
		if o, ok := g.animals.Get(aid); ok && animal.DefOf(o.Species).Predator {
			p, _ := g.positions.Get(aid)
			consider(p)
		}
	}
	return best, found
}

// validPrey: a living animal of another species, or a standing colonist.
func (g *Game) validPrey(self, t entity.EntityID) bool {
	if t == self {
		return false
	}
	if o, ok := g.animals.Get(t); ok {
		me, _ := g.animals.Get(self)
		return !o.Dead() && (o.Species != me.Species || o.Tame != me.Tame)
	}
	return g.standingPawn(t)
}

// findPrey: the nearest animal the predator can take within HuntRadius
// (huntable), else a standing colonist within AggroRadius. A pack
// predator's mates nearby join the hunt.
func (g *Game) findPrey(id entity.EntityID, pos Position, def animal.Def) entity.EntityID {
	best, bestD := entity.EntityID(0), def.HuntRadius+1
	for _, aid := range g.animalIDs {
		if !g.huntable(id, aid) {
			continue
		}
		p, _ := g.positions.Get(aid)
		if d := weapon.Distance(pos.X, pos.Y, p.X, p.Y); d < bestD {
			best, bestD = aid, d
		}
	}
	if best != 0 {
		if def.Pack {
			g.callPack(id, pos, best)
		}
		return best
	}
	bestD = def.AggroRadius + 1
	for _, pid := range g.pawnIDs {
		if !g.standingPawn(pid) {
			continue
		}
		p, _ := g.positions.Get(pid)
		if d := weapon.Distance(pos.X, pos.Y, p.X, p.Y); d < bestD {
			best, bestD = pid, d
		}
	}
	return best
}

// huntable: prey a predator can take: a living non-predator no bigger
// than itself (XS or S only for a small-prey hunter), never a hive
// insect or a legendary.
func (g *Game) huntable(self, t entity.EntityID) bool {
	if !g.validPrey(self, t) {
		return false
	}
	o, ok := g.animals.Get(t)
	if !ok {
		return false
	}
	me, _ := g.animals.Get(self)
	od, md := animal.DefOf(o.Species), animal.DefOf(me.Species)
	if od.Predator || od.Hive || od.Legendary {
		return false
	}
	size := animal.RosterOf(o.Species).Size
	if md.SmallPrey {
		return size <= animal.SizeS
	}
	return size <= animal.RosterOf(me.Species).Size
}

// callPack sets idle wild pack mates of id's species within packRadius on
// the same prey.
func (g *Game) callPack(id entity.EntityID, pos Position, prey entity.EntityID) {
	me, _ := g.animals.Get(id)
	for _, aid := range g.animalIDs {
		if aid == id {
			continue
		}
		o, ok := g.animals.Get(aid)
		if !ok || o.Species != me.Species || o.Tame != me.Tame || o.Target != 0 || o.Manhunter != 0 {
			continue
		}
		if p, _ := g.positions.Get(aid); chebyshev(pos, p) <= packRadius {
			o.Target = prey
			g.animals.Set(aid, o)
		}
	}
}

// animalGoTo paths towards (gx, gy), clamped to the map and nudged to a
// nearby walkable tile.
func (g *Game) animalGoTo(id entity.EntityID, a *animal.State, gx, gy int) {
	w, h := g.grid.Width(), g.grid.Height()
	gx, gy = clampInt(gx, 0, w-1), clampInt(gy, 0, h-1)
	if !g.grid.Walkable(gx, gy) {
		found := false
		for r := 1; r <= 3 && !found; r++ {
			for dy := -r; dy <= r && !found; dy++ {
				for dx := -r; dx <= r && !found; dx++ {
					if g.grid.Walkable(gx+dx, gy+dy) {
						gx, gy, found = gx+dx, gy+dy, true
					}
				}
			}
		}
		if !found {
			return
		}
	}
	pos, _ := g.positions.Get(id)
	if pos.X == gx && pos.Y == gy {
		a.Path, a.PathI = nil, 0
		return
	}
	var q pathing.Query = g.hostileMap()
	if a.Tame {
		q = g.grid // the colony's own animals use its doors
	}
	path, ok := pathing.Find(q, pos.X, pos.Y, gx, gy)
	if !ok {
		// no way there (a shut door, say): don't search again for a while
		a.Path, a.PathI = nil, 0
		a.RetryAt = uint64(g.clock.Now()) + pathRetry
		return
	}
	a.Path, a.PathI, a.GoalX, a.GoalY = path, 0, gx, gy
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// moveAnimal takes an animal's next path step if it is due and the way
// is clear (the animal half of phaseMovement; m, its record, says it
// is due).
func (g *Game) moveAnimal(id entity.EntityID, m motionRec, now uint64) {
	a, ok := g.animals.Get(id)
	if !ok {
		return // removed earlier this tick
	}
	if a.PathI >= len(a.Path) {
		if m.BlockedTicks != 0 || m.BlockedBy != 0 {
			m.BlockedTicks, m.BlockedBy = 0, 0
			g.motion.Set(id, m)
		}
		return
	}
	next := a.Path[a.PathI]
	if !g.grid.Walkable(next.X, next.Y) || !a.Tame && g.doorAt(next.X, next.Y) {
		a.Path, a.PathI = nil, 0
		g.animals.Set(id, a)
		return
	}
	from, _ := g.positions.Get(id)
	to := Position{X: next.X, Y: next.Y}
	v, alt := g.decide(id, from, to)
	switch v {
	case stepGo:
		a.PathI++
		g.stepAnimal(id, &a, &m, from, to, now)
		return
	case stepSidestep:
		g.positions.Set(id, alt) // replan from the new cell
		g.animalGoTo(id, &a, a.GoalX, a.GoalY)
		g.stepAnimal(id, &a, &m, from, alt, now)
		return
	case stepArriveNear:
		a.Path, a.PathI = nil, 0
		if alt != from {
			g.stepAnimal(id, &a, &m, from, alt, now)
			return
		}
		a.RetryAt = now + uint64(a.Step()) // stood still: pause a step before replanning
		g.animals.Set(id, a)
		return
	}
	// stepWait or stepRepath: stay put and try again next tick
	m.BlockedTicks++
	m.BlockedBy, _ = g.occupantAt(to.X, to.Y)
	if v == stepRepath {
		if path, ok := g.pathAround(id, a.GoalX, a.GoalY); ok {
			a.Path, a.PathI = path, 0
		}
	}
	if m.BlockedTicks > blockedFailTicks {
		a.Path, a.PathI = nil, 0 // give up; it thinks of something else
		m.BlockedTicks, m.BlockedBy = 0, 0
	}
	g.animals.Set(id, a)
	g.motion.Set(id, m)
}

// stepAnimal moves an animal from -> to at tick now and springs a trap
// there if it is a trap victim.
func (g *Game) stepAnimal(id entity.EntityID, a *animal.State, m *motionRec, from, to Position, now uint64) {
	g.positions.Set(id, to)
	g.stainStep(id, from) // stains.go: a bleeding animal drips
	m.stepped(now, g.weatherSlow(to.X, to.Y, a.StepCost(isDiagonal(from, to))))
	g.motion.Set(id, *m)
	g.animals.Set(id, *a)
	if g.trapVictim(*a) && g.hasTraps() && g.trapAt(to.X, to.Y) != 0 {
		g.springTrap(id, to.X, to.Y)
	}
}

// killAnimal drops its corpse and removes it (an explosive one bursts). A predator that made the
// kill eats half of it.
func (g *Game) killAnimal(id, by entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok {
		return
	}
	pos, _ := g.positions.Get(id)
	if a.Tame {
		g.message(pos.X, pos.Y, "%s the %s has died.", a.Name, a.Species.Label())
	}
	if k, ok := g.animals.Get(by); ok && animal.DefOf(k.Species).Predator {
		// the predator feeds on the kill; the carcass is left behind
		k.Hunger, k.Target, k.Mode, k.Attacker = 0, 0, animal.Wander, 0
		g.animals.Set(by, k)
	}
	if by != 0 && by != id {
		g.logEvent(by, logEntry{Kind: logKilled, Other: id, Name: animalLabel(a)})
	}
	g.removeAnimal(id)
	g.stainAnimalDeath(a.Species, pos) // stains.go
	g.dropCorpse(a.Species, pos.X, pos.Y)
	if animal.DefOf(a.Species).Explosive {
		g.burst(a.Species, pos)
	}
}

func (g *Game) removeAnimal(id entity.EntityID) {
	delete(g.logs, id)
	g.animals.Delete(id)
	g.positions.Delete(id)
	kept := g.animalIDs[:0]
	for _, aid := range g.animalIDs {
		if aid != id {
			kept = append(kept, aid)
		}
	}
	g.animalIDs = kept
	// predators chasing it pick a new target next tick
	for _, aid := range g.animalIDs {
		if o, ok := g.animals.Get(aid); ok && (o.Target == id || o.Attacker == id) {
			if o.Target == id {
				o.Target = 0
			}
			if o.Attacker == id {
				o.Attacker = 0
			}
			g.animals.Set(aid, o)
		}
	}
}

func (g *Game) markHunt(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok {
			continue
		}
		p, _ := g.positions.Get(id)
		if p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY {
			a.Designated = on
			g.animals.Set(id, a)
		}
	}
}

func (g *Game) animalAt(x, y int) entity.EntityID {
	for _, id := range g.animalIDs {
		if p, ok := g.positions.Get(id); ok && p.X == x && p.Y == y {
			return id
		}
	}
	return 0
}

func (g *Game) animalViews() []AnimalView {
	out := make([]AnimalView, 0, len(g.animalIDs))
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok {
			continue
		}
		p, _ := g.positions.Get(id)
		out = append(out, AnimalView{
			ID: id, Species: a.Species.String(), Label: a.Species.Label(), X: p.X, Y: p.Y,
			Attack: animal.RosterOf(a.Species).Attack.String(), Female: animalFemale(id, a.Species),
			Size:       animal.BodyLength(a.Species),
			Health:     a.HP() * 100 / animal.DefOf(a.Species).MaxHP,
			Mode:       a.Mode.String(),
			Designated: a.Designated,
			Hostile:    g.hostile(a) || a.Enraged(uint64(g.clock.Now())),
			Tame:       a.Tame,
			Name:       a.Name,
			TameMark:   a.TameMark,
			Manhunter:  a.Enraged(uint64(g.clock.Now())),
		})
		av := &out[len(out)-1]
		m := g.motionOf(id)
		av.StepTick, av.StepTicks = m.StepTick, m.StepTicks
		if a.PathI < len(a.Path) {
			av.NextX, av.NextY, av.HasNext = a.Path[a.PathI].X, a.Path[a.PathI].Y, true
			av.BlockedBy = m.BlockedBy
		}
		g.fillAnimalAction(av, id, a)
		av.Log = g.logLines(id) // eventlog.go
	}
	return out
}

// animalFemale reports a dimorphic animal's sex, fixed by its id
// (presentation: only the art differs).
func animalFemale(id entity.EntityID, s animal.Species) bool {
	return animal.RosterOf(s).Dimorphic && (uint64(id)*0x9E3779B97F4A7C15)>>63 == 1
}
