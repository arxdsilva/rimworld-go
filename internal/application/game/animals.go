// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"

	"rimworld-go/internal/application/game/animalview"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/region"
	"rimworld-go/pkg/mathx"
	"rimworld-go/pkg/msgfmt"
)

// Animal gaits (PLAY-022, AnimalView.Gait): wandering, grazing and penned
// milling amble; errands walk; fleeing, hunting and charging run.
const (
	GaitAmble = "amble"
	GaitWalk  = "walk"
	GaitRun   = "run"
)

// AnimalView is one living wild animal; AnimalTrainingView one training
// skill of a tame one (game/animalview).
type (
	AnimalView         = animalview.View
	AnimalTrainingView = animalview.Training
)

// Wildlife at colony start (wildlife.go has the draw; population.go the
// carrying capacity that tops it up).
const (
	startHerdsMin   = animal.StartHerdsMin
	startHerdsMax   = animal.StartHerdsMax
	wildStartRadius = 12
	wolfStartRadius = 16
	dawnHour        = 6
)

// SpawnAnimal places an animal of the named species (fixtures, tools).
func (g *Game) SpawnAnimal(species string, x, y int) (entity.EntityID, error) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	s, ok := animal.ParseSpecies(species)
	if !ok {
		return 0, fmt.Errorf("spawn animal: unknown species %q", species)
	}
	return g.spawnAnimal(s, x, y)
}

func (g *Game) spawnAnimal(s animal.Species, x, y int) (entity.EntityID, error) {
	if !g.dryWalkable(x, y) {
		return 0, fmt.Errorf("spawn animal at (%d, %d): tile not walkable", x, y)
	}
	id := g.entities.Next()
	g.animalIDs = append(g.animalIDs, id)
	g.positions.Set(id, Position{X: x, Y: y})
	// fed, rested and adult, with the sex its id gives (spawnHerd draws
	// ages, needs and sexes for wild herds)
	g.animals.Set(id, animal.State{Species: s, NextThink: uint64(g.clock.Now()) + uint64(g.rng.IntN(300)),
		Food: animal.Full, Rest: animal.Full, HasSex: true, Female: animal.ArtFemale(uint64(id))})
	g.invalidateCreatures() // animal_index.go: a mid-tick arrival is found at once
	return id, nil
}

// randomTileAway samples walkable tiles at least minR from the landing.
func (g *Game) randomTileAway(minR int) (int, int, bool) {
	cx, cy := g.landingSpot() // setup.go
	for try := 0; try < 400; try++ {
		x, y := g.rng.IntN(g.grid.Width()), g.rng.IntN(g.grid.Height())
		if weapon.Distance(x, y, cx, cy) >= minR && g.dryWalkable(x, y) {
			return x, y, true
		}
	}
	return 0, 0, false
}

// phaseAnimals runs each animal's needs, reflexes and periodic decisions.
func (g *Game) phaseAnimals() {
	now := uint64(g.clock.Now())
	g.traces = projectile.Prune(g.traces, now)
	g.pruneBursts(now)
	g.pruneCropRaids(now)
	g.migrate(now)
	g.troughDeliveries(now)                         // trough_feed.go: loads haulers brought this tick
	g.lifePhase(now)                                // animal_life.go: eggs incubate
	g.penTick(now)                                  // pens.go: unpenned days, open pens, auto-cut
	g.raid.Ships.Tick(now)                          // raid/shippart.go (DEF-4): guards wake, the blight spreads
	g.wl.ids = append(g.wl.ids[:0], g.animalIDs...) // the list changes as animals die or leave
	for _, id := range g.wl.ids {
		live := g.animals.Ref(id)
		if live == nil {
			continue
		}
		a := live.State()
		if a.Dormant {
			continue // vault.go: a sealed vault's sleeper neither thinks nor needs
		}
		if a.Tame && (now+uint64(id))%animal.DecideEvery == 0 {
			decayAnimalTraining(g, id, &a, now)
		}
		if a.Cooldown > 0 {
			a.Cooldown--
		}
		if !g.tickNeeds(id, &a, now) {
			continue // starved to death
		}
		if !g.tickLife(id, &a, now) { // animal_life.go
			continue // died of old age
		}
		thinkAnimalReady(g, id, &a, now)
		if !g.animals.Has(id) {
			continue
		}
		rage.KeepChaseWarned(&a, now) // rage/chase.go
		if a.Tame || a.FeedClaim != 0 || a.Mode == animal.Feed && a.Feed.Kind == animal.FeedCarcass {
			g.syncFeedClaim(id, &a) // animal_needs.go: an eater's reservation
		}
		live.AssignState(a) // it steps in phaseMovement
	}
}

// thinkAnimal normalizes direct callers, then runs normal animal reflexes.
func (g *Game) thinkAnimal(id entity.EntityID, a *animal.State, now uint64) {
	a.Normalise(uint64(id)) // fixtures and direct invocations also normalize
	thinkAnimalReady(g, id, a, now)
}

func thinkAnimalReady(g *Game, id entity.EntityID, a *animal.State, now uint64) {
	m := trainingAt(g, id)
	before := m
	if !thinkAnimalCore(g, id, a, &m, now) && m.Task.Source != 0 {
		herd.CancelHaul(animalHaulHost{haulHost{g}}, g.reserve, id, a, &m)
	}
	if m != before && g.animals.Has(id) {
		saveTraining(g, id, m)
	}
}

type animalHaulHost struct{ haulHost }

func (h animalHaulHost) Allowed(id entity.EntityID, x, y int) bool { return h.g.areaAllows(id, x, y) }
func (h animalHaulHost) Take(id, sid entity.EntityID, n int) (entity.EntityID, bool) {
	return (herdHost{h.g}).Take(id, sid, n)
}
func (h animalHaulHost) GoTo(id entity.EntityID, a animal.State, x, y int) animal.State {
	h.g.animalGoTo(id, &a, x, y)
	return a
}
func (h animalHaulHost) Deposit(id entity.EntityID, st item.Stack) { h.g.mergeOrSet(id, st) }
func (h animalHaulHost) HaulOffers(id entity.EntityID, a animal.State) []herd.HaulOffer {
	g := h.g
	r, layer := g.regionsNow(), region.Wild
	var areaGen uint64
	if g.worldSt != nil {
		areaGen = g.worldSt.areaGen
	}
	if g.gatesOpen(id, &a) {
		layer = region.Tame
	}
	return g.idx.animalHaul.Offers(h, g.reserve, id, herd.HaulScope{Regions: r, Layer: layer, Area: g.restrictedRef(id), Flee: a.Mode == animal.Flee, Safe: func(x, y int) bool { return !g.animalFireBlocked(&a, x, y) }},
		herd.HaulRevision{Stacks: g.stacks.Ver(), Zones: g.zones.Ver(), Fog: g.fog.ver, Grid: g.grid.Version(), Structures: g.structVer, Fences: g.regs.fenceGen, Frames: g.frames.Ver(), Claims: g.reserve.Ver(), Areas: areaGen, Home: uint64(g.homeArea.ver), Fire: g.fire.ver})
}

func thinkAnimalCore(g *Game, id entity.EntityID, a *animal.State, m *training.Maintenance, now uint64) bool {
	a.Errand = false               // restored by errands during this tick
	if g.downedThink(id, a, now) { // animal_health.go: a downed animal lies still
		return false
	}
	if a.Tame && (now+uint64(id))%animal.DecideEvery == 0 && g.rest.Animals.EndMedicalRest(id, a.Mode == animal.Rest, sleeping(*a), a.Injury > 0) && a.Mode == animal.Rest {
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	}
	if animal.IsMech(a.Species) {
		g.raid.Mechs.Think(id, a, now) // raid/mech.go (DEF-4): a machine, never the animal brain
		return false
	}
	def := animal.DefOf(a.Species)
	pos, _ := g.positions.Get(id)
	if rageOf(g).ThinkManhunter(id, a, now) {
		return false
	}
	if g.ropedStep(id, a, pos, now) { // rope.go: roped, or led to its pen
		a.Errand = true // a led animal walks
		return false
	}
	if g.beingTamed(id) { // holds still for the handler
		a.Path, a.PathI, a.Mode = nil, 0, animal.Wander
		a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
		return false
	}
	if rageOf(g).FleeCheck(id, a, rage.Pos(pos), def, now) {
		return false
	}
	if a.ChargeSince != 0 || !a.Tame && now >= a.NextCharge &&
		(now+uint64(id))%animal.DecideEvery == 0 && animal.Charger(a.Species) {
		rageOf(g).ChargeCheck(id, a, rage.Pos(pos), now)
	}
	if a.Tame && g.masterDuty(id, a, now) { // train_animals.go: guard and release
		return false
	}
	if a.Tame && def.Predator && a.HuntSince != 0 {
		rageOf(g).GuardBreaksHunt(id, a, rage.Pos(pos), now) // rage/guard.go: a threat comes before a meal
	}
	if a.Tame && def.Predator && a.HuntSince == 0 {
		if rageOf(g).GuardCheck(id, a, rage.Pos(pos), now) {
			return false
		}
	} else if a.Target == 0 && !def.Predator {
		a.HuntSince = 0
	} else if rageOf(g).FightCheck(id, a, rage.Pos(pos), def, now) { // a tame predator's hunt runs as a wild one's (PLAY-017)
		return false
	}
	if a.Leaving {
		g.walkOff(id, a, pos, now)
		return false
	}
	if now >= a.NextDecide {
		g.decideAnimal(id, a, pos, now)
		if a.Mode == animal.Attack && a.Target != 0 { // T3: off hunting
			rageOf(g).Chase(id, a, now)
			return false
		}
	}
	switch a.Mode {
	case animal.Feed:
		g.feedStep(id, a, pos, now)
		return false
	case animal.Rest:
		return false // lies down (or walks to its herd's bed)
	case animal.Confused:
		if g.confusedStep(id, a, pos, def, now) { // bonds.go: its colonist died
			return false
		}
	case animal.Wander:
	default:
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	}
	if a.Tame && g.followMaster(id, a, pos, now) { // train_animals.go: obedient, master drafted
		a.Errand = true // keeping up with its master, it walks
		return false
	}
	if g.lifeMove(id, a, pos, now) { // animal_life.go: mating, following its mother
		a.Errand = a.Mate != 0 // the male walks to his mate; a young one trailing its mother ambles
		return false
	}
	if m.HaulProgress != 0 && herd.HaulStep(animalHaulHost{haulHost{g}}, g.reserve, id, a, m, now) {
		a.Errand = true
		return true
	}
	if a.Tame {
		g.wanderTame(id, a, pos, def, now)
		return false
	}
	if a.PathI >= len(a.Path) && now >= a.NextThink {
		r := def.WanderRadius
		g.animalGoTo(id, a, pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r)
		a.NextThink = now + 240 + uint64(g.rng.IntN(480))
	}
	return false
}

// posOf is id's position (the zero cell when it has none).
func (g *Game) posOf(id entity.EntityID) Position {
	p, _ := g.positions.Get(id)
	return p
}

// animalGoTo paths towards (gx, gy), clamped to the map and nudged to a
// nearby walkable tile.
func (g *Game) animalGoTo(id entity.EntityID, a *animal.State, gx, gy int) {
	gates := g.gatesOpen(id, a) // rope.go: livestock pass gates only when led
	w, h := g.grid.Width(), g.grid.Height()
	gx, gy = mathx.Clamp(gx, 0, w-1), mathx.Clamp(gy, 0, h-1)
	var found bool
	gx, gy, found = herd.WalkTarget(gx, gy, func(x, y int) bool { return g.grid.Walkable(x, y) && !g.fenceBars(gates, x, y) })
	if !found {
		return
	}

	if g.animalFireBlocked(a, gx, gy) { // fire_danger.go: no walk up to a fire
		a.Path, a.PathI = nil, 0
		a.RetryAt = uint64(g.clock.Now()) + pathRetry
		return
	}
	pos, _ := g.positions.Get(id)
	// Soft area and fire costs do not change the movement layer.
	q := g.softAnimalQuery(id, g.animalPathQuery(gates, false))
	herd.AssignPath(a, g.searcher(), q, pos.X, pos.Y, gx, gy, uint64(g.clock.Now())+pathRetry)
}

// moveAnimal takes an animal's next path step if it is due and the way
// is clear (the animal half of phaseMovement; m, its record, says it
// is due).
func (g *Game) moveAnimal(id entity.EntityID, m motionRec, now uint64) {
	ar := g.animals.Ref(id)
	if ar == nil {
		return // removed earlier this tick
	}
	if ar.Downed && ar.PathI < len(ar.Path) { // animal_health.go: a downed animal takes no step
		ar.Path, ar.PathI = nil, 0
		return
	}
	if ar.PathI >= len(ar.Path) {
		if m.BlockedTicks != 0 || m.BlockedBy != 0 {
			m.BlockedTicks, m.BlockedBy = 0, 0
			g.motion.Set(id, m)
		}
		return
	}
	a := ar.State()
	next := a.Path[a.PathI]
	gates := g.gatesOpen(id, &a) // rope.go: livestock pass gates only when led
	if !g.grid.Walkable(next.X, next.Y) || !gates && g.doorAt(next.X, next.Y) || g.fenceBars(gates, next.X, next.Y) {
		a.Path, a.PathI = nil, 0
		ar.AssignState(a)
		return
	}
	from, _ := g.positions.Get(id)
	to := Position{X: next.X, Y: next.Y}
	v, alt := g.decide(id, from, to)
	switch v {
	case stepGo:
		a.PathI++
		g.stepAnimal(id, ar, &a, &m, from, to, now)
		return
	case stepSidestep:
		g.positions.Set(id, alt) // replan from the new cell
		g.animalGoTo(id, &a, a.GoalX, a.GoalY)
		g.stepAnimal(id, ar, &a, &m, from, alt, now)
		return
	case stepArriveNear:
		a.Path, a.PathI = nil, 0
		if alt != from {
			g.stepAnimal(id, ar, &a, &m, from, alt, now)
			return
		}
		a.RetryAt = now + uint64(a.Step()) // stood still: pause a step before replanning
		ar.AssignState(a)
		return
	}
	// stepWait or stepRepath: stay put and try again next tick
	m.BlockedTicks++
	m.BlockedBy, _ = g.occupantAt(to.X, to.Y) // a search around it is queued (paths.go runRepaths applies it)
	if v == stepRepath {
		if path, ok := g.pathAround(id, a.GoalX, a.GoalY); ok {
			a.Path, a.PathI = path, 0
		}
	}
	if m.BlockedTicks > blockedFailTicks {
		a.Path, a.PathI = nil, 0 // give up; it thinks of something else
		m.BlockedTicks, m.BlockedBy = 0, 0
	}
	ar.AssignState(a)
	g.motion.Set(id, m)
}

// stepAnimal moves an animal from -> to at tick now and springs a trap
// there if it is a trap victim.
func (g *Game) stepAnimal(id entity.EntityID, live *animal.Live, a *animal.State, m *motionRec, from, to Position, now uint64) {
	g.positions.Set(id, to)
	g.stainStep(id, from) // stains.go: a bleeding animal drips
	// the gait is chosen as the step starts and stays with it (PLAY-022:
	// a mode change mid-step never changes this step's ticks or label)
	a.StepGait = a.GaitAt(now)
	m.stepped(now, g.weatherSlow(to.X, to.Y, a.StepCostAt(a.StepGait, isDiagonal(from, to))))
	g.motion.Set(id, *m)
	live.AssignState(*a)
	if t := g.combat.Traps; t.Victim(*a) && t.At(to.X, to.Y) != 0 { // one object lookup: no building scan
		t.Spring(id, to.X, to.Y)
	}
}

// KillAnimal kills animal id where it stands (dev and debug tools).
func (g *Game) KillAnimal(id entity.EntityID) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	g.killAnimal(id, 0)
}

// killerName is who did it, for a death message: "a timber wolf", or a
// colonist's or pet's name.
func (g *Game) killerName(id entity.EntityID) string {
	if a, ok := g.animals.Get(id); ok && !(a.Tame && a.Name != "") {
		return "a " + a.Species.Label()
	}
	return g.creatureName(id)
}

// killAnimal drops its corpse and removes it (an explosive one bursts).
// A predator that made the kill eats it (T4): its feed target becomes the
// carcass, and eating it consumes the corpse.
func (g *Game) killAnimal(id, by entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok {
		return
	}
	pos, _ := g.positions.Get(id)
	if a.Tame {
		var text string
		if a.Food == 0 && a.Starving != 0 {
			text = fmt.Sprintf("%s the %s has starved to death.", a.Name, a.Species.Label())
		} else if by != 0 && by != id {
			text = fmt.Sprintf("%s the %s was killed by %s.", a.Name, a.Species.Label(), g.killerName(by))
		} else {
			text = fmt.Sprintf("%s the %s has died.", a.Name, a.Species.Label())
		}
		if c := entity.EntityID(a.Bond); c != 0 && g.isPawn(c) {
			// a bonded companion is a letter about the colonist who lost it
			pawnLetterf(g, letters.Caution, "Bonded animal died", c, "%s %s has lost a bonded companion.", text, g.personName(c))
		} else {
			g.message(pos.X, pos.Y, "%s", text)
		}
	}
	if by != 0 && by != id {
		g.logEvent(by, logEntry{Kind: evlog.Killed, Other: id, Name: animalLabel(a)})
	}
	// the colony's death (wildlife.go), else the corpse is unclaimed (PLAY-017)
	colony := a.Tame || g.colonyKill(by, pos)
	out := forbids(g).CorpseForbidden(pos.X, pos.Y, killTaken(g, by, id)) // game/allow: outside the home area
	g.onAnimalDied(id, by)                                                // bonds.go: the bonded colonist's grief, before the state goes
	g.removeAnimal(id)
	g.stainAnimalDeath(a.Species, pos) // stains.go
	corpse := g.dropCorpseAs(a.Species, pos.X, pos.Y, !colony, out, animal.CorpseYieldOf(a, uint64(g.clock.Now()), g.herdOf().Careful(id), g.clock.Calendar()))
	toxicRot(g, corpse, body.ToxicBuildup(a.Body)) // incidents.go (FOOD-6)
	g.herdOf().Killed(by, id, corpse)              // game/herd (FOOD-3): a hunter carries its kill to storage
	k, ok := g.animals.Get(by)
	if ok && !animal.IsMech(a.Species) && (animal.DefOf(k.Species).Predator || k.HuntSince != 0) { // a wreck is no meal
		if k.Tame && k.HuntSince != 0 { // a quiet line: the colony's hunter fed itself
			g.post(MessageView{Tick: uint64(g.clock.Now()), Msg: msgfmt.Of("%s hunted %s.", animalLabel(k), rage.WithArticle(a.Species.Label())),
				X: pos.X, Y: pos.Y, Focus: by})
		}
		g.feedOnKill(by, &k, corpse, pos) // animal_needs.go
		g.animals.Set(by, k)
	}
	if animal.DefOf(a.Species).Explosive {
		g.burst(a.Species, pos)
	}
}

func (g *Game) removeAnimal(id entity.EntityID) {
	packCargoLost(g, id)
	g.detachAnimal(id)
	saveTraining(g, id, training.Maintenance{})
}

// detachAnimal removes the active-map record while a live transfer retains
// its global training maintenance and ownership.
func (g *Game) detachAnimal(id entity.EntityID) {
	if m := trainingAt(g, id); m.Task.Source != 0 && g.animals.Has(id) {
		a := g.animals.Ref(id).State()
		herd.CancelHaul(animalHaulHost{haulHost{g}}, g.reserve, id, &a, &m)
		saveTraining(g, id, m)
	}
	g.care.Doctor.PatientDied(id)
	g.releaseBed(id)
	g.care.Rest.Clear(id)
	if a := g.animals.Ref(id); a != nil && a.FeedClaim != 0 {
		g.reserve.ReleaseEntity(id, a.FeedClaim) // an eater's stack is free again
	}
	g.reserve.ReleaseTarget(id) // RES-1: claims on it (a tamer, a hunter) end
	delete(g.logs, id)
	g.care.Doctor.Forget(id) // vet_animals.go
	g.raid.Mechs.Forget(id)  // raid/mech.go (DEF-4)
	rageOf(g).Forget(id)     // rage/memory.go (EAI): its bash and foe
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
	markHuntWhere(g, minX, minY, maxX, maxY, on, nil)
}

// markHuntWhere marks the animals keep takes (nil: all; AX-1 slaughter:
// tame ones) in the normalized rectangle.
func markHuntWhere(g *Game, minX, minY, maxX, maxY int, on bool, keep func(animal.State) bool) {
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok || keep != nil && !keep(a) {
			continue
		}
		p, _ := g.positions.Get(id)
		if p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY && !g.fogged(p.X, p.Y) {
			a.Designated = on
			g.animals.Set(id, a)
		}
	}
}

func (g *Game) animalAt(x, y int) entity.EntityID {
	id, _, _ := g.nearestWhere(x, y, 0, ckAnimal, func(o entity.EntityID, _ Position) bool {
		_, ok := g.animals.Get(o)
		return ok
	})
	return id
}

func (g *Game) animalViews() []AnimalView {
	out := make([]AnimalView, 0, len(g.animalIDs))
	now := uint64(g.clock.Now())
	young := g.youngByMother() // bond_views.go
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok {
			continue
		}
		p, _ := g.positions.Get(id)
		out = append(out, animalview.Live(id, a, p.X, p.Y, animalview.Female(id, a),
			g.hostile(a) || a.Enraged(now), rageOf(g).ChaseTarget(a), now))
		av := &out[len(out)-1]
		if g.worldSt != nil {
			av.Visiting(g.worldSt.visitGroups, g.ng.factions)
		}
		m := g.motionOf(id)
		av.StepTick, av.StepTicks = m.StepTick, m.StepTicks
		if a.PathI < len(a.Path) {
			av.NextX, av.NextY, av.HasNext = a.Path[a.PathI].X, a.Path[a.PathI].Y, true
			av.BlockedBy = m.BlockedBy
		}
		g.fillAnimalAction(av, id, a)
		av.Gait = animalview.Gait(a, m.StepTick, m.StepTicks, now, av.HasNext) // animals_gait.go
		av.Downed = a.Downed
		if av.Sleeping = lyingDown(a); av.Sleeping { // animal_health.go: asleep or downed
			av.Action = "" // the renderer draws the rest pose
		}
		g.fillAnimalNeeds(av, a) // animal_needs.go
		if s := g.chargeLine(a); s != "" {
			av.State = s // action_view.go: "charging Ada"
		}
		fillDormantView(av, a)                   // vault.go: a sealed vault's sleeper
		av.Log = g.logLines(id)                  // eventlog.go
		g.fillAnimalBonds(av, id, a, now, young) // bond_views.go
		av.AreaRef = g.AreaOf(id)                // areas.go
		g.fillAnimalPen(av, id, &a)              // pens.go: Penned, Pen
		g.fillAnimalHealth(av, id, a, now)       // animal_health.go (MED-09)
		// raid/mech.go (DEF-4): a machine, perhaps stunned by an EMP
		av.Mech, av.Stunned, av.Staggered = animal.IsMech(a.Species), g.raid.Mechs.Stunned(id, now), staggered(g, id)
		// EAI: "Breaking through the door", "Defending its herd", "Manhunter (grief)"
		av.Bashing, av.Defending, av.Grief = rageOf(g).Bashing(id, a, now), rageOf(g).Defending(a), a.Tame && a.Enraged(now)
	}
	return out
}
