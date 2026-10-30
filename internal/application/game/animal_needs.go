package game

import (
	"rimworld-go/internal/application/game/forage"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/region"
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
	if now%animal.DecideEvery == 0 { // FEED-001: tame animals' feeding memory (forage)
		g.wl.Forage.Prune(now, func(id entity.EntityID) bool { return g.animals.Ref(id) != nil })
	}
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
	if lyingDown(*a) { // animal_health.go: asleep or downed
		a.Food -= rateStep(now, animal.FoodPerDay/2, clock.TicksPerDay)
		a.Rest = min(animal.Full, a.Rest+rateStep(now, g.rest.Animals.SleepRate(id, a.Tame, animal.SleepPerHour), clock.TicksPerHour)) // BED-1: rest/animals.go
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
	hunter := def.Predator || animal.EcologyOf(a.Species).Diet.MeatOnly()
	if !a.Tame && hunter && a.Food < animal.FeedStart && !badlyHurt(*a) && now >= a.FleeUntil && g.grown(*a, now) { // T3 (never while hurt: T10; the young don't hunt)
		if prey := g.findPrey(id, pos, def); prey != 0 {
			a.Mode, a.Target, a.HuntSince = animal.Attack, prey, now
			g.warnChase(id, a, prey, false) // chase.go: a colonist as prey warns the player
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
	if penned(a) {
		return // livestock sleep in their pen or where they roam
	}
	if a.Tame { // BED-1: a pet goes to its animal bed (rest/animals.go), else sleeps where it is
		if bx, by, ok := g.rest.Animals.Bed(id, pos.X, pos.Y); ok {
			g.animalGoTo(id, a, bx, by)
		}
		return
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
// (no attacker) only wake it, and a wild flee-class animal runs. A hit
// that downs it (or one on a downed animal) gets no reaction.
func (g *Game) animalHurt(id entity.EntityID, a *animal.State, by entity.EntityID, now uint64) {
	a.NextDecide = now
	if a.DownsNow() {
		g.downAnimal(id, a, now) // animal_health.go: too hurt to stand
	}
	if a.Downed {
		return // lying where it fell: it neither fights back nor runs
	}
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
// eats meat and has room for it. A tame one eats only what it hunted
// (PLAY-017), holding the carcass against haulers while it eats.
func (g *Game) feedOnKill(by entity.EntityID, k *animal.State, corpse entity.EntityID, at Position) {
	hunted := k.HuntSince != 0
	k.Target, k.Attacker, k.HuntSince = 0, 0, 0
	k.Mode, k.Path, k.PathI = animal.Wander, nil, 0
	if corpse == 0 || k.Tame && !hunted || !animal.EcologyOf(k.Species).Diet.Meat() || k.Food >= animal.FeedStop {
		return
	}
	k.Mode = animal.Feed
	k.Feed = animal.FeedTarget{Kind: animal.FeedCarcass, ID: corpse, X: at.X, Y: at.Y}
	k.FeedLeft = animal.CarcassTicks
	k.NextFeed = uint64(g.clock.Now()) + feedWalkLimit
	if k.Tame {
		g.syncFeedClaim(by, k)
	}
}

// --- the food search (plan 2.4) -------------------------------------------

// pickFeed finds an animal's next meal and sets off for it. A wild
// animal takes the nearest food within animal.FeedRadius by kind
// (findFeed): carcass, loose raw food, grass, wild plants, crops, living
// trees (tree-eaters). A tame one (PLAY-017) takes the nearest meal of
// any kind its diet allows, or hunts (tameMeal); in an enclosed pen it
// first eats inside the pen, then from the pen's troughs; an area animal
// stays inside its allowed area.
func (g *Game) pickFeed(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	diet := animal.EcologyOf(a.Species).Diet
	if !a.Tame {
		if t, ok := g.findFeed(a, pos, diet, nil); ok {
			return g.startFeed(a, t, now)
		}
		return false
	}
	// a starving search (FEED-001) reaches the whole map, or a penned
	// animal's pen and the troughs beside it
	far := max(pos.X, g.grid.Width()-1-pos.X, pos.Y, g.grid.Height()-1-pos.Y)
	if m, c := g.penAt(pos.X, pos.Y); m != 0 { // pens.go
		if penned(a) {
			far = min(far, max(pos.X-c.minX, c.maxX-pos.X, pos.Y-c.minY, c.maxY-pos.Y)+1)
		}
		inPen := func(x, y int) bool { return g.inPenCells(c, x, y) }
		if g.tameMeal(id, a, pos, diet, inPen, far, now) {
			return true
		}
		if t, ok := g.penTrough(a, m, pos, diet); ok { // trough_feed.go
			return g.startFeed(a, t, now)
		}
	}
	var allow func(x, y int) bool
	if !penned(a) {
		allow = func(x, y int) bool { return g.inArea(id, x, y) } // areas.go: an area animal eats inside it
	}
	return g.tameMeal(id, a, pos, diet, allow, far, now)
}

// tameMeal sets a tame animal off for the meal findTameFeed picks: it
// walks to food (holding a stack it will eat against others), or hunts.
func (g *Game) tameMeal(id entity.EntityID, a *animal.State, pos Position, diet animal.Diet, allow func(x, y int) bool, far int, now uint64) bool {
	b := g.findTameFeed(id, a, pos, diet, allow, far, now)
	switch {
	case !b.OK:
		return false
	case b.Prey != 0:
		a.Mode, a.Target, a.HuntSince = animal.Attack, b.Prey, now
		a.Feed, a.FeedLeft = animal.FeedTarget{}, 0
		return true
	}
	g.startFeed(a, b.T, now)
	g.syncFeedClaim(id, a)
	return true
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
			case st.Kind == item.CorpseAnimal && diet.Meat() && animal.Species(st.Animal) != a.Species && !animal.IsMech(animal.Species(st.Animal)):
				carcass.offer(animal.FeedCarcass, iid, st.X, st.Y, d)
			case !a.Tame && !st.Rotten() && !animal.Prepared(st.Kind) && animal.FeedValue(diet, st.Kind) > 0:
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

// --- tame animals' food search (PLAY-017) --------------------------------

// findTameFeed is the meal a tame animal of diet picks (forage.Pick):
// animal feed first, a meal only when no feed is in reach (FEED-001),
// within FeedRadius of pos, or far (its whole map or pen) while starving
// (animal.FarSearch). Only tiles allow accepts (when set), the animal can
// reach on its layer (doors and gates shut for unled livestock: rope.go
// gatesOpen; never behind a fence) and it has not just given up on
// (forage.Memory):
//   - items its diet takes (animal.FeedValue: kibble, straw, raw food,
//     pemmican; meals as the last resort), not forbidden, rotten or held
//     by anyone else; carcasses (meat eaters); stockpiled ones too;
//   - grass, and browse (berry bushes too) or living trees, never a plant
//     in a growing zone or marked for harvest;
//   - for a hungry, grown, unhurt tame predator, a wild animal to hunt
//     (tamePrey) within its hunt radius.
//
// It draws no random numbers and reads the food and creature indexes,
// never whole lists.
func (g *Game) findTameFeed(id entity.EntityID, a *animal.State, pos Position, diet animal.Diet, allow func(x, y int) bool, far int, now uint64) forage.Best {
	r := animal.FeedRadius
	if g.wl.Forage.Searching(id, a.Food) {
		r = max(r, far)
	}
	l, marks := region.Wild, g.wl.Forage.Marks(id)
	if g.gatesOpen(id, a) {
		l = region.Tame
	}
	reach := forage.Reach(g.regionsNow(), l, pos.X, pos.Y, g.grid.Width(), g.grid.Height(), g.fences().cells)
	ok := func(x, y int) bool {
		return (allow == nil || allow(x, y)) && (reach == nil || reach(x, y)) && !forage.Skips(marks, x, y, now)
	}
	need := max(1, animal.Full-a.Food)
	meal := func(v int) int { return min(g.portion(a, now, animal.Meal(a.Species, v)), need) }
	var b forage.Pick
	fx := g.food()
	if diet != animal.NoDiet {
		fx.items.each(pos.X, pos.Y, r, func(iid entity.EntityID) bool {
			st, have := g.stacks.Get(iid)
			if !have || st.Carrier != 0 || st.Qty <= 0 || st.Forbidden || weapon.Distance(pos.X, pos.Y, st.X, st.Y) > r || !ok(st.X, st.Y) {
				return true
			}
			if o, taken := g.reserve.EntityOwner(iid); taken && o != id {
				return true
			}
			d, t := weapon.Distance(pos.X, pos.Y, st.X, st.Y), animal.FeedTarget{ID: iid, X: st.X, Y: st.Y}
			switch {
			case st.Kind == item.CorpseAnimal && diet.Meat() && animal.Species(st.Animal) != a.Species && !animal.IsMech(animal.Species(st.Animal)):
				t.Kind = animal.FeedCarcass
				b.Offer(animal.TierFeed, t, 0, d, meal(min(animal.CarcassFood(animal.Species(st.Animal)), animal.Full)), forage.RankCarcass)
			case !st.Rotten() && animal.FeedValue(diet, st.Kind) > 0:
				t.Kind = animal.FeedItem
				per := g.portion(a, now, animal.Meal(a.Species, animal.FeedValue(diet, st.Kind)))
				b.Offer(animal.FoodTier(st.Kind), t, 0, d, min(per*st.Qty, need), forage.RankItem)
			}
			return true
		})
	}
	if diet.Plants() {
		if x, y, found := g.nearestGrass(pos, b.Within(r), ok); found {
			b.Offer(animal.TierFeed, animal.FeedTarget{Kind: animal.FeedGrass, X: x, Y: y}, 0, weapon.Distance(pos.X, pos.Y, x, y),
				meal(plant.GrassMeal(animal.GrazeFood, g.grassGrowthAt(x, y))), forage.RankGrass)
		}
	}
	if diet.Plants() || diet == animal.Dendrovore {
		pr := b.Within(r)
		fx.plants.each(pos.X, pos.Y, pr, func(pid entity.EntityID) bool {
			p, have := g.plants.Get(pid)
			if !have || p.Zone || p.Designated || weapon.Distance(pos.X, pos.Y, p.X, p.Y) > pr || !ok(p.X, p.Y) {
				return true
			}
			pd, t := plant.DefOf(p.Species), animal.FeedTarget{ID: pid, X: p.X, Y: p.Y}
			switch {
			case diet == animal.Dendrovore:
				if pd.Trunk && p.Percent() >= 50 {
					t.Kind = animal.FeedTree
					b.Offer(animal.TierFeed, t, 0, weapon.Distance(pos.X, pos.Y, p.X, p.Y), meal(animal.TreeFood), forage.RankPlant)
				}
			case pd.Browse && p.Percent() >= 30:
				if _, zoned := g.zoneAt(p.X, p.Y); !zoned { // food.go: a growing zone's cell
					t.Kind = animal.FeedPlant
					b.Offer(animal.TierFeed, t, 0, weapon.Distance(pos.X, pos.Y, p.X, p.Y), meal(animal.BrowseFood), forage.RankPlant)
				}
			}
			return true
		})
	}
	def := animal.DefOf(a.Species)
	if def.Predator && a.Food < animal.FeedStart && !badlyHurt(*a) && now >= a.FleeUntil && g.grown(*a, now) &&
		(a.Master == 0 || !g.drafted(entity.EntityID(a.Master))) { // not while its master is drafted
		hr := b.Within(min(r, huntRadius(def)))
		prey, pp, found := g.nearestWhere(pos.X, pos.Y, hr, ckAnimal, func(o entity.EntityID, p Position) bool {
			return ok(p.X, p.Y) && g.tamePrey(id, o, now) && g.reachesFoe(true, pos, p)
		})
		if found {
			ps := g.animals.Ref(prey).Species
			b.Offer(animal.TierFeed, animal.FeedTarget{}, prey, weapon.Distance(pos.X, pos.Y, pp.X, pp.Y),
				meal(min(animal.CarcassFood(ps), animal.Full)), forage.RankPrey)
		}
	}
	return b.Best()
}

// tamePrey: an animal a tame predator may hunt: a wild one it could take
// (huntable: never a predator, hive insect or legendary, nor bigger than
// itself), not asleep in a vault, marked for taming or enraged. Colonists,
// visitors, prisoners and every tame or penned animal are never prey.
func (g *Game) tamePrey(self, o entity.EntityID, now uint64) bool {
	p := g.animals.Ref(o)
	if p == nil || p.Tame || p.Dormant || p.TameMark || p.Enraged(now) {
		return false
	}
	return g.huntable(self, o)
}

// tameHuntOn reports a tame hunter may keep after target at tp: still
// wild prey, inside its allowed area (fightCheck gives the hunt up else).
func (g *Game) tameHuntOn(id, target entity.EntityID, tp Position) bool {
	return g.tamePrey(id, target, uint64(g.clock.Now())) && g.inArea(id, tp.X, tp.Y)
}

// syncFeedClaim keeps a tame animal's reservation on the stack it walks
// to and eats (and none other): taken when it picks one, released when
// the meal ends, changes or is dropped by anything (phaseAnimals runs it
// every tick for tame animals and for any animal still holding one).
func (g *Game) syncFeedClaim(id entity.EntityID, a *animal.State) {
	var want entity.EntityID
	if a.Tame && a.Mode == animal.Feed && (a.Feed.Kind == animal.FeedItem || a.Feed.Kind == animal.FeedCarcass) {
		want = a.Feed.ID
	}
	if a.FeedClaim == want {
		return
	}
	if a.FeedClaim != 0 {
		g.reserve.ReleaseEntity(id, a.FeedClaim)
	}
	a.FeedClaim = 0
	if want != 0 && g.reserve.TryEntity(id, want) {
		a.FeedClaim = want
	}
}

// tameMayEat re-checks a tame animal's stack each step (PLAY-017): not
// forbidden, not rotten (raw food), and held by no one else.
func (g *Game) tameMayEat(a *animal.State, t animal.FeedTarget, st item.Stack) bool {
	if st.Forbidden || t.Kind == animal.FeedItem && st.Rotten() {
		return false
	}
	_, taken := g.reserve.EntityOwner(t.ID)
	return !taken || a.FeedClaim == t.ID
}

// tameFeedState is what a tame animal's view says it is eating:
// "searching far for food" (starving, not there yet: FEED-001), "eating
// a meal (no feed in reach)", "eating kibble", "eating hay", "eating
// berries", "eating from the stockpile", or its mode.
func (g *Game) tameFeedState(id entity.EntityID, a animal.State) string {
	t := a.Feed
	if p, _ := g.positions.Get(id); g.wl.Forage.IsFar(id) && weapon.Distance(p.X, p.Y, t.X, t.Y) > feedReach(t) {
		return "searching far for food"
	}
	switch t.Kind {
	case animal.FeedItem, animal.FeedCarcass:
		st, ok := g.stacks.Get(t.ID)
		switch {
		case ok && t.Kind == animal.FeedItem && animal.FoodTier(st.Kind) == animal.TierMeal:
			return "eating a meal (no feed in reach)"
		case ok && (st.Kind == item.Kibble || st.Kind == item.Straw || st.Kind == item.Berries):
			return "eating " + map[item.Kind]string{item.Kibble: "kibble", item.Straw: "hay", item.Berries: "berries"}[st.Kind]
		case g.stockpiled(t.X, t.Y):
			return "eating from the stockpile"
		}
	case animal.FeedPlant:
		if p, ok := g.plants.Get(t.ID); ok && plant.DefOf(p.Species).Yields == item.Berries {
			return "eating berries"
		}
	}
	return a.Mode.String()
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
	cells, w, h := g.seedGrass().cells, g.grid.Width(), g.grid.Height() // hasGrass, unrolled: a starving search reads the whole map
	try := func(x, y int) bool {
		return uint(x) < uint(w) && uint(y) < uint(h) && cells[y*w+x] == 1 && g.grid.Walkable(x, y) && ok(x, y)
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
		return ok && st.Carrier == 0 && st.Qty > 0 && st.X == t.X && st.Y == t.Y && (!a.Tame || g.tameMayEat(a, t, st))
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
		case now > a.NextFeed: // can't get there: a tame one skips the spot a while (FEED-001)
			if a.Tame {
				g.wl.Forage.GaveUp(id, t.X, t.Y, now)
			}
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
	g.wl.Forage.Ate(id) // a far search ends with a meal (FEED-001)
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
			g.setPlant(t.ID, p)
		}
		if !a.Tame {
			g.wl.CropRaids = append(g.wl.CropRaids, cropRaid{X: t.X, Y: t.Y, Tick: now, Species: a.Species})
		}
		return animal.CropFood
	}
	if d.Regrows {
		p.Growth = d.GrowTicks * d.RegrowTo / 100
		g.setPlant(t.ID, p)
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
	g.putStack(id, st)
}

// fillAnimalNeeds fills the needs and life fields of an animal view.
func (g *Game) fillAnimalNeeds(av *AnimalView, a animal.State) {
	now := uint64(g.clock.Now())
	av.State = a.Mode.String()
	switch {
	case a.Downed:
		av.State = "downed" // animal_health.go
	case a.Mode == animal.Attack && a.HuntSince != 0:
		av.State = "hunting"
		if p := g.animals.Ref(a.Target); p != nil {
			av.State = "hunting " + withArticle(p.Species.Label()) // "hunting a hare" (chase.go)
		}
	case a.Tame && a.Mode == animal.Feed:
		av.State = g.tameFeedState(av.ID, a) // "eating kibble", "eating from the stockpile"
	case a.Tame && g.wl.Forage.IsFar(av.ID):
		av.State = "searching far for food" // starving, nothing found yet (FEED-001)
	}
	av.Food, av.Rest = a.Food*100/animal.Full, a.Rest*100/animal.Full
	if !a.HasSex || animal.NeedsFree(a.Species) {
		av.Food, av.Rest = 100, 100
	}
	av.Diet = speciesViewOf(a.Species).diet // animal_view_species.go
	av.Hunts = a.Tame && animal.DefOf(a.Species).Predator
	av.Leaving = a.Leaving || a.Mode == animal.Leave
	av.Herd = a.Herd
	days := animal.AgeDays(a.Born, now, a.Species)
	av.AgeDays, av.Adult = days, animal.Adult(a.Species, days)
	av.Stage, av.Scale = animal.Stage(a.Species, av.Adult), animal.GrowthScale(a.Species, days)
	g.fillAnimalLife(av, a, now) // animal_life.go
}
