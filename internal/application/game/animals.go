package game

import (
	"fmt"

	"rimworld-go/internal/application/game/animalview"
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
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
	startHerdsMin   = 4
	startHerdsMax   = 6
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

// randomTileAway finds a walkable tile at least minR (Chebyshev) from
// the landing spot (the map centre for legacy colonies); ok is false
// after a bounded number of tries.
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

// phaseAnimals runs every animal: its needs every tick, its reflexes
// (fleeing, fighting, eating, walking) every tick, and a full decision
// (animal_needs.go) every animal.DecideEvery ticks or when something
// happens to it. The creature index is the tick's one build (animal_index.go).
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
		a, ok := g.animals.Get(id)
		if !ok {
			continue
		}
		if a.Dormant {
			continue // vault.go: a sealed vault's sleeper neither thinks nor needs
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
		g.thinkAnimal(id, &a, now)
		if !g.animals.Has(id) {
			continue
		}
		rage.KeepChaseWarned(&a, now) // rage/chase.go
		if a.Tame || a.FeedClaim != 0 || a.Mode == animal.Feed && a.Feed.Kind == animal.FeedCarcass {
			g.syncFeedClaim(id, &a) // animal_needs.go: an eater's reservation
		}
		g.animals.Set(id, a) // it steps in phaseMovement
	}
}

// thinkAnimal is one animal's tick: a downed animal only lies there;
// otherwise rage, being tamed, fleeing and fighting come first; then a
// leaving animal walks off; then needs pick between wandering, feeding and
// resting (decide, paced).
func (g *Game) thinkAnimal(id entity.EntityID, a *animal.State, now uint64) {
	a.Normalise(uint64(id))        // an older save's (or a fixture's) hunger counter
	a.Errand = false               // set again below by the errands that are no mode (PLAY-022)
	if g.downedThink(id, a, now) { // animal_health.go: a downed animal lies still
		return
	}
	if animal.IsMech(a.Species) {
		g.raid.Mechs.Think(id, a, now) // raid/mech.go (DEF-4): a machine, never the animal brain
		return
	}
	def := animal.DefOf(a.Species)
	pos, _ := g.positions.Get(id)
	if rageOf(g).ThinkManhunter(id, a, now) {
		return
	}
	if g.ropedStep(id, a, pos, now) { // rope.go: roped, or led to its pen
		a.Errand = true // a led animal walks
		return
	}
	if g.beingTamed(id) { // holds still for the handler
		a.Path, a.PathI, a.Mode = nil, 0, animal.Wander
		a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
		return
	}
	if rageOf(g).FleeCheck(id, a, rage.Pos(pos), def, now) {
		return
	}
	rageOf(g).ChargeCheck(id, a, rage.Pos(pos), now) // a wild predator may rush someone close
	if a.Tame && g.masterDuty(id, a, now) {          // train_animals.go: guard and release
		return
	}
	if a.Tame && def.Predator && a.HuntSince != 0 {
		rageOf(g).GuardBreaksHunt(id, a, rage.Pos(pos), now) // rage/guard.go: a threat comes before a meal
	}
	if a.Tame && def.Predator && a.HuntSince == 0 {
		if rageOf(g).GuardCheck(id, a, rage.Pos(pos), now) {
			return
		}
	} else if rageOf(g).FightCheck(id, a, rage.Pos(pos), def, now) { // a tame predator's hunt runs as a wild one's (PLAY-017)
		return
	}
	if a.Leaving {
		g.walkOff(id, a, pos, now)
		return
	}
	if now >= a.NextDecide {
		g.decideAnimal(id, a, pos, now)
		if a.Mode == animal.Attack && a.Target != 0 { // T3: off hunting
			rageOf(g).Chase(id, a, now)
			return
		}
	}
	switch a.Mode {
	case animal.Feed:
		g.feedStep(id, a, pos, now)
		return
	case animal.Rest:
		return // lies down (or walks to its herd's bed)
	case animal.Confused:
		if g.confusedStep(id, a, pos, def, now) { // bonds.go: its colonist died
			return
		}
	case animal.Wander:
	default:
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	}
	if a.Tame && g.followMaster(id, a, pos, now) { // train_animals.go: obedient, master drafted
		a.Errand = true // keeping up with its master, it walks
		return
	}
	if g.lifeMove(id, a, pos, now) { // animal_life.go: mating, following its mother
		a.Errand = a.Mate != 0 // the male walks to his mate; a young one trailing its mother ambles
		return
	}
	if a.Tame {
		g.wanderTame(id, a, pos, def, now)
		return
	}
	if a.PathI >= len(a.Path) && now >= a.NextThink {
		r := def.WanderRadius
		g.animalGoTo(id, a, pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r)
		a.NextThink = now + 240 + uint64(g.rng.IntN(480))
	}
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
	if !g.grid.Walkable(gx, gy) || g.fenceBars(gates, gx, gy) {
		found := false
		for r := 1; r <= 3 && !found; r++ {
			for dy := -r; dy <= r && !found; dy++ {
				for dx := -r; dx <= r && !found; dx++ {
					if g.grid.Walkable(gx+dx, gy+dy) && !g.fenceBars(gates, gx+dx, gy+dy) {
						gx, gy, found = gx+dx, gy+dy, true
					}
				}
			}
		}
		if !found {
			return
		}
	}
	if g.animalFireBlocked(a, gx, gy) { // fire_danger.go: no walk up to a fire
		a.Path, a.PathI = nil, 0
		a.RetryAt = uint64(g.clock.Now()) + pathRetry
		return
	}
	pos, _ := g.positions.Get(id)
	if pos.X == gx && pos.Y == gy {
		a.Path, a.PathI = nil, 0
		return
	}
	// the colony's own animals use its doors and gates; no animal
	// crosses a fence (fences.go)
	q := g.softAnimalQuery(id, g.animalPathQuery(gates, false)) // area_soft.go: areas and fire cost extra
	path, ok := g.searcher().Find(q, pos.X, pos.Y, gx, gy)
	if !ok {
		// no way there (a shut door, say): don't search again for a while
		a.Path, a.PathI = nil, 0
		a.RetryAt = uint64(g.clock.Now()) + pathRetry
		return
	}
	a.Path, a.PathI, a.GoalX, a.GoalY = path, 0, gx, gy
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
		a := *ar
		a.Path, a.PathI = nil, 0
		g.animals.Set(id, a)
		return
	}
	if ar.PathI >= len(ar.Path) {
		if m.BlockedTicks != 0 || m.BlockedBy != 0 {
			m.BlockedTicks, m.BlockedBy = 0, 0
			g.motion.Set(id, m)
		}
		return
	}
	a := *ar
	next := a.Path[a.PathI]
	gates := g.gatesOpen(id, &a) // rope.go: livestock pass gates only when led
	if !g.grid.Walkable(next.X, next.Y) || !gates && g.doorAt(next.X, next.Y) || g.fenceBars(gates, next.X, next.Y) {
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
	g.animals.Set(id, a)
	g.motion.Set(id, m)
}

// stepAnimal moves an animal from -> to at tick now and springs a trap
// there if it is a trap victim.
func (g *Game) stepAnimal(id entity.EntityID, a *animal.State, m *motionRec, from, to Position, now uint64) {
	g.positions.Set(id, to)
	g.stainStep(id, from) // stains.go: a bleeding animal drips
	// the gait is chosen as the step starts and stays with it (PLAY-022:
	// a mode change mid-step never changes this step's ticks or label)
	a.StepGait = a.GaitAt(now)
	m.stepped(now, g.weatherSlow(to.X, to.Y, a.StepCostAt(a.StepGait, isDiagonal(from, to))))
	g.motion.Set(id, *m)
	g.animals.Set(id, *a)
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
	corpse := g.dropCorpseAs(a.Species, pos.X, pos.Y, !colony, out, animal.CorpseYieldOf(a, uint64(g.clock.Now()), g.herdOf().Careful(id)))
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
	if a := g.animals.Ref(id); a != nil && a.FeedClaim != 0 {
		g.reserve.ReleaseEntity(id, a.FeedClaim) // an eater's stack is free again
	}
	g.reserve.ReleaseTarget(id) // RES-1: claims on it (a tamer, a hunter) end
	delete(g.logs, id)
	g.care.Doctor.Forget(id) // vet_animals.go
	g.raid.Mechs.Forget(id)  // raid/mech.go (DEF-4)
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
		sv := animalview.Of(a.Species)
		out = append(out, AnimalView{
			ID: id, Species: a.Species.String(), Label: sv.Label, X: p.X, Y: p.Y,
			Attack: sv.Attack, Female: animalFemale(id, a),
			Size:       sv.Size,
			Health:     a.HP() * 100 / sv.MaxHP,
			Mode:       a.Mode.String(),
			Designated: a.Designated,
			Hostile:    g.hostile(a) || a.Enraged(uint64(g.clock.Now())),
			TargetID:   rageOf(g).ChaseTarget(a),
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
		av.Gait = viewGait(a, m, now, av.HasNext) // animals_gait.go
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
	}
	return out
}

// actionState is a as the action view reads it: feeding and leaving
// animals look like wandering ones (a grazer standing still grazes).
func actionState(a animal.State) animal.State {
	switch a.Mode {
	case animal.Feed, animal.Leave, animal.Rest:
		a.Mode = animal.Wander
	}
	return a
}

// animalFemale reports an animal's sex (simulated since SIM-024; an older
// save's animals keep the sex their id gave the art).
func animalFemale(id entity.EntityID, a animal.State) bool {
	if !a.HasSex {
		return animal.ArtFemale(uint64(id))
	}
	return a.Female
}

// viewGait is AnimalView.Gait (PLAY-022 C4): the gait of the step the
// animal is taking. It is the gait the step started with (stepAnimal
// writes it to State.StepGait), not what the animal's mode says now. It
// reads "" while the animal is not moving: before its first step, lying
// down, or once its last step has ended with no cell left on its path.
func viewGait(a animal.State, m motionRec, now uint64, hasNext bool) string {
	if a.StepGait == animal.GaitNone || lyingDown(a) {
		return ""
	}
	if !hasNext && now >= m.StepTick+uint64(m.StepTicks) {
		return ""
	}
	switch a.StepGait {
	case animal.GaitAmble:
		return GaitAmble
	case animal.GaitWalk:
		return GaitWalk
	case animal.GaitRun:
		return GaitRun
	}
	return ""
}
