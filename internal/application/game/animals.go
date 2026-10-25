package game

import (
	"fmt"

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
	// TargetID is who a hostile animal is chasing (0 = none, PLAY-017).
	TargetID  entity.EntityID
	StepTicks int // duration of the current step in ticks (for lerp)
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
	// Needs and life (SIM-024..026). State is the behaviour ("wandering",
	// "feeding", "resting", "hunting", "attacking", "fleeing", "leaving");
	// Food and Rest are percent. Stage is "young" or "adult" (or the
	// archetype's word for young); Scale the body size factor, 0.55..1 as
	// the young grow. Product is "milk", "wool", "ember oil" or "" with
	// ProductPct its fullness. Herd groups animals that move together.
	State       string
	Food, Rest  int
	Diet        string
	Hunts       bool // PLAY-017: a tame predator (hunts wild animals when hungry)
	AgeDays     int
	Adult       bool
	Stage       string
	Scale       float64
	Pregnant    bool
	PregPct     int
	EggsFertile bool
	Product     string
	ProductPct  int
	Sleeping    bool
	Leaving     bool
	Herd        uint32
	// Dormant (MAPGEN-3): a sealed vault's sleeping defender (vault.go).
	Dormant bool
	// PLAY-015: Penned livestock show Pen (the pen's name, "Roaming" or
	// "Being roped"); other tame animals have an allowed area AreaRef
	// (command.AreaUnrestricted, command.AreaHome or an area id).
	Penned  bool
	Pen     string
	AreaRef int
	// ENG-003 (M10): the master's and bonded colonist's names and ids
	// (tame animals), which training.Skill skills are learnt, and every
	// skill's row (Training, in training.Skill order). Mother is the
	// mother's name while she lives on the map; Offspring the names of
	// her young still on it.
	Master   string
	Bond     string
	Trained  []bool
	MasterID entity.EntityID
	// CanFollow (PLAY-015): trained enough to follow a drafted master;
	// Follows: it will (the player can turn it off).
	CanFollow, Follows bool
	BondID             entity.EntityID
	Training           []AnimalTrainingView
	Mother             string
	Offspring          []string
}

// AnimalTrainingView is one training skill of a tame animal: Allowed if
// its species can learn it, On if handlers train it, Steps of Needed.
type AnimalTrainingView struct {
	Skill   int // training.Skill (SetTraining's Index)
	Label   string
	Allowed bool
	On      bool
	Learned bool
	Steps   int
	Needed  int
}

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
// happens to it. The creature index is rebuilt once here first.
func (g *Game) phaseAnimals() {
	now := uint64(g.clock.Now())
	g.traces = projectile.Prune(g.traces, now)
	g.pruneBursts(now)
	g.pruneCropRaids(now)
	g.migrate(now)
	g.rebuildCreatures()                            // animal_index.go
	g.troughDeliveries(now)                         // trough_feed.go: loads haulers brought this tick
	g.lifePhase(now)                                // animal_life.go: eggs incubate
	g.penTick(now)                                  // pens.go: unpenned days, open pens, auto-cut
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
		keepChaseWarned(&a, now) // chase.go
		if a.Tame || a.FeedClaim != 0 {
			g.syncFeedClaim(id, &a) // animal_needs.go: a tame eater's reservation
		}
		g.animals.Set(id, a) // it steps in phaseMovement
	}
}

// thinkAnimal is one animal's tick: rage, being tamed, fleeing and
// fighting come first; then a leaving animal walks off; then needs pick
// between wandering, feeding and resting (decide, paced).
func (g *Game) thinkAnimal(id entity.EntityID, a *animal.State, now uint64) {
	a.Normalise(uint64(id)) // an older save's (or a fixture's) hunger counter
	def := animal.DefOf(a.Species)
	pos, _ := g.positions.Get(id)
	if g.thinkManhunter(id, a, now) {
		return
	}
	if g.ropedStep(id, a, pos, now) { // rope.go: roped, or led to its pen
		return
	}
	if g.beingTamed(id) { // holds still for the handler
		a.Path, a.PathI, a.Mode = nil, 0, animal.Wander
		a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
		return
	}
	if g.fleeCheck(id, a, pos, def, now) {
		return
	}
	g.chargeCheck(id, a, pos, now)          // a wild predator may rush someone close
	if a.Tame && g.masterDuty(id, a, now) { // train_animals.go: guard and release
		return
	}
	if a.Tame && def.Predator && a.HuntSince != 0 {
		g.guardBreaksHunt(id, a, pos, now) // handling.go: a threat comes before a meal
	}
	if a.Tame && def.Predator && a.HuntSince == 0 {
		if g.guardCheck(id, a, pos, now) {
			return
		}
	} else if g.fightCheck(id, a, pos, def, now) { // a tame predator's hunt runs as a wild one's (PLAY-017)
		return
	}
	if a.Leaving {
		g.walkOff(id, a, pos, now)
		return
	}
	if now >= a.NextDecide {
		g.decideAnimal(id, a, pos, now)
		if a.Mode == animal.Attack && a.Target != 0 { // T3: off hunting
			g.chase(id, a, now)
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
		return
	}
	if g.lifeMove(id, a, pos, now) { // animal_life.go: mating, following its mother
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

// fleeCheck runs a skittish animal away from threats (a tame one from
// wild predators only), and any animal, wild or tame, still in its
// FleeUntil window from whoever hurt it. Threats are looked for every
// animal.ThreatEvery ticks (staggered by id) and every tick while
// fleeing; the goal (fleeFrom) is picked again when the run ends or
// every 40 ticks, and kept while it is at least as far from the threat.
// It reports true while the animal is fleeing.
func (g *Game) fleeCheck(id entity.EntityID, a *animal.State, pos Position, def animal.Def, now uint64) bool {
	var tp Position
	threat := false
	scan := a.Mode == animal.Flee || (now+uint64(id))%animal.ThreatEvery == 0
	r := def.FleeRadius
	if a.Mode == animal.Flee {
		r += animal.FleeRelease // run well clear before grazing again
	}
	dist := 8
	switch {
	case a.Tame && def.Predator: // guards don't run from predators
	case a.Tame:
		dist = 6
		if scan {
			tp, threat = g.nearestPredator(id, pos, r)
		}
	case def.Skittish && r > 0 && scan: // tame-natured animals only run once hurt
		tp, threat = g.nearestThreat(id, pos, r)
	}
	// a predator seen across a fence or water can't get at it: no reason
	// to run (running would take it out through a gate, into the threat)
	if threat && !g.reachesFoe(false, tp, pos) {
		threat = false
	}
	if !threat && scan {
		tp, threat = g.animalFireThreat(a, pos) // fire_danger.go: every animal runs from fire
	}
	if now < a.FleeUntil && a.Attacker != 0 {
		if p, ok := g.positions.Get(a.Attacker); ok {
			tp, threat = p, true
		}
	}
	if threat {
		lost := a.PathI >= len(a.Path) && now >= a.RetryAt
		if a.Mode != animal.Flee || lost || (now+uint64(id))%40 == 0 {
			if a.Mode != animal.Flee {
				a.Feed, a.FeedLeft, a.Target = animal.FeedTarget{}, 0, 0 // a sleeper wakes, a grazer drops its meal
				a.Path, a.PathI = nil, 0
			}
			a.Mode = animal.Flee
			g.fleeFrom(id, a, pos, tp, dist, false) // animal_flee.go
		}
		return true
	}
	if a.Mode == animal.Flee {
		if a.PathI < len(a.Path) {
			return true // finish the run
		}
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
		a.NextDecide = now // the run is over: think again at once
	}
	return false
}

// fightCheck runs a wild animal's fight: a predator's hunt or revenge, a
// woken territorial animal's (animal_needs.go). A badly hurt fighter
// breaks off (T10) and a hunt that fails gives up (T5). It reports true
// while the animal fights.
func (g *Game) fightCheck(id entity.EntityID, a *animal.State, pos Position, def animal.Def, now uint64) bool {
	// prey, or whoever hurt it (its own kind too: animalHurt)
	if a.Target != 0 && !g.validPrey(id, a.Target) && !(a.Target == a.Attacker && g.validFoe(id, a.Target)) {
		g.endFight(a)
	}
	calm := !badlyHurt(*a) && now >= a.FleeUntil
	if a.Target == 0 && def.Predator && !a.Tame && calm && a.Attacker != 0 && g.validPrey(id, a.Attacker) &&
		g.reachesFoe(a.Tame, pos, g.posOf(a.Attacker)) { // never one it can't get at (no flee/attack flapping)
		a.Target = a.Attacker // revenge; on a colonist the player is warned (chase.go)
		g.warnChase(id, a, a.Target, false)
	}
	if a.Target == 0 {
		a.HuntSince = 0 // its prey died under another's teeth, or was never set
		if def.Predator {
			a.Attacker = 0
		}
		return false
	}
	if badlyHurt(*a) && !a.Enraged(now) && !def.Legendary {
		g.breakOff(id, a, pos, now)
		return true
	}
	tp, _ := g.positions.Get(a.Target)
	d := weapon.Distance(pos.X, pos.Y, tp.X, tp.Y)
	switch {
	case a.HuntSince != 0 && (now-a.HuntSince > animal.HuntGiveUp || d > huntRadius(def)+animal.HuntLeash ||
		a.Tame && !g.tameHuntOn(id, a.Target, tp)): // a tame hunter's prey is wild and in its area (PLAY-017)
		g.endFight(a) // the prey got away: try again later
		a.NextFeed, a.NextDecide = now+animal.HuntRetry, now
		return false
	case a.HuntSince == 0 && !def.Predator && d > territorialLeash:
		g.endFight(a) // chased it off
		return false
	}
	g.chase(id, a, now)
	return true
}

// chargeCheck is a wild predator's charge (PLAY-010 2.3 C). An awake,
// unhurt, calm predator with no target looks every animal.DecideEvery
// ticks (staggered by id) for a standing colonist or raider within
// animal.ChargeRadius and, when it finds one, rolls animal.ChargePct;
// any roll waits animal.ChargeCooldown for the next. A charge is an
// Attack on that person (fightCheck chases it) that ends beyond
// animal.ChargeLeash, after animal.ChargeTicks, or once the target is
// down or the animal turned to something else; the cooldown starts then.
// Off where predators leave colonists alone (Wanderer, Gentle).
func (g *Game) chargeCheck(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	if a.ChargeSince != 0 {
		tp, ok := g.positions.Get(a.Target)
		if a.Tame || a.Mode != animal.Attack || !ok || !g.standingPawn(a.Target) ||
			now-a.ChargeSince > animal.ChargeTicks || weapon.Distance(pos.X, pos.Y, tp.X, tp.Y) > animal.ChargeLeash {
			if a.Mode == animal.Attack && a.Target != 0 {
				if a.Attacker == a.Target {
					a.Attacker = 0 // it let them go: no revenge straight after
				}
				g.endFight(a)
			}
			a.ChargeSince, a.NextCharge = 0, now+animal.ChargeCooldown
		}
		return
	}
	if a.Tame || !animal.Charger(a.Species) || now < a.NextCharge || (now+uint64(id))%animal.DecideEvery != 0 ||
		a.Target != 0 || a.Mode == animal.Flee || sleeping(*a) || badlyHurt(*a) || a.Enraged(now) || now < a.FleeUntil ||
		!g.predatorsHuntColonists() { // story.go: difficulty (NG-1)
		return
	}
	who, _, ok := g.nearestWhere(pos.X, pos.Y, animal.ChargeRadius, ckPawn|ckRaider, func(o entity.EntityID, _ Position) bool {
		return g.standingPawn(o)
	})
	if !ok {
		return
	}
	a.NextCharge = now + animal.ChargeCooldown
	if g.rng.IntN(100) >= animal.ChargePct {
		return
	}
	a.Mode, a.Target, a.HuntSince, a.ChargeSince = animal.Attack, who, 0, now
	a.Path, a.PathI, a.Feed, a.FeedLeft = nil, 0, animal.FeedTarget{}, 0
	g.logEvent(who, logEntry{Kind: logChargedBy, Other: id, Name: g.killerName(id)})
	g.logEvent(id, logEntry{Kind: logCharged, Other: who, Name: g.creatureName(who)})
	g.warnChase(id, a, who, true) // chase.go: "A cougar charges Ada!" (raiders are skipped)
}

// posOf is id's position (the zero cell when it has none).
func (g *Game) posOf(id entity.EntityID) Position {
	p, _ := g.positions.Get(id)
	return p
}

// nearestThreat: the closest standing colonist or predator within r.
func (g *Game) nearestThreat(id entity.EntityID, pos Position, r int) (Position, bool) {
	_, p, ok := g.nearestWhere(pos.X, pos.Y, r, ckPawn|ckAnimal, func(o entity.EntityID, _ Position) bool {
		if o == id {
			return false
		}
		if a := g.animals.Ref(o); a != nil {
			return animal.DefOf(a.Species).Predator
		}
		return g.standingPawn(o)
	})
	return p, ok
}

// validPrey: a living animal of another species, or a standing colonist.
func (g *Game) validPrey(self, t entity.EntityID) bool {
	if t == self {
		return false
	}
	if o := g.animals.Ref(t); o != nil {
		me := g.animals.Ref(self)
		if me == nil {
			me = &animal.State{}
		}
		return !o.Dead() && (o.Species != me.Species || o.Tame != me.Tame)
	}
	return g.standingPawn(t)
}

// findPrey: the nearest animal the predator can take within its hunt
// radius (huntable), else a standing colonist within AggroRadius. A pack
// predator's mates nearby join the hunt.
func (g *Game) findPrey(id entity.EntityID, pos Position, def animal.Def) entity.EntityID {
	best, _, ok := g.nearestWhere(pos.X, pos.Y, huntRadius(def), ckAnimal, func(o entity.EntityID, _ Position) bool {
		return g.huntable(id, o)
	})
	if ok {
		if def.Pack {
			g.callPack(id, pos, best)
		}
		return best
	}
	if !g.predatorsHuntColonists() { // story.go: difficulty (NG-1)
		return 0
	}
	best, _, _ = g.nearestWhere(pos.X, pos.Y, def.AggroRadius, ckPawn, func(o entity.EntityID, _ Position) bool {
		return g.standingPawn(o)
	})
	return best
}

// huntRadius is how far an animal looks for prey: its species' hunt
// radius, or smallPreyRadius for a meat eater that is no predator.
func huntRadius(def animal.Def) int {
	if def.HuntRadius > 0 {
		return def.HuntRadius
	}
	return smallPreyRadius
}

// huntable: prey a predator can take: a living non-predator no bigger
// than itself (XS or S only for a small-prey hunter, or a meat eater that
// is no predator), never a hive insect or a legendary.
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
	if md.SmallPrey || !md.Predator {
		return size <= animal.SizeS
	}
	return size <= animal.RosterOf(me.Species).Size
}

// callPack sets idle wild pack mates of id's species within packRadius on
// the same prey.
func (g *Game) callPack(id entity.EntityID, pos Position, prey entity.EntityID) {
	me, _ := g.animals.Get(id)
	now := uint64(g.clock.Now())
	g.within(pos.X, pos.Y, packRadius, ckAnimal, func(aid entity.EntityID, _ Position) {
		if aid == id {
			return
		}
		o, ok := g.animals.Get(aid)
		if !ok || o.Species != me.Species || o.Tame != me.Tame || o.Target != 0 || o.Manhunter != 0 || badlyHurt(o) || !g.grown(o, now) {
			return
		}
		o.Target, o.HuntSince = prey, now
		o.Feed, o.FeedLeft = animal.FeedTarget{}, 0
		g.animals.Set(aid, o)
	})
}

// animalGoTo paths towards (gx, gy), clamped to the map and nudged to a
// nearby walkable tile.
func (g *Game) animalGoTo(id entity.EntityID, a *animal.State, gx, gy int) {
	gates := g.gatesOpen(id, a) // rope.go: livestock pass gates only when led
	w, h := g.grid.Width(), g.grid.Height()
	gx, gy = clampInt(gx, 0, w-1), clampInt(gy, 0, h-1)
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
	ar := g.animals.Ref(id)
	if ar == nil {
		return // removed earlier this tick
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
			g.pawnLetter("Bonded animal died", fmt.Sprintf("%s %s has lost a bonded companion.", text, g.personName(c)), c)
		} else {
			g.message(pos.X, pos.Y, "%s", text)
		}
	}
	if by != 0 && by != id {
		g.logEvent(by, logEntry{Kind: logKilled, Other: id, Name: animalLabel(a)})
	}
	// the colony's death (wildlife.go), else the corpse is unclaimed (PLAY-017)
	colony := a.Tame || g.colonyKill(by, pos)
	g.onAnimalDied(id, by) // bonds.go: the bonded colonist's grief, before the state goes
	g.removeAnimal(id)
	g.stainAnimalDeath(a.Species, pos) // stains.go
	corpse := g.dropCorpseAs(a.Species, pos.X, pos.Y, !colony)
	if k, ok := g.animals.Get(by); ok && (animal.DefOf(k.Species).Predator || k.HuntSince != 0) {
		if k.Tame && k.HuntSince != 0 { // a quiet line: the colony's hunter fed itself
			g.post(MessageView{Tick: uint64(g.clock.Now()), Text: fmt.Sprintf("%s hunted %s.", animalLabel(k), withArticle(a.Species.Label())),
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
		g.reserve.ReleaseEntity(id, a.FeedClaim) // a tame eater's stack is free again
	}
	delete(g.logs, id)
	delete(g.tendedUntil, id) // vet_animals.go
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
		sv := speciesViewOf(a.Species) // animal_view_species.go
		out = append(out, AnimalView{
			ID: id, Species: a.Species.String(), Label: sv.label, X: p.X, Y: p.Y,
			Attack: sv.attack, Female: animalFemale(id, a),
			Size:       sv.size,
			Health:     a.HP() * 100 / sv.maxHP,
			Mode:       a.Mode.String(),
			Designated: a.Designated,
			Hostile:    g.hostile(a) || a.Enraged(uint64(g.clock.Now())),
			TargetID:   g.chaseTarget(a),
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
		g.fillAnimalAction(av, id, actionState(a))
		if av.Sleeping = sleeping(a); av.Sleeping {
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
