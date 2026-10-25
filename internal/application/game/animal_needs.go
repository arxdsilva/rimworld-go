package game

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
)

// Animal needs and the behaviour state machine (SIM-024, plan 2.3). Food
// and rest drain every tick (integer steps, no randomness); every
// animal.DecideEvery ticks, staggered by id, or at once when something
// happens to it (hurt, a run or a meal finished), an animal decides:
//
//	T1  Wander -> Feed    Food < FeedStart and now >= NextFeed
//	T2  Feed -> Wander    Food >= FeedStop, or nothing left to eat (NextFeed = now + NoFoodRetry)
//	T3  Feed -> hunt      a wild meat eater that found no carcass or meat
//	T4  hunt -> Feed      it killed: the carcass is its meal (killAnimal)
//	T5  hunt -> Wander    the prey got away or no strike in HuntGiveUp (NextFeed = now + HuntRetry)
//	T6  -> Rest           sleep hours and Rest < RestStart (not while hungry), or Rest < RestCollapse
//	T7  Rest -> Wander    Rest full, or out of sleep hours and Rest >= RestWakeDay
//	T8  Rest -> Attack    a sleeping territorial animal or predator is hurt
//	T9  -> Flee           threats (fleeCheck, animals.go): a sleeper wakes
//	T10 fight -> Flee     a badly hurt fighter breaks off
//	T12 -> Leave          Leaving: walks off the map (no feeding or rest)
//	T13 starving          Food 0: an injury every StarveEvery ticks
//
// Every start threshold has a stop threshold or a cooldown apart from it,
// so no animal flaps between two states. A hunt runs in Attack mode
// (movement and combat treat it as a fight) with HuntSince set. Hive
// insects and legendaries have no needs; manhunters ignore theirs.

const (
	// territorialLeash: a woken territorial animal chases its attacker this
	// far before letting it go.
	territorialLeash = 12
	// smallPreyRadius: how far a meat eater that is no predator looks for
	// small prey.
	smallPreyRadius = 12
	// herdBedRadius: a resting animal looks this far for a herd mate to
	// bed down beside, and lies within herdBedNear of it.
	herdBedRadius = 16
	herdBedNear   = 3
	// feedWalkLimit: an animal that has not reached its meal this long
	// after picking it gives it up.
	feedWalkLimit = 1500
	// breakOffTicks: a badly hurt fighter runs this long.
	breakOffTicks = 900
	// cropRaidKeep: crops eaten this recently feed the alert.
	cropRaidKeep = clock.TicksPerHour
)

// cropRaid is one crop meal of a wild animal (saved with the wildlife
// state; it feeds the "eating crops" alert).
type cropRaid struct {
	X, Y    int
	Tick    uint64
	Species animal.Species
}

// CropRaidView is a crop a wild animal ate in the last hour.
type CropRaidView struct {
	X, Y    int
	Tick    uint64
	Species string
}

// CropRaids lists the crops wild animals ate in the last hour, oldest
// first (read-only; for the "Wild animals eating crops" alert).
func (g *Game) CropRaids() []CropRaidView {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	now := uint64(g.clock.Now())
	var out []CropRaidView
	for _, c := range g.wl.CropRaids {
		if now-c.Tick <= cropRaidKeep {
			out = append(out, CropRaidView{X: c.X, Y: c.Y, Tick: c.Tick, Species: c.Species.String()})
		}
	}
	return out
}

func (g *Game) pruneCropRaids(now uint64) {
	if len(g.wl.CropRaids) == 0 {
		return
	}
	kept := g.wl.CropRaids[:0]
	for _, c := range g.wl.CropRaids {
		if now-c.Tick <= cropRaidKeep {
			kept = append(kept, c)
		}
	}
	g.wl.CropRaids = kept
}

// rateStep is this tick's share of per units over period ticks: the
// difference of a running total, so the sum over a period is exact.
func rateStep(now uint64, per, period int) int {
	if now == 0 {
		return 0
	}
	p, q := uint64(per), uint64(period)
	return int(now*p/q - (now-1)*p/q)
}

// sleeping: an animal lying down (resting and not walking to its bed).
func sleeping(a animal.State) bool { return a.Mode == animal.Rest && a.PathI >= len(a.Path) }

// tickNeeds drains food and rest (rest refills while asleep, food then
// drains at half speed) and starves an animal with no food left. It
// reports false when the animal starved to death.
func (g *Game) tickNeeds(id entity.EntityID, a *animal.State, now uint64) bool {
	a.Normalise(uint64(id))
	if animal.NeedsFree(a.Species) {
		a.Food, a.Rest = animal.Full, animal.Full
		return true
	}
	if sleeping(*a) {
		a.Food -= rateStep(now, animal.FoodPerDay/2, clock.TicksPerDay)
		a.Rest = min(animal.Full, a.Rest+rateStep(now, animal.SleepPerHour, clock.TicksPerHour))
	} else {
		a.Food -= rateStep(now, animal.FoodPerDay, clock.TicksPerDay)
		a.Rest = max(0, a.Rest-rateStep(now, animal.RestPerDay, clock.TicksPerDay))
	}
	if a.Food > 0 {
		a.Starving = 0
		return true
	}
	a.Food = 0
	if a.Starving == 0 {
		a.Starving = now
		g.logEvent(id, logEntry{Kind: logStarving})
		return true
	}
	if (now-a.Starving)%animal.StarveEvery != 0 {
		return true
	}
	a.Injury++
	if !a.Dead() {
		return true
	}
	g.animals.Set(id, *a)
	g.killAnimal(id, 0)
	return false
}

// nextDecide is the next tick of id's decision cycle after now.
func nextDecide(id entity.EntityID, now uint64) uint64 {
	return now + animal.DecideEvery - (now+uint64(id))%animal.DecideEvery
}

// decideAnimal is an animal's full decision (T1-T3, T6, T7): rest, feed or
// wander. Fleeing, fighting and leaving animals never get here.
func (g *Game) decideAnimal(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	a.NextDecide = nextDecide(id, now)
	g.bondDecide(id, a, pos, now) // bonds.go: healing and the proximity bond, tame animals only
	if animal.NeedsFree(a.Species) || a.Enraged(now) {
		return
	}
	sleepy := animal.SleepHours(a.Species, g.Date().Hour)
	if a.Mode == animal.Rest {
		if a.Rest >= animal.Full || !sleepy && a.Rest >= animal.RestWakeDay { // T7
			a.Mode, a.Path, a.PathI, a.NextThink = animal.Wander, nil, 0, now
		}
		return
	}
	if a.Mode != animal.Wander && a.Mode != animal.Feed {
		return
	}
	if a.Rest < animal.RestCollapse || sleepy && a.Rest < animal.RestStart && a.Food >= animal.FeedStart { // T6
		g.startRest(id, a, pos)
		return
	}
	if a.Mode == animal.Feed {
		if a.Food >= animal.FeedStop { // T2: full
			a.Mode, a.Feed, a.FeedLeft = animal.Wander, animal.FeedTarget{}, 0
			return
		}
		if a.Feed.Kind != animal.FeedNone && g.feedThere(a, a.Feed) {
			return // on its way, or eating
		}
		if !g.pickFeed(id, a, pos, now) {
			g.nothingToEat(id, a, pos, now)
		}
		return
	}
	if a.Food < animal.FeedStart && now >= a.NextFeed { // T1
		if !g.pickFeed(id, a, pos, now) {
			g.nothingToEat(id, a, pos, now)
		}
	}
}

// nothingToEat: nothing to eat in reach. A wild meat eater hunts (T3); anyone
// else wanders and looks again after NoFoodRetry (T2).
func (g *Game) nothingToEat(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	def := animal.DefOf(a.Species)
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	hunter := def.Predator || animal.EcologyOf(a.Species).Diet == animal.Carnivore
	if !a.Tame && hunter && a.Food < animal.FeedStart && !badlyHurt(*a) && now >= a.FleeUntil && g.grown(*a, now) { // T3 (never while hurt: T10; the young don't hunt)
		if prey := g.findPrey(id, pos, def); prey != 0 {
			a.Mode, a.Target, a.HuntSince = animal.Attack, prey, now
			return
		}
	}
	a.Mode, a.NextFeed = animal.Wander, now+animal.NoFoodRetry
}

// startRest beds an animal down (T6): a wild one beside a resting herd
// mate when one is near, else where it stands (a tame one too: livestock
// can't leave their pen, pens.go).
func (g *Game) startRest(id entity.EntityID, a *animal.State, pos Position) {
	a.Mode, a.Path, a.PathI = animal.Rest, nil, 0
	a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
	if a.Tame {
		return // livestock sleep in their pen or where they roam; pets where they are
	}
	if a.Herd == 0 {
		return
	}
	_, mp, ok := g.nearestWhere(pos.X, pos.Y, herdBedRadius, ckAnimal, func(o entity.EntityID, _ Position) bool {
		m, ok := g.animals.Get(o)
		return ok && o != id && m.Herd == a.Herd && !m.Tame && m.Mode == animal.Rest
	})
	if ok && weapon.Distance(pos.X, pos.Y, mp.X, mp.Y) > herdBedNear {
		g.animalGoTo(id, a, mp.X+clampInt(pos.X-mp.X, -2, 2), mp.Y+clampInt(pos.Y-mp.Y, -2, 2))
	}
}

// walkOff takes a leaving animal to its exit and off the map (T12).
func (g *Game) walkOff(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	a.Mode = animal.Leave
	if weapon.Distance(pos.X, pos.Y, a.ExitX, a.ExitY) <= 1 {
		g.leaveMap(id)
		return
	}
	if a.PathI >= len(a.Path) && now >= a.RetryAt {
		g.animalGoTo(id, a, a.ExitX, a.ExitY)
	}
}

// startLeaving sends animal id off the map at (x, y) (population and
// event code: a herd over capacity, a crossing).
func (g *Game) startLeaving(id entity.EntityID, x, y int) {
	a, ok := g.animals.Get(id)
	if !ok {
		return
	}
	a.Leaving, a.ExitX, a.ExitY = true, x, y
	a.Mode, a.Path, a.PathI, a.Feed, a.FeedLeft = animal.Leave, nil, 0, animal.FeedTarget{}, 0
	g.animals.Set(id, a)
}

// leaveMap removes an animal that walked off the map (a tame one is lost
// to its bonded colonist).
func (g *Game) leaveMap(id entity.EntityID) {
	if a := g.animals.Ref(id); a != nil && a.Tame {
		g.onAnimalLost(id) // bonds.go
	}
	g.removeAnimal(id)
}

// badlyHurt: below animal.BadlyHurt percent of its hit points.
func badlyHurt(a animal.State) bool {
	return a.HP()*100 < animal.DefOf(a.Species).MaxHP*animal.BadlyHurt
}

// breakOff: a badly hurt fighter runs from its foe (T10).
func (g *Game) breakOff(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	foe := a.Target
	if a.Attacker != 0 {
		foe = a.Attacker
	}
	g.endFight(a)
	a.Attacker, a.FleeUntil, a.Mode = foe, now+breakOffTicks, animal.Flee
	if fp, ok := g.positions.Get(foe); ok {
		fx, fy := animal.FleeGoal(pos.X, pos.Y, fp.X, fp.Y, 8)
		g.animalGoTo(id, a, fx, fy)
	}
	g.logEvent(id, logEntry{Kind: logFled, Other: foe, Name: g.creatureName(foe)})
}

// endFight drops an animal's target and hunt.
func (g *Game) endFight(a *animal.State) {
	a.Target, a.HuntSince = 0, 0
	if a.Mode == animal.Attack {
		a.Mode, a.Path, a.PathI = animal.Wander, nil, 0
	}
}

// animalHurt is the reaction side of an animal being hurt (combat.go
// hurt; PLAY-010): it thinks again at once, and a sleeper wakes. With an
// attacker it always fights or runs: a fighter (canFightBack) goes for its
// attacker, keeping a foe it already has; anyone who can't fight back
// runs on every hit (fleeHit). A manhunter fights on. Traps and bursts
// (no attacker) only wake it, and a wild flee-class animal runs.
func (g *Game) animalHurt(id entity.EntityID, a *animal.State, by entity.EntityID, now uint64) {
	a.NextDecide = now
	if by == 0 || by == id {
		g.hurtBlind(a, now)
		return
	}
	if a.Enraged(now) {
		return // an enraged animal fights on instead of bolting (no flee/attack flapping)
	}
	if g.canFightBack(id, *a, by) {
		fightBack(a, by, g.validFoe(id, a.Target))
		return
	}
	g.fleeHit(id, a, by, now)
}

// feedOnKill: the killer of an animal feeds on its carcass (T4) when it
// eats meat and has room for it.
func (g *Game) feedOnKill(by entity.EntityID, k *animal.State, corpse entity.EntityID, at Position) {
	k.Target, k.Attacker, k.HuntSince = 0, 0, 0
	k.Mode, k.Path, k.PathI = animal.Wander, nil, 0
	if corpse == 0 || k.Tame || !animal.EcologyOf(k.Species).Diet.Meat() || k.Food >= animal.FeedStop {
		return
	}
	k.Mode = animal.Feed
	k.Feed = animal.FeedTarget{Kind: animal.FeedCarcass, ID: corpse, X: at.X, Y: at.Y}
	k.FeedLeft = animal.CarcassTicks
	k.NextFeed = uint64(g.clock.Now()) + feedWalkLimit
}

// --- the food search (plan 2.4) -------------------------------------------

// pickFeed finds an animal's next meal and sets off for it: a tame animal
// in an enclosed pen first eats its pen's grass and plants, then from the
// pen's troughs; anyone else (an area animal inside its allowed area) the
// nearest food within animal.FeedRadius, by kind:
// carcass, loose raw food (wild only), grass, wild plants, crops (wild
// only), living trees (tree-eaters).
func (g *Game) pickFeed(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	diet := animal.EcologyOf(a.Species).Diet
	var allow func(x, y int) bool
	if a.Tame {
		if m, c := g.penAt(pos.X, pos.Y); m != 0 { // pens.go
			inPen := func(x, y int) bool { return g.inPenCells(c, x, y) }
			if t, ok := g.findFeed(a, pos, diet, inPen); ok {
				return g.startFeed(a, t, now)
			}
			if t, ok := g.penTrough(a, m, pos, diet); ok { // trough_feed.go
				return g.startFeed(a, t, now)
			}
		}
		if !penned(a) {
			allow = func(x, y int) bool { return g.inArea(id, x, y) } // areas.go: an area animal eats inside it
		}
	}
	if t, ok := g.findFeed(a, pos, diet, allow); ok {
		return g.startFeed(a, t, now)
	}
	return false
}

func (g *Game) startFeed(a *animal.State, t animal.FeedTarget, now uint64) bool {
	a.Mode, a.Feed, a.FeedLeft = animal.Feed, t, eatTicks(t.Kind)
	a.Path, a.PathI, a.NextFeed = nil, 0, now+feedWalkLimit
	return true
}

func eatTicks(k animal.FeedKind) int {
	switch k {
	case animal.FeedTree:
		return animal.TreeTicks
	case animal.FeedCarcass:
		return animal.CarcassTicks
	case animal.FeedItem, animal.FeedTrough:
		return animal.ItemTicks
	}
	return animal.GrazeTicks
}

// feedBest is the best candidate of one food kind so far.
type feedBest struct {
	t  animal.FeedTarget
	d  int
	ok bool
}

func (b *feedBest) offer(k animal.FeedKind, id entity.EntityID, x, y, d int) {
	if !b.ok || d < b.d || d == b.d && id < b.t.ID {
		*b = feedBest{t: animal.FeedTarget{Kind: k, ID: id, X: x, Y: y}, d: d, ok: true}
	}
}

// findFeed is the meal an animal of diet would pick within FeedRadius of
// pos (only tiles allow accepts, when it is set).
func (g *Game) findFeed(a *animal.State, pos Position, diet animal.Diet, allow func(x, y int) bool) (animal.FeedTarget, bool) {
	const r = animal.FeedRadius
	reach := g.fenceReachable(a, pos) // fences.go: nothing behind a fence
	ok := func(x, y int) bool { return (allow == nil || allow(x, y)) && (reach == nil || reach(x, y)) }
	fx := g.food()
	if diet.Meat() || diet.Plants() && !a.Tame {
		var carcass, raw feedBest
		fx.items.each(pos.X, pos.Y, r, func(iid entity.EntityID) bool {
			st, have := g.stacks.Get(iid)
			if !have || st.Carrier != 0 || st.Qty <= 0 || !ok(st.X, st.Y) {
				return true
			}
			d := weapon.Distance(pos.X, pos.Y, st.X, st.Y)
			if d > r || g.stockpiled(st.X, st.Y) { // never from the colony's stores
				return true
			}
			if _, taken := g.reserve.EntityOwner(iid); taken {
				return true
			}
			switch {
			case st.Kind == item.CorpseAnimal && diet.Meat() && animal.Species(st.Animal) != a.Species:
				carcass.offer(animal.FeedCarcass, iid, st.X, st.Y, d)
			case !a.Tame && !st.Rotten() && animal.FeedValue(diet, st.Kind) > 0:
				raw.offer(animal.FeedItem, iid, st.X, st.Y, d)
			}
			return true
		})
		if carcass.ok {
			return carcass.t, true
		}
		if raw.ok {
			return raw.t, true
		}
	}
	if diet.Plants() {
		if x, y, found := g.nearestGrass(pos, r, ok); found {
			return animal.FeedTarget{Kind: animal.FeedGrass, X: x, Y: y}, true
		}
	}
	if !diet.Plants() && diet != animal.Dendrovore {
		return animal.FeedTarget{}, false
	}
	var browse, crop, tree feedBest
	fx.plants.each(pos.X, pos.Y, r, func(pid entity.EntityID) bool {
		p, have := g.plants.Get(pid)
		if !have || !ok(p.X, p.Y) {
			return true
		}
		d := weapon.Distance(pos.X, pos.Y, p.X, p.Y)
		if d > r {
			return true
		}
		pd := plant.DefOf(p.Species)
		switch {
		case diet == animal.Dendrovore:
			if pd.Trunk && !p.Zone && p.Percent() >= 50 {
				tree.offer(animal.FeedTree, pid, p.X, p.Y, d)
			}
		case p.Zone && pd.Crop:
			if !a.Tame && p.Percent() >= animal.CropDies+5 {
				crop.offer(animal.FeedPlant, pid, p.X, p.Y, d)
			}
		case pd.Browse && p.Percent() >= 30:
			browse.offer(animal.FeedPlant, pid, p.X, p.Y, d)
		}
		return true
	})
	for _, b := range []feedBest{browse, crop, tree} {
		if b.ok {
			return b.t, true
		}
	}
	return animal.FeedTarget{}, false
}

// nearestGrass is the grass tile a grazer within r of pos picks: the
// nearest walkable one, ring by ring (Chebyshev), each ring in a fixed
// order. While some grass is regrowing (PLAY-010) it weighs growth too:
// the fullest tile of the first ring holding grass and the ring after it
// (ties to ring order, so a full field picks exactly the nearest tile),
// passing over shoots under plant.GrassGrazeMin unless no fuller grass is
// in reach at all. That threshold is the hysteresis that keeps a grazer
// off the tiles it just bared.
func (g *Game) nearestGrass(pos Position, r int, ok func(x, y int) bool) (int, int, bool) {
	try := func(x, y int) bool {
		return g.grid.InBounds(x, y) && g.hasGrass(x, y) && g.grid.Walkable(x, y) && ok(x, y)
	}
	if g.grassSt().nGrowing == 0 { // every tile full: the nearest
		var bx, by int
		found := false
		grassRings(pos, 0, r, func(x, y int) bool {
			if try(x, y) {
				bx, by, found = x, y, true
			}
			return found
		})
		return bx, by, found
	}
	if x, y, found := g.fullestGrass(pos, r, plant.GrassGrazeMin, try); found {
		return x, y, true
	}
	return g.fullestGrass(pos, r, 1, try)
}

// fullestGrass is the fullest grass tile (growth at least least) of the
// first ring within r of pos holding one and the ring after it; ties go
// to the nearer ring, then ring order.
func (g *Game) fullestGrass(pos Position, r, least int, try func(x, y int) bool) (int, int, bool) {
	bx, by, best, last := 0, 0, 0, r
	grassRings(pos, 0, r, func(x, y int) bool {
		if d := max(abs(x-pos.X), abs(y-pos.Y)); d > last {
			return true
		}
		if !try(x, y) {
			return false
		}
		if v := g.grassGrowthAt(x, y); v >= least && v > best {
			if best == 0 {
				last = min(r, max(abs(x-pos.X), abs(y-pos.Y))+1)
			}
			bx, by, best = x, y, v
		}
		return false
	})
	return bx, by, best > 0
}

// grassRings visits the tiles at Chebyshev distance d0..r from pos, ring
// by ring, each ring in nearestGrass's fixed order, until fn says stop.
func grassRings(pos Position, d0, r int, fn func(x, y int) bool) {
	if d0 == 0 {
		if fn(pos.X, pos.Y) {
			return
		}
		d0 = 1
	}
	for d := d0; d <= r; d++ {
		for x := pos.X - d; x <= pos.X+d; x++ {
			if fn(x, pos.Y-d) || fn(x, pos.Y+d) {
				return
			}
		}
		for y := pos.Y - d + 1; y <= pos.Y+d-1; y++ {
			if fn(pos.X-d, y) || fn(pos.X+d, y) {
				return
			}
		}
	}
}

// stockpiled: (x, y) lies in a stockpile.
func (g *Game) stockpiled(x, y int) bool {
	_, ok := g.zoneOf(zone.KindStockpile, x, y)
	return ok
}

// feedThere reports that an animal's meal t is still there to eat.
func (g *Game) feedThere(a *animal.State, t animal.FeedTarget) bool {
	switch t.Kind {
	case animal.FeedGrass:
		return g.hasGrass(t.X, t.Y)
	case animal.FeedPlant, animal.FeedTree:
		p, ok := g.plants.Get(t.ID)
		return ok && p.X == t.X && p.Y == t.Y
	case animal.FeedItem, animal.FeedCarcass:
		st, ok := g.stacks.Get(t.ID)
		return ok && st.Carrier == 0 && st.Qty > 0 && st.X == t.X && st.Y == t.Y
	case animal.FeedTrough:
		return g.troughHas(t.ID, animal.EcologyOf(a.Species).Diet) // trough_feed.go
	}
	return false
}

// feedReach is how close an animal stands to eat t: on the grass tile, or
// beside anything else.
func feedReach(t animal.FeedTarget) int {
	if t.Kind == animal.FeedGrass {
		return 0
	}
	return 1
}

// feedSpot is the tile an animal walks to for t: the tile itself when it
// can stand there, else the walkable neighbour nearest to pos.
func (g *Game) feedSpot(pos Position, t animal.FeedTarget) (int, int, bool) {
	if t.Kind != animal.FeedTrough && g.grid.Walkable(t.X, t.Y) {
		return t.X, t.Y, true
	}
	bx, by, bestD := 0, 0, 1<<30
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := t.X+dx, t.Y+dy
			if (dx != 0 || dy != 0) && g.grid.Walkable(x, y) {
				if d := weapon.Distance(pos.X, pos.Y, x, y); d < bestD {
					bx, by, bestD = x, y, d
				}
			}
		}
	}
	return bx, by, bestD < 1<<30
}

// feedStep walks a feeding animal to its meal and eats it.
func (g *Game) feedStep(id entity.EntityID, a *animal.State, pos Position, now uint64) {
	t := a.Feed
	if t.Kind == animal.FeedNone || !g.feedThere(a, t) {
		a.Feed, a.FeedLeft, a.NextDecide = animal.FeedTarget{}, 0, now // gone: look again
		return
	}
	if weapon.Distance(pos.X, pos.Y, t.X, t.Y) > feedReach(t) {
		switch {
		case a.PathI < len(a.Path):
		case now > a.NextFeed: // can't get there
			a.Mode, a.Feed, a.FeedLeft = animal.Wander, animal.FeedTarget{}, 0
			a.NextFeed, a.NextDecide = now+animal.NoFoodRetry, now
		case now >= a.RetryAt:
			if x, y, ok := g.feedSpot(pos, t); ok {
				g.animalGoTo(id, a, x, y)
			} else {
				a.NextFeed = now // no way to stand by it: give up next tick
			}
		}
		return
	}
	a.Path, a.PathI = nil, 0
	if a.FeedLeft--; a.FeedLeft > 0 {
		return
	}
	g.eat(id, a, t, now)
}

// eat finishes a meal: the food is consumed (or eaten down) and the
// animal's food rises by the meal for its size. Young animals eat half
// portions (portion, animal_life.go): the same meal fills them
// animal.YoungMeal times as far, so a stack or trough feeds them with
// half the items.
func (g *Game) eat(id entity.EntityID, a *animal.State, t animal.FeedTarget, now uint64) {
	a.Feed, a.FeedLeft, a.NextDecide = animal.FeedTarget{}, 0, now
	diet := animal.EcologyOf(a.Species).Diet
	gain := 0
	switch t.Kind {
	case animal.FeedGrass:
		growth := g.grassGrowthAt(t.X, t.Y) // PLAY-010: shoots feed less
		if g.grazeGrass(t.X, t.Y) {         // grass.go: bare, then regrows
			gain = plant.GrassMeal(animal.GrazeFood, growth)
		}
	case animal.FeedTree:
		g.removePlant(t.ID) // gnawed down: no wood
		gain = animal.TreeFood
	case animal.FeedPlant:
		gain = g.browse(id, a, t, now)
	case animal.FeedCarcass:
		st, _ := g.stacks.Get(t.ID)
		meal := animal.CarcassFood(animal.Species(st.Animal))
		gain = min(meal, animal.Full) // a.Food is capped below
		g.logEvent(id, logEntry{Kind: logAte, Name: animal.Species(st.Animal).Label() + " carcass"})
		g.takeItems(t.ID, st, 1)
		g.shareCarcass(id, t, meal-animal.Meal(a.Species, animal.Full-a.Food), now)
	case animal.FeedItem:
		st, _ := g.stacks.Get(t.ID)
		per := g.portion(a, now, animal.Meal(a.Species, animal.FeedValue(diet, st.Kind)))
		n := min(st.Qty, max(1, (animal.Full-a.Food+per-1)/max(per, 1)))
		g.takeItems(t.ID, st, n)
		a.Food = min(animal.Full, a.Food+n*per)
		return
	case animal.FeedTrough:
		// trough_feed.go; the young want half as much from it
		k := g.portion(a, now, 1)
		a.Food = min(animal.Full, a.Food+k*g.eatFromTrough(t.ID, a.Species, diet, (animal.Full-a.Food+k-1)/k))
		return
	}
	a.Food = min(animal.Full, a.Food+g.portion(a, now, animal.Meal(a.Species, gain)))
}

// shareCarcass: the animals feeding on the same carcass beside the one
// that finished it share what is left (left food, before body size), in
// id order: a pack eats one kill together.
func (g *Game) shareCarcass(eater entity.EntityID, t animal.FeedTarget, left int, now uint64) {
	if left <= 0 {
		return
	}
	g.within(t.X, t.Y, 2, ckAnimal, func(o entity.EntityID, _ Position) {
		m, ok := g.animals.Get(o)
		if !ok || o == eater || left <= 0 || m.Mode != animal.Feed || m.Feed.Kind != animal.FeedCarcass || m.Feed.ID != t.ID {
			return
		}
		got := min(animal.Meal(m.Species, left), animal.Full-m.Food)
		m.Food += got
		left -= got * 100 / max(1, animal.Meal(m.Species, 100))
		m.Feed, m.FeedLeft, m.NextDecide = animal.FeedTarget{}, 0, now
		g.logEvent(o, logEntry{Kind: logAte, Name: "a shared carcass"})
		g.animals.Set(o, m)
	})
}

// browse eats a plant down: a crop loses part of its growth (and dies when
// little is left), a regrowing bush is cropped back, anything else is
// eaten whole.
func (g *Game) browse(id entity.EntityID, a *animal.State, t animal.FeedTarget, now uint64) int {
	p, _ := g.plants.Get(t.ID)
	d := plant.DefOf(p.Species)
	if p.Zone && d.Crop {
		p.Growth -= d.GrowTicks * animal.CropBite / 100
		if p.Growth < 0 || p.Percent() < animal.CropDies {
			g.removePlant(t.ID)
		} else {
			g.plants.Set(t.ID, p)
		}
		if !a.Tame {
			g.wl.CropRaids = append(g.wl.CropRaids, cropRaid{X: t.X, Y: t.Y, Tick: now, Species: a.Species})
		}
		return animal.CropFood
	}
	if d.Regrows {
		p.Growth = d.GrowTicks * d.RegrowTo / 100
		g.plants.Set(t.ID, p)
	} else {
		g.removePlant(t.ID)
	}
	return animal.BrowseFood
}

// takeItems eats n of stack id (st is its current state).
func (g *Game) takeItems(id entity.EntityID, st item.Stack, n int) {
	if st.Qty <= n {
		g.deleteItem(id)
		return
	}
	st.Qty -= n
	g.stacks.Set(id, st)
}

// fillAnimalNeeds fills the needs and life fields of an animal view.
func (g *Game) fillAnimalNeeds(av *AnimalView, a animal.State) {
	now := uint64(g.clock.Now())
	av.State = a.Mode.String()
	if a.Mode == animal.Attack && a.HuntSince != 0 {
		av.State = "hunting"
	}
	av.Food, av.Rest = a.Food*100/animal.Full, a.Rest*100/animal.Full
	if !a.HasSex || animal.NeedsFree(a.Species) {
		av.Food, av.Rest = 100, 100
	}
	av.Diet = speciesViewOf(a.Species).diet // animal_view_species.go
	av.Leaving = a.Leaving || a.Mode == animal.Leave
	av.Herd = a.Herd
	days := animal.AgeDays(a.Born, now, a.Species)
	av.AgeDays, av.Adult = days, animal.Adult(a.Species, days)
	av.Stage, av.Scale = animal.Stage(a.Species, av.Adult), animal.GrowthScale(a.Species, days)
	g.fillAnimalLife(av, a, now) // animal_life.go
}
