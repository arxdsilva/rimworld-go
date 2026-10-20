package game

import (
	"fmt"

	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// AnimalView is one living wild animal.
type AnimalView struct {
	ID         entity.EntityID
	Species    string
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
}

// Wildlife at colony start and the daily top-up.
const (
	startHerds      = 2
	herdSize        = 3
	startWolves     = 2
	minDustbucks    = 4
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

func (g *Game) spawnHerd(x, y, n int) {
	for i := 0; i < n; i++ {
		for try := 0; try < 12; try++ {
			hx, hy := x+g.rng.IntN(5)-2, y+g.rng.IntN(5)-2
			if _, err := g.spawnAnimal(animal.Dustbuck, hx, hy); err == nil {
				break
			}
		}
	}
}

// spawnWildlife places dustbuck herds and ridgewolves away from the
// start site.
func (g *Game) spawnWildlife() {
	g.wild = true
	for h := 0; h < startHerds; h++ {
		if x, y, ok := g.randomTileAway(wildStartRadius); ok {
			g.spawnHerd(x, y, herdSize)
		}
	}
	for w := 0; w < startWolves; w++ {
		if x, y, ok := g.randomTileAway(wolfStartRadius); ok {
			_, _ = g.spawnAnimal(animal.Ridgewolf, x, y)
		}
	}
}

// migrate tops up dustbucks at dawn so hunting stays sustainable.
func (g *Game) migrate(now uint64) {
	if !g.wild || now%uint64(clock.TicksPerDay) != uint64(dawnHour*clock.TicksPerHour) {
		return
	}
	n := 0
	for _, id := range g.animalIDs {
		if a, ok := g.animals.Get(id); ok && a.Species == animal.Dustbuck {
			n++
		}
	}
	if n >= minDustbucks {
		return
	}
	// enter at a map edge
	w, h := g.grid.Width(), g.grid.Height()
	for try := 0; try < 40; try++ {
		var x, y int
		switch g.rng.IntN(4) {
		case 0:
			x, y = g.rng.IntN(w), 1
		case 1:
			x, y = g.rng.IntN(w), h-2
		case 2:
			x, y = 1, g.rng.IntN(h)
		default:
			x, y = w-2, g.rng.IntN(h)
		}
		if g.grid.Walkable(x, y) {
			g.spawnHerd(x, y, herdSize)
			return
		}
	}
}

func (g *Game) phaseAnimals() {
	now := uint64(g.clock.Now())
	g.traces = projectile.Prune(g.traces, now)
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
		g.stepAnimal(id, &a, now)
		if _, still := g.animals.Get(id); !still {
			continue // a trap killed it
		}
		g.animals.Set(id, a)
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
		tp, threat := g.nearestThreat(id, pos, def.FleeRadius)
		if now < a.FleeUntil {
			if p, ok := g.positions.Get(a.Attacker); ok {
				tp, threat = p, true
			}
		}
		if threat {
			if a.Mode != animal.Flee || a.PathI >= len(a.Path) || (now+uint64(id))%40 == 0 {
				a.Mode = animal.Flee
				fx, fy := animal.FleeGoal(pos.X, pos.Y, tp.X, tp.Y, 8)
				g.animalGoTo(id, a, fx, fy)
			}
			return
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

// findPrey: the nearest dustbuck within HuntRadius, else a standing
// colonist within AggroRadius.
func (g *Game) findPrey(id entity.EntityID, pos Position, def animal.Def) entity.EntityID {
	best, bestD := entity.EntityID(0), def.HuntRadius+1
	for _, aid := range g.animalIDs {
		if !g.validPrey(id, aid) {
			continue
		}
		p, _ := g.positions.Get(aid)
		if d := weapon.Distance(pos.X, pos.Y, p.X, p.Y); d < bestD {
			best, bestD = aid, d
		}
	}
	if best != 0 {
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

func (g *Game) stepAnimal(id entity.EntityID, a *animal.State, now uint64) {
	if a.PathI >= len(a.Path) || (now+uint64(id))%uint64(a.Step()) != 0 {
		return
	}
	next := a.Path[a.PathI]
	if !g.grid.Walkable(next.X, next.Y) || !a.Tame && g.doorAt(next.X, next.Y) {
		a.Path, a.PathI = nil, 0
		return
	}
	g.positions.Set(id, Position{X: next.X, Y: next.Y})
	a.PathI++
	if g.trapVictim(*a) && g.hasTraps() && g.trapAt(next.X, next.Y) != 0 {
		g.animals.Set(id, *a)
		g.springTrap(id, next.X, next.Y)
		if b, ok := g.animals.Get(id); ok {
			*a = b
		}
	}
}

// killAnimal drops its meat and removes it. A predator that made the
// kill eats half of it.
func (g *Game) killAnimal(id, by entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok {
		return
	}
	pos, _ := g.positions.Get(id)
	if a.Tame {
		g.message(pos.X, pos.Y, "%s the %s has died.", a.Name, a.Species)
	}
	if k, ok := g.animals.Get(by); ok && animal.DefOf(k.Species).Predator {
		// the predator feeds on the kill; the carcass is left behind
		k.Hunger, k.Target, k.Mode, k.Attacker = 0, 0, animal.Wander, 0
		g.animals.Set(by, k)
	}
	g.removeAnimal(id)
	g.dropStack(animal.DefOf(a.Species).Corpse, 1, pos.X, pos.Y)
}

func (g *Game) removeAnimal(id entity.EntityID) {
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
			ID: id, Species: a.Species.String(), X: p.X, Y: p.Y,
			Health:     a.HP() * 100 / animal.DefOf(a.Species).MaxHP,
			Mode:       a.Mode.String(),
			Designated: a.Designated,
			Hostile:    g.hostile(a) || a.Enraged(uint64(g.clock.Now())),
			StepTicks:  a.Step(),
			Tame:       a.Tame,
			Name:       a.Name,
			TameMark:   a.TameMark,
			Manhunter:  a.Enraged(uint64(g.clock.Now())),
		})
	}
	return out
}
