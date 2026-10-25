package game

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal life (SIM-026, plan 4.1-4.3): growing up, old age, mating,
// birth, eggs and hatching. Everything runs inside phaseAnimals, from
// fields on animal.State and item.Stack, with no state of its own:
//
//   - Young animals (younger than AdultDays) don't mate, produce, lay or
//     hunt, and eat half portions: a meal fills them animal.YoungMeal
//     times as far (eat, animal_needs.go). A young one follows its mother
//     (the nearest grown female of its species, tameness and herd),
//     wandering within animal.FollowNear tiles of her.
//   - Old age: from OldAgeFrom percent of its lifespan an animal rolls an
//     OldAgeChance at dawn, once a day.
//   - Mating: each grown male checks once a day, at its own awake tick
//     (animal.MateTick: no state, and never twice a day), for a grown
//     female within MateRadius; both fed (MateFood), wandering, not
//     leaving, she neither pregnant nor carrying eggs, both wild or both
//     tame, and wild pairs only while the map is under its carrying
//     capacity. A MateChance roll; on success he walks to her while she
//     waits, MateTicks beside her, then she is pregnant (live bearers) or
//     carries MatedEggsOf eggs (layers). A mating not done by MateUntil,
//     or either partner leaving Wander, is called off.
//   - Birth: at the due tick the litter is born on free tiles beside her:
//     young, random sex, her herd and her tameness (named when tame).
//     A pregnancy dies with its mother.
//   - Eggs: a layer lays at its own tick of the day (animal.LayTick) when
//     fed (LayFood): a fertile egg while mated (every EggDays), and tame
//     chickens, ducks and geese a plain egg every day besides (every
//     third egg of a mating fertile). Wild plain layers lay only their
//     fertile eggs, so the wild map is not strewn with eggs. Tame layers
//     lay in their pen beside its trough when they are in one.
//   - Hatching: every climate interval each fertile egg stack on the
//     ground incubates while 0 C <= air <= 45 C; below -5 C it dies (a
//     plain egg). At HatchTicks the whole stack hatches at once.

// lifeEvery: a product fills in steps of this many ticks (staggered by
// id), so the per-tick cost of life is a few compares.
const lifeEvery = 250

// tickLife is one animal's life for the tick (phaseAnimals, after its
// needs). It reports false when the animal died of old age.
func (g *Game) tickLife(id entity.EntityID, a *animal.State, now uint64) bool {
	if a.Mate != 0 && a.Mode != animal.Wander {
		g.endMating(id, a) // she lay down, he went to eat, either fled
	}
	if a.Pregnant != 0 && now >= a.Pregnant {
		g.giveBirth(id, a, now)
	}
	tod := now % clock.TicksPerDay
	if tod == dawnHour*clock.TicksPerHour && animal.Breeds(a.Species) && g.oldAge(id, a, now) {
		return false
	}
	fill := (now+uint64(id))%lifeEvery == 0
	var daily bool
	switch {
	case !a.Female:
		daily = tod == animal.MateTick(a.Species, uint64(id))
	case animal.Layer(a.Species):
		daily = tod == animal.LayTick(a.Species, uint64(id))
	}
	if !fill && !daily || !animal.Breeds(a.Species) || !g.grown(*a, now) {
		return true
	}
	if fill && a.Product < animal.ProductFull && animal.Produces(a.Species, a.Female, true) {
		per := animal.ProductPerDay(a.Species, a.Food)
		a.Product = min(animal.ProductFull, a.Product+rateSpan(now, per, clock.TicksPerDay, lifeEvery))
	}
	switch {
	case daily && a.Female:
		g.layEgg(id, a, now)
	case daily:
		g.mateCheck(id, a, now)
	}
	return true
}

// rateSpan is the share of per units over period ticks that falls in the
// span ticks up to now (rateStep over a span).
func rateSpan(now uint64, per, period, span int) int {
	from := uint64(0)
	if now > uint64(span) {
		from = now - uint64(span)
	}
	p, q := uint64(per), uint64(period)
	return int(now*p/q - from*p/q)
}

// lifePhase is the phase-level part of life (phaseAnimals, once a tick):
// fertile eggs incubate every climate interval.
func (g *Game) lifePhase(now uint64) {
	if now%climateInterval == 0 {
		g.incubate()
	}
}

// grown reports a grown animal.
func (g *Game) grown(a animal.State, now uint64) bool {
	return animal.Adult(a.Species, animal.AgeDays(a.Born, now, a.Species))
}

// portion is a meal of v food to a: young animals eat half portions.
func (g *Game) portion(a *animal.State, now uint64, v int) int {
	if g.grown(*a, now) {
		return v
	}
	return v * animal.YoungMeal
}

// --- old age --------------------------------------------------------------

// oldAge rolls the day's old-age death for an old animal; it reports true
// when it died.
func (g *Game) oldAge(id entity.EntityID, a *animal.State, now uint64) bool {
	p := animal.OldAgeChance(a.Species, animal.AgeDays(a.Born, now, a.Species))
	if p <= 0 || g.rng.IntN(1000) >= p {
		return false
	}
	pos, _ := g.positions.Get(id)
	if a.Tame {
		g.message(pos.X, pos.Y, "%s the %s died of old age.", a.Name, a.Species.Label())
	}
	g.logEvent(id, logEntry{Kind: logOldAge})
	g.endMating(id, a)
	g.removeAnimal(id)
	g.dropCorpse(a.Species, pos.X, pos.Y)
	if animal.DefOf(a.Species).Explosive {
		g.burst(a.Species, pos)
	}
	return true
}

// --- mating ---------------------------------------------------------------

// mateCheck is a grown male's daily look for a mate (the MateChance roll
// only when there is one).
func (g *Game) mateCheck(id entity.EntityID, a *animal.State, now uint64) {
	if a.Mode != animal.Wander || a.Leaving || a.Mate != 0 || a.Food < animal.MateFood || a.Enraged(now) || g.beingTamed(id) {
		return
	}
	if !a.Tame && g.wildCount() >= g.wildlifeCap() { // population.go
		return
	}
	pos, _ := g.positions.Get(id)
	fid, _, ok := g.nearestWhere(pos.X, pos.Y, animal.MateRadius, ckAnimal, func(o entity.EntityID, _ Position) bool {
		f, ok := g.animals.Get(o)
		return ok && o != id && g.mateable(f, *a, now) && !g.beingTamed(o)
	})
	if !ok || g.rng.IntN(100) >= animal.MateChance {
		return
	}
	f, _ := g.animals.Get(fid)
	until := now + animal.MateWalkLimit + animal.MateTicks
	a.Mate, a.MateLeft, a.MateUntil = fid, animal.MateTicks, until
	a.Path, a.PathI, a.NextThink = nil, 0, now
	f.Mate, f.MateLeft, f.MateUntil = id, animal.MateTicks, until
	f.Path, f.PathI = nil, 0
	g.animals.Set(fid, f)
}

// mateable: f is a female male m may mate with.
func (g *Game) mateable(f, m animal.State, now uint64) bool {
	return f.Species == m.Species && f.HasSex && f.Female && f.Tame == m.Tame && g.grown(f, now) &&
		f.Food >= animal.MateFood && f.Mode == animal.Wander && !f.Leaving && f.Mate == 0 &&
		f.Pregnant == 0 && f.Mated == 0 && !f.Enraged(now)
}

// lifeMove is a wandering animal's life movement (thinkAnimal): a mating
// under way, or a young one following its mother. It reports true when it
// moved (or held) the animal this tick.
func (g *Game) lifeMove(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	if a.Mate != 0 {
		return g.mateStep(id, a, pos, now)
	}
	return g.followMother(id, a, pos, now)
}

// mateStep: she holds still; he walks to her, then mates for MateTicks.
func (g *Game) mateStep(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	p, ok := g.animals.Get(a.Mate)
	if !ok || p.Mate != id || p.Mode != animal.Wander || now > a.MateUntil {
		g.endMating(id, a)
		return false
	}
	if a.Female {
		a.Path, a.PathI = nil, 0
		return true
	}
	pp, _ := g.positions.Get(a.Mate)
	if weapon.Distance(pos.X, pos.Y, pp.X, pp.Y) > 1 {
		if a.PathI >= len(a.Path) && now >= a.RetryAt {
			g.animalGoTo(id, a, pp.X, pp.Y)
		}
		return true
	}
	a.Path, a.PathI = nil, 0
	if a.MateLeft--; a.MateLeft > 0 {
		return true
	}
	fid := a.Mate
	if animal.Layer(p.Species) {
		p.Mated = animal.MatedEggsOf(p.Species)
	} else {
		p.Pregnant = now + animal.GestTicks(p.Species)
	}
	a.Mate, a.MateLeft, a.MateUntil = 0, 0, 0
	p.Mate, p.MateLeft, p.MateUntil = 0, 0, 0
	g.animals.Set(fid, p)
	g.logEvent(id, logEntry{Kind: logMated, Other: fid, Name: animalLabel(p)})
	g.logEvent(fid, logEntry{Kind: logMated, Other: id, Name: animalLabel(*a)})
	return true
}

// endMating calls a's mating off on both sides.
func (g *Game) endMating(id entity.EntityID, a *animal.State) {
	if a.Mate == 0 {
		return
	}
	if p, ok := g.animals.Get(a.Mate); ok && p.Mate == id {
		p.Mate, p.MateLeft, p.MateUntil = 0, 0, 0
		g.animals.Set(a.Mate, p)
	}
	a.Mate, a.MateLeft, a.MateUntil = 0, 0, 0
}

// --- the young --------------------------------------------------------------

// mother is the grown female of a's species, tameness and herd nearest
// pos within MotherRadius.
func (g *Game) mother(id entity.EntityID, a animal.State, pos Position, now uint64) (Position, bool) {
	_, mp, ok := g.nearestWhere(pos.X, pos.Y, animal.MotherRadius, ckAnimal, func(o entity.EntityID, _ Position) bool {
		m, ok := g.animals.Get(o)
		return ok && o != id && m.Species == a.Species && m.Female && m.Tame == a.Tame &&
			(a.Herd == 0 || m.Herd == a.Herd) && g.grown(m, now)
	})
	return mp, ok
}

// followMother wanders a young animal near its mother; false when it is
// grown, still walking, not due to think, or has no mother near.
func (g *Game) followMother(id entity.EntityID, a *animal.State, pos Position, now uint64) bool {
	if a.PathI < len(a.Path) || now < a.NextThink || g.grown(*a, now) {
		return false
	}
	mp, ok := g.mother(id, *a, pos, now)
	if !ok {
		return false
	}
	a.NextThink = now + 120 + uint64(g.rng.IntN(240))
	r := animal.FollowNear
	if weapon.Distance(pos.X, pos.Y, mp.X, mp.Y) > r {
		r = 1 // catch up
	}
	g.animalGoTo(id, a, mp.X+g.rng.IntN(2*r+1)-r, mp.Y+g.rng.IntN(2*r+1)-r)
	return true
}

// giveBirth: a pregnant animal's litter is born beside her.
func (g *Game) giveBirth(id entity.EntityID, a *animal.State, now uint64) {
	a.Pregnant = 0
	life := animal.LifeOf(a.Species)
	n := life.LitterMin + g.rng.IntN(max(life.LitterMax-life.LitterMin, 0)+1)
	pos, _ := g.positions.Get(id)
	born := 0
	for i := 0; i < n; i++ {
		x, y, ok := g.birthSpot(pos.X, pos.Y)
		if !ok {
			break
		}
		cid, ok := g.spawnYoung(a.Species, x, y, a.Tame, a.Herd, now)
		if !ok {
			break
		}
		if c, ok := g.animals.Get(cid); ok {
			c.Mother = uint64(id)
			g.animals.Set(cid, c)
		}
		g.logEvent(cid, logEntry{Kind: logBorn, Other: id, Name: animalLabel(*a)})
		born++
	}
	if born == 0 {
		return
	}
	g.logEvent(id, logEntry{Kind: logGaveBirth, Amount: born, Name: youngName(a.Species, born)})
	if a.Tame {
		g.message(pos.X, pos.Y, "%s gave birth to %s.", a.Name, youngPhrase(a.Species, born))
	}
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
	return x, y, g.grid.Walkable(x, y)
}

// youngName is n young of s for a log line ("2 goat kids").
func youngName(s animal.Species, n int) string {
	w := animal.YoungWord(s)
	if n != 1 {
		w = pluralYoung(w)
	}
	return s.Label() + " " + w
}

// youngPhrase is n young of s in a sentence ("a goat kid", "2 goat
// kids").
func youngPhrase(s animal.Species, n int) string {
	if n != 1 {
		return fmt.Sprintf("%d %s", n, youngName(s, n))
	}
	if strings.ContainsRune("aeiou", rune(s.Label()[0])) {
		return "an " + youngName(s, 1)
	}
	return "a " + youngName(s, 1)
}

func pluralYoung(w string) string {
	switch {
	case w == "young":
		return w
	case strings.HasSuffix(w, "calf"):
		return w[:len(w)-1] + "ves"
	}
	return w + "s"
}

// --- eggs -------------------------------------------------------------------

// layEgg: a grown female layer's egg of the day, if she lays today.
func (g *Game) layEgg(id entity.EntityID, a *animal.State, now uint64) {
	day := int(now / clock.TicksPerDay)
	if a.Food < animal.LayFood || a.Leaving || !animal.LaysToday(a.Species, uint64(id), day) {
		return
	}
	fertile := false
	if a.Mated > 0 {
		fertile = animal.FertileEgg(a.Species, a.Mated)
		a.Mated--
	}
	plain := animal.EcologyOf(a.Species).Eggs == animal.PlainEggs && a.Tame
	if !fertile && !plain {
		return
	}
	pos, _ := g.positions.Get(id)
	x, y := pos.X, pos.Y
	if a.Tame {
		if m, _ := g.penAt(x, y); m != 0 { // pens.go
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
	st, _ := g.stacks.Get(sid)
	if fertile {
		st.Animal, st.Tame = uint8(a.Species), a.Tame
	}
	g.mergeOrSet(sid, st) // onto the eggs already there
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

// incubate advances every fertile egg stack on the ground by a climate
// interval: warm eggs incubate, frozen ones die, ripe ones hatch.
func (g *Game) incubate() {
	var eggs []entity.EntityID
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.FertileEgg && st.Carrier == 0 {
			eggs = append(eggs, id)
		}
	}
	for _, id := range eggs {
		st, ok := g.stacks.Get(id)
		if !ok || st.Kind != item.FertileEgg {
			continue // merged away
		}
		t := int(g.temperatureAt(st.X, st.Y))
		switch {
		case t < animal.EggFreeze:
			st.Kind, st.Animal, st.Tame, st.Hatch = item.Egg, 0, false, 0
			g.mergeOrSet(id, st)
			continue
		case t < animal.HatchMin || t > animal.HatchMax:
			continue
		}
		st.Hatch += climateInterval
		if st.Hatch < animal.HatchTicks(animal.Species(st.Animal)) {
			g.stacks.Set(id, st)
			continue
		}
		g.hatch(id, st)
	}
}

// hatch: the whole stack of fertile eggs hatches where it lies.
func (g *Game) hatch(id entity.EntityID, st item.Stack) {
	s := animal.Species(st.Animal)
	now := uint64(g.clock.Now())
	g.deleteItem(id)
	herd := uint32(0)
	if !st.Tame {
		herd = g.wildHerdNear(s, st.X, st.Y, now)
	}
	n := 0
	for i := 0; i < st.Qty; i++ {
		x, y, ok := g.birthSpot(st.X, st.Y)
		if !ok {
			break
		}
		cid, ok := g.spawnYoung(s, x, y, st.Tame, herd, now)
		if !ok {
			break
		}
		g.logEvent(cid, logEntry{Kind: logHatched})
		n++
	}
	if n == 0 || !st.Tame {
		return
	}
	if n == 1 {
		g.message(st.X, st.Y, "%s hatched.", capitalise(youngPhrase(s, 1)))
		return
	}
	g.message(st.X, st.Y, "%s hatched.", youngPhrase(s, n))
}

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

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// fillAnimalLife fills the breeding and product fields of an animal view.
func (g *Game) fillAnimalLife(av *AnimalView, a animal.State, now uint64) {
	if a.Pregnant != 0 {
		gest := animal.GestTicks(a.Species)
		av.Pregnant = true
		if gest > 0 && a.Pregnant > now {
			av.PregPct = clampInt(int(100-(a.Pregnant-now)*100/gest), 0, 100)
		} else {
			av.PregPct = 100
		}
	}
	av.EggsFertile = animal.Layer(a.Species) && a.Mated > 0
	if animal.Produces(a.Species, a.Female, av.Adult) {
		av.Product = animal.EcologyOf(a.Species).Product.String()
		av.ProductPct = a.Product * 100 / animal.ProductFull
	}
}
