// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// Animal life is updated within phaseAnimals, including growth and breeding.

// tickLife reports false when the animal dies of old age.
func (g *Game) tickLife(id entity.EntityID, a *animal.State, now uint64) bool {
	return animalLife(g).Tick(id, a, now, dawnHour)
}

// lifePhase is the phase-level part of life (phaseAnimals, once a tick):
// each fertile egg incubates once a climate interval, on its own tick
// (cadence); the auto-slaughter check runs once a day.
func (g *Game) lifePhase(now uint64) {
	g.incubate(now)
	if herd.CheckTick(now) {
		g.herdOf().Auto.Check() // game/herd (FOOD-3): auto-slaughter, once a day
	}
}

// grown checks adult age.
func (g *Game) grown(a animal.State, now uint64) bool {
	return animal.Adult(a.Species, animal.AgeDays(a.Born, now, a.Species, g.clock.Calendar()))
}

// portion scales meals for young animals.
func (g *Game) portion(a *animal.State, now uint64, v int) int {
	if g.grown(*a, now) {
		return v
	}
	return v * animal.YoungMeal
}

// --- old age --------------------------------------------------------------

// oldAge reports whether this animal died from aging.
func (g *Game) oldAge(id entity.EntityID, a *animal.State, now uint64) bool {
	p := animal.OldAgeChance(a.Species, animal.AgeDays(a.Born, now, a.Species, g.clock.Calendar()))
	if p <= 0 || g.rng.IntN(1000) >= p {
		return false
	}
	pos, _ := g.positions.Get(id)
	if a.Tame {
		g.message(pos.X, pos.Y, "%s the %s died of old age.", a.Name, a.Species.Label())
	}
	g.logEvent(id, logEntry{Kind: evlog.OldAge})
	g.endMating(id, a)
	g.removeAnimal(id)
	toxicRot(g, g.dropCorpseAs(a.Species, pos.X, pos.Y, !a.Tame, forbids(g).DropForbidden(pos.X, pos.Y), animal.CorpseYieldOf(*a, now, true, g.clock.Calendar())), body.ToxicBuildup(a.Body)) // wildlife.go: a wild one's is unclaimed (PLAY-017)
	if animal.DefOf(a.Species).Explosive {
		g.burst(a.Species, pos)
	}
	return true
}

// --- mating ---------------------------------------------------------------

// mateCheck rolls mating when an eligible partner is available.
func (g *Game) mateCheck(id entity.EntityID, a *animal.State, now uint64) {
	animalLife(g).MateCheck(id, a, now)
}

// lifeMove is a wandering animal's life movement (thinkAnimal): a mating
// under way, or a young one following its mother. It reports true when it
// moved (or held) the animal this tick.
func (g *Game) lifeMove(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	return animalLife(g).Move(id, a, pathing.Point(pos), now)
}

// mateStep: she holds still; he walks to her, then mates for MateTicks.
func (g *Game) mateStep(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	return animalLife(g).MateStep(id, a, pathing.Point(pos), now)
}

// endMating calls a's mating off on both sides.
func (g *Game) endMating(id entity.EntityID, a *animal.State) { animalLife(g).EndMating(id, a) }

// --- the young --------------------------------------------------------------

// giveBirth delivers the decided litter, keeping the pregnancy and any
// unspawned young until a valid birth tile becomes available.
func (g *Game) giveBirth(id entity.EntityID, a *animal.State, now uint64) {
	animalLife(g).GiveBirth(id, a, now)
}

// spawnYoung places a newborn (or hatchling) of s at (x, y).
func (g *Game) spawnYoung(s animal.Species, x, y int, tame bool, herd uint32, now uint64) (entity.EntityID, bool) {
	cid, err := g.spawnAnimal(s, x, y)
	if err != nil {
		return 0, false
	}
	c, _ := g.animals.Get(cid)
	c.Born, c.Female, c.Herd = animal.BornAt(now, 0), g.rng.IntN(2) == 0, herd
	if tame {
		c.Tame, c.Name = true, petNames[g.rng.IntN(len(petNames))]
	}
	g.animals.Set(cid, c)
	return cid, true
}

// birthSpot is a free walkable tile beside (x, y) (no creature on it), in
// a fixed order, else (x, y) itself when it is walkable.
func (g *Game) birthSpot(x, y int) (int, int, bool) {
	for _, d := range [][2]int{{0, 1}, {1, 0}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
		nx, ny := x+d[0], y+d[1]
		if !g.dryWalkable(nx, ny) {
			continue
		}
		if _, _, taken := g.nearestWhere(nx, ny, 0, ckAll, nil); !taken {
			return nx, ny, true
		}
	}
	return x, y, g.dryWalkable(x, y)
}

// --- eggs -------------------------------------------------------------------

// layOne consumes one pending egg only after it is laid. Wild plain
// layers can lay without a mate; previously fertilized wild eggs hatch.
func (g *Game) layOne(id entity.EntityID, a *animal.State) {
	fertile := animal.FertileEgg(a.Species, a.Mated)
	plain := animal.EcologyOf(a.Species).Eggs == animal.PlainEggs
	if !fertile && !plain {
		return
	}
	pos, ok := g.positions.Get(id)
	if !ok {
		return
	}
	x, y := pos.X, pos.Y
	if a.Tame {
		if bx, by, ok := eggBoxNear(g, x, y); ok { // A2: an egg box in reach first
			x, y = bx, by
		} else if m, _ := g.penAt(x, y); m != 0 { // pens.go
			x, y = g.eggBox(m, x, y)
		}
	}
	kind := item.Egg
	if fertile {
		kind = item.FertileEgg
	}
	sid, err := g.SpawnStack(kind, 1, x, y)
	if err != nil {
		return
	}
	if a.Mated > 0 {
		a.Mated--
	}
	st, _ := g.stacks.Get(sid)
	st.Animal = uint8(a.Species) // FOOD-1: an egg feeds by its species (animal.StackNutrition)
	if fertile {
		st.Tame = a.Tame
	}
	st.Unclaimed = !a.Tame                                   // a wild mother's egg waits for a haul mark (PLAY-017)
	st.Forbidden = !a.Tame && forbids(g).DropForbidden(x, y) // and starts forbidden outside Home (AL-4)
	g.mergeOrSet(sid, st)                                    // onto the eggs already there
}

// eggBox is where a hen in pen lays: the walkable pen cell nearest the
// pen's trough (the one nearest (x, y)), else (x, y).
func (g *Game) eggBox(pen entity.EntityID, x, y int) (int, int) {
	var tb Position
	bestD, found := 1<<30, false
	for _, bid := range g.bldgIDs {
		b, ok := g.isTrough(bid)
		if !ok {
			continue
		}
		if p, ok := g.troughPen(b); !ok || p != pen {
			continue
		}
		if d := weapon.Distance(x, y, b.X, b.Y); d < bestD {
			tb, bestD, found = Position{X: b.X, Y: b.Y}, d, true
		}
	}
	if !found {
		return x, y
	}
	bx, by, bestD := x, y, 1<<30
	c := g.penRegion(pen)
	for cy := tb.Y - 2; cy <= tb.Y+2 && bestD > 1; cy++ {
		for cx := tb.X - 2; cx <= tb.X+2 && bestD > 1; cx++ {
			if (cx != tb.X || cy != tb.Y) && g.inPenCells(c, cx, cy) {
				if d := weapon.Distance(tb.X, tb.Y, cx, cy); d < bestD {
					bx, by, bestD = cx, cy, d
				}
			}
		}
	}
	return bx, by
}

// eggBoxReach is how far (tiles) a tame layer goes to an egg box (A2;
// original tuning: the wiki gives no distance).
const eggBoxReach = 20

// eggBoxNear is the nearest allowed egg box with room within eggBoxReach
// of (x, y) that a tame animal there can walk to (A2, wiki: Egg box).
func eggBoxNear(g *Game, x, y int) (bx, by int, ok bool) {
	best, q := eggBoxReach+1, g.animalPathQuery(true, false)
	for _, bid := range g.bldgIDs {
		b := g.bldgs.Ref(bid)
		if b == nil || b.Kind != construction.EggBox || b.Forbidden || pathing.Dist(x, y, b.X, b.Y) >= best {
			continue
		}
		r, can := q.(pathing.Reacher)
		if _, n, c := g.storeView(bid); n < c && (!can || r.Reach(x, y, b.X, b.Y)) {
			best, bx, by = pathing.Dist(x, y, b.X, b.Y), b.X, b.Y
		}
	}
	return bx, by, best <= eggBoxReach
}

// incubate advances each fertile egg stack on the ground whose turn is
// now by a climate interval: warm eggs incubate, frozen ones die, ripe
// ones hatch.
func (g *Game) incubate(now uint64) {
	var eggs []entity.EntityID
	for id := range cadence.Each(now, climateInterval, cadence.OffEgg, g.entities.Peek()) {
		if st := g.stacks.Ref(id); st != nil && st.Kind == item.FertileEgg && st.Carrier == 0 {
			eggs = append(eggs, id)
		}
	}
	for _, id := range eggs {
		st, ok := g.stacks.Get(id)
		if !ok || st.Kind != item.FertileEgg {
			continue // merged away
		}
		if t := int(g.temperatureAt(st.X, st.Y)); t < animal.HatchMin || t > animal.HatchMax {
			st.Kind, st.Tame, st.Hatch = item.Egg, false, 0 // wiki: Eggs: ruined outside 0-50 C (FOOD-3); the species stays: its nutrition (FOOD-1)
			g.mergeOrSet(id, st)
			continue
		}
		st.Hatch += climateInterval
		if st.Hatch < animal.HatchTicks(animal.Species(st.Animal)) {
			g.putStack(id, st)
			continue
		}
		g.hatch(id, st)
	}
}

// hatch creates only young that can occupy real tiles. Unhatched eggs
// retain both their quantity and incubation progress for the next attempt.
func (g *Game) hatch(id entity.EntityID, st item.Stack) { animalLife(g).Hatch(id, st) }

// wildHerdNear is the herd of the nearest wild grown animal of s within
// MotherRadius of (x, y), else a new herd.
func (g *Game) wildHerdNear(s animal.Species, x, y int, now uint64) uint32 {
	mid, _, ok := g.nearestWhere(x, y, animal.MotherRadius, ckAnimal, func(o entity.EntityID, _ Position) bool {
		m, ok := g.animals.Get(o)
		return ok && m.Species == s && !m.Tame && m.Herd != 0 && g.grown(m, now)
	})
	if ok {
		m, _ := g.animals.Get(mid)
		return m.Herd
	}
	g.wl.LastHerd++
	return g.wl.LastHerd
}

// fillAnimalLife fills the breeding and product fields of an animal view.
func (g *Game) fillAnimalLife(av *AnimalView, a animal.State, now uint64) {
	if a.Pregnant != 0 {
		gest := animal.GestTicks(a.Species)
		av.Pregnant = true
		if gest > 0 && a.Pregnant > now {
			av.PregPct = mathx.Clamp(int(100-(a.Pregnant-now)*100/gest), 0, 100)
		} else {
			av.PregPct = 100
		}
	}
	av.EggsFertile = animal.Layer(a.Species) && a.Mated > 0
	if a.Tame {
		av.FoodPolicy = g.sk.diet.Of(av.ID, true).Name // FOOD-4
	}
	w := animal.Wildness(a.Species) // FOOD-3: revenge, wildness, handling (wiki)
	av.Revenge, av.Wildness, av.MinHandling = g.revengePct(animal.DefOf(a.Species).Revenge), w/10, animal.MinHandling(w)
	av.ToxicBuildup = int(body.ToxicBuildup(a.Body)) / 10                                                 // FOOD-6
	av.ComfortMin, av.ComfortMax, av.Hypothermia, av.Heatstroke = g.exposure.AnimalView(av.ID, a.Species) // game/bodytemp (TEMP-3)
	if animal.Produces(a.Species, a.Female, av.Adult) {
		av.Product = animal.EcologyOf(a.Species).Product.String()
		av.ProductPct = a.Product * 100 / animal.ProductFull
	}
}

// animalLife composes the existing husbandry role with map-specific facts.
// No new mutable owner, time source or random stream is introduced.
func animalLife(g *Game) herd.Life { return herd.Life{H: lifeHost{herdHost{g}}} }

type lifeHost struct{ herdHost }

func (h lifeHost) Position(id entity.EntityID) (pathing.Point, bool) {
	p, ok := h.g.positions.Get(id)
	return pathing.Point(p), ok
}
func (h lifeHost) NearestAnimal(x, y, r int, keep func(entity.EntityID, animal.State) bool) (entity.EntityID, pathing.Point, bool) {
	id, p, ok := h.g.nearestWhere(x, y, r, ckAnimal, func(id entity.EntityID, _ Position) bool { a, ok := h.g.animals.Get(id); return ok && keep(id, a) })
	return id, pathing.Point(p), ok
}
func (h lifeHost) BeingHandled(id entity.EntityID) bool { return h.g.beingTamed(id) }
func (h lifeHost) GoTo(id entity.EntityID, a animal.State, x, y int) animal.State {
	h.g.animalGoTo(id, &a, x, y)
	return a
}
func (h lifeHost) Event(id entity.EntityID, k uint8, other entity.EntityID, n int, name string) {
	h.g.logEvent(id, logEntry{Kind: k, Other: other, Amount: n, Name: name})
}
func (h lifeHost) Birthday(id entity.EntityID, a animal.State, now uint64) animal.State {
	g := h.g
	if cadence.Due(now, clock.TicksPerDay, cadence.OffMedical, id) && animal.RollAsthmaBirthday(&a, now, g.stream(streamMedical).IntN, g.clock.Calendar()) {
		g.markAnimalPatient(id)
	}
	return a
}
func (h lifeHost) OldAge(id entity.EntityID, a animal.State, now uint64) bool {
	return h.g.oldAge(id, &a, now)
}
func (h lifeHost) BirthSpot(x, y int) (int, int, bool) { return h.g.birthSpot(x, y) }
func (h lifeHost) SpawnYoung(s animal.Species, x, y int, tame bool, herdID uint32, now uint64) (entity.EntityID, bool) {
	return h.g.spawnYoung(s, x, y, tame, herdID, now)
}
func (h lifeHost) WildHerdNear(s animal.Species, x, y int, now uint64) uint32 {
	return h.g.wildHerdNear(s, x, y, now)
}
func (h lifeHost) LayOne(id entity.EntityID, a animal.State) animal.State {
	h.g.layOne(id, &a)
	return a
}
func (h lifeHost) PutEgg(id entity.EntityID, st item.Stack) { h.g.putStack(id, st) }
