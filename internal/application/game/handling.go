package game

import (
	"slices"

	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
	"rimworld-go/pkg/msgfmt"
)

// Animal handling. Hunters also tame: an animal marked for taming holds
// still while a handler coaxes it, then is tamed (or not; a botched try
// can enrage it). Tame animals have names; livestock live in pens (pens.go)
// and the rest around the colony; they pass through the colony's doors and
// never spring its traps; tame ridgewolves guard the colony against
// raiders and hostile animals. Wild animals hurt by colonists may go
// manhunter, hunting every colonist for a day; the storyteller also sends
// whole manhunter packs. Rage, guards, hunts, charges, flight and chase
// warnings live in game/rage, reached through rageOf (end of this file).

const (
	// pathRetry: after a failed path search an animal waits this long
	// before searching again.
	pathRetry = 90
)

var petNames = []string{"Biscuit", "Pepper", "Juniper", "Ash", "Clover", "Moss", "Tansy", "Rook", "Sorrel",
	"Bramble", "Fennel", "Hazel", "Ember", "Thistle", "Burr", "Wren"}

func (g *Game) markTame(x1, y1, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x1, y1, x2, y2)
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok || on && (a.Tame || animal.DefOf(a.Species).TameChance == 0) {
			continue // hive insects are never tamed
		}
		if p, _ := g.positions.Get(id); p.X >= minX && p.X <= maxX && p.Y >= minY && p.Y <= maxY && !g.fogged(p.X, p.Y) {
			a.TameMark = on
			g.animals.Set(id, a)
		}
	}
}

// beingTamed: a handler has claimed this animal for a taming, milking,
// shearing, training, slaughter or feeding job, or a doctor for tending it.
func (g *Game) beingTamed(id entity.EntityID) bool {
	owner, ok := g.reserve.EntityOwner(id)
	if !ok {
		return false
	}
	j, _ := g.jobs.Get(owner)
	switch j.Kind {
	case job.Tame, job.Milk, job.Shear, job.Train, job.TendAnimal, job.Rope:
		return j.Target == id
	}
	return herd.Holds(j, id) // FOOD-3: a slaughter or a feeding holds it too
}

// herdOf is the herd (game/herd, FOOD-3: taming, milking and shearing,
// slaughter, auto-slaughter, feeding downed animals, hunters' kills),
// built on first use over the settings saved in the wildlife state (a
// load replaces both).
func (g *Game) herdOf() *herd.Herd {
	if g.wl.herd == nil {
		g.wl.herd = herd.New(herdHost{g}, &g.wl.Herd)
	}
	return g.wl.herd
}

// herdHost implements herd.Host over the game.
type herdHost struct{ g *Game }

func (h herdHost) Now() uint64                                  { return uint64(h.g.clock.Now()) }
func (h herdHost) Roll(n int) int                               { return h.g.rng.IntN(n) }
func (h herdHost) Message(x, y int, format string, args ...any) { h.g.message(x, y, format, args...) }
func (h herdHost) Name(id entity.EntityID) string               { return h.g.name(id) }
func (h herdHost) Animals(id entity.EntityID) int               { return int(h.g.skillOf(id, skill.Animals).Level) }
func (h herdHost) AnimalIDs() []entity.EntityID                 { return h.g.animalIDs }
func (h herdHost) Animal(id entity.EntityID) (animal.State, bool) {
	return h.g.animals.Get(id)
}
func (h herdHost) SetAnimal(id entity.EntityID, a animal.State) { h.g.animals.Set(id, a) }
func (h herdHost) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := h.g.positions.Get(id)
	return p.X, p.Y, ok
}
func (h herdHost) AnyAnimal(keep func(entity.EntityID, *animal.State) bool) bool {
	return h.g.anyAnimal(keep) // jobindex.go
}
func (h herdHost) Nearest(id entity.EntityID, keep func(entity.EntityID, *animal.State) bool) (entity.EntityID, int, int, bool) {
	g := h.g
	pos, _ := g.positions.Get(id)
	aid, p, ok := g.nearestWhere(pos.X, pos.Y, max(g.grid.Width(), g.grid.Height()), ckAnimal, func(o entity.EntityID, _ Position) bool {
		a := g.animals.Ref(o)
		return a != nil && keep(o, a)
	})
	return aid, p.X, p.Y, ok
}
func (h herdHost) Kill(aid, by entity.EntityID)                     { h.g.killAnimal(aid, by) }
func (h herdHost) StartJob(id entity.EntityID, rec job.Record) bool { return h.g.startJob(id, rec) }
func (h herdHost) SetJob(id entity.EntityID, j job.Record)          { h.g.jobs.Set(id, j) }
func (h herdHost) Job(id entity.EntityID) (job.Record, bool)        { return h.g.jobs.Get(id) }
func (h herdHost) Owner(t entity.EntityID) (entity.EntityID, bool)  { return h.g.reserve.EntityOwner(t) }
func (h herdHost) Claim(id, t entity.EntityID) bool                 { return h.g.reserve.TryEntity(id, t) }
func (h herdHost) BesideSpot(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.besideSpot(id, x, y)
}
func (h herdHost) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	return h.g.pathTo(id, x, y)
}
func (h herdHost) Work(id entity.EntityID) int                  { return h.g.workProgress(id, skAnimals, true) }
func (h herdHost) Stack(sid entity.EntityID) (item.Stack, bool) { return h.g.stacks.Get(sid) }
func (h herdHost) Delete(sid entity.EntityID)                   { h.g.deleteItem(sid) }
func (h herdHost) HaulToStorage(id, sid entity.EntityID) bool {
	return h.g.haulToStockpile(id, h.pos(id), []entity.EntityID{sid}, 0)
}
func (h herdHost) pos(id entity.EntityID) Position   { p, _ := h.g.positions.Get(id); return p }
func (h herdHost) FedAnimal(id, aid entity.EntityID) { h.g.logPair(aid, id, logFedBy, logFed, 0, 0) }
func (h herdHost) Policy(aid entity.EntityID) func(item.Kind) bool {
	return h.g.sk.diet.Menu(aid, true).Has
}

// NearestFood is the nearest free, fresh food stack accept takes for id.
func (h herdHost) NearestFood(id entity.EntityID, accept func(item.Kind) bool) (entity.EntityID, int, int) {
	g := h.g
	return g.nearestFree(id, func(k item.Kind) bool { return k.IsFood() && accept(k) }) // orders.go
}

// Take: id, at stack sid, picks up qty of it, split off a bigger stack.
func (h herdHost) Take(id, sid entity.EntityID, qty int) (entity.EntityID, bool) {
	g := h.g
	st, ok := g.stacks.Get(sid)
	if !ok || st.Carrier != 0 || st.Qty <= 0 || st.Rotten() {
		return 0, false
	}
	if st.Qty <= qty {
		st.Carrier = id
		g.putStack(sid, st)
		return sid, true
	}
	st.Qty -= qty
	g.putStack(sid, st)
	carry := g.entities.Next()
	g.itemIDs = append(g.itemIDs, carry)
	st.Qty, st.Carrier = qty, id
	g.putStack(carry, st)
	return carry, true
}

// Drop spawns st at its tile, onto any stack it merges with.
func (h herdHost) Drop(st item.Stack) {
	g := h.g
	if sid, err := g.SpawnStack(st.Kind, st.Qty, st.X, st.Y); err == nil {
		s, _ := g.stacks.Get(sid)
		s.Animal = st.Animal
		g.mergeOrSet(sid, s)
	}
}

// Tamed: handler id tamed animal aid: it joins the colony, named, and may
// bond with its tamer.
func (h herdHost) Tamed(id, aid entity.EntityID) {
	g := h.g
	a, _ := g.animals.Get(aid)
	ap, _ := g.positions.Get(aid)
	a.Tame, a.TameMark, a.Designated = true, false, false
	a.Name = petNames[g.rng.IntN(len(petNames))]
	a.Mode, a.Target, a.Attacker, a.Path, a.PathI = animal.Wander, 0, 0, nil, 0
	g.message(ap.X, ap.Y, "%s tamed a %s. Its name is %s.", g.name(id), a.Species.Label(), a.Name)
	g.logEvent(aid, logEntry{Kind: evlog.TamedBy, Other: id, Name: g.name(id)})
	g.logEvent(id, logEntry{Kind: evlog.Tamed, Other: aid, Name: a.Name + " the " + a.Species.Label()})
	g.animals.Set(aid, a)
	g.onTamed(id, aid) // bonds.go: a chance of a bond with the tamer
}

// TameFailed: id failed to tame aid, which waits out its retry and may
// go manhunter at its species' wiki chance (story.go: the difficulty).
func (h herdHost) TameFailed(id, aid entity.EntityID) {
	g := h.g
	a, _ := g.animals.Get(aid)
	ap, _ := g.positions.Get(aid)
	now := uint64(g.clock.Now())
	a.TameRetry = now + animal.TameRetryTicks
	g.logEvent(aid, logEntry{Kind: evlog.ResistedTaming, Other: id, Name: g.name(id)})
	g.logEvent(id, logEntry{Kind: evlog.FailedTame, Other: aid, Name: a.Species.Label()})
	if g.rng.IntN(100) < g.revengePct(animal.DefOf(a.Species).TameRevenge) {
		rageOf(g).Enrage(aid, &a, id, now)
	} else {
		g.message(ap.X, ap.Y, "%s failed to tame the %s.", g.name(id), a.Species.Label())
	}
	g.animals.Set(aid, a)
}

// wanderTame: livestock keep to their pen, or roam when they have none
// (pens.go); other tame animals stay near the colony, inside their
// allowed area (areas.go inArea).
func (g *Game) wanderTame(id entity.EntityID, a *animal.State, pos Position, def animal.Def, now uint64) {
	a.Target = 0
	if a.PathI < len(a.Path) || now < a.NextThink {
		return
	}
	a.NextThink = now + 240 + uint64(g.rng.IntN(480))
	if penned(a) {
		r := penRoamRadius
		_, c := g.homePen(id, a, pos)
		if c != nil {
			r = def.WanderRadius
		}
		x, y := pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r
		x, y = mathx.Clamp(x, 0, g.grid.Width()-1), mathx.Clamp(y, 0, g.grid.Height()-1)
		if c != nil && !g.inPenCells(c, x, y) { // pens need not be rectangles
			return
		}
		if c == nil {
			// roaming: never towards a wild predator in sight (it would
			// only run again: no flee/roam flapping)
			if pp, near := rageOf(g).NearestPredator(id, rage.Pos(pos), r+def.FleeRadius+animal.FleeRelease); near &&
				rage.DistSq(rage.Pos{X: x, Y: y}, pp) < rage.DistSq(rage.Pos(pos), pp) {
				return
			}
		}
		g.animalGoTo(id, a, x, y)
		return
	}
	// stay near the colony, inside the allowed area
	hx, hy := g.home()
	r := def.WanderRadius
	var x, y int
	if weapon.Distance(pos.X, pos.Y, hx, hy) > 12 {
		x, y = hx+g.rng.IntN(7)-3, hy+g.rng.IntN(7)-3
	} else {
		x, y = pos.X+g.rng.IntN(2*r+1)-r, pos.Y+g.rng.IntN(2*r+1)-r
	}
	if g.inArea(id, x, y) {
		g.animalGoTo(id, a, x, y)
	}
}

// home is roughly where the colony lives: its first standing colonist's
// bed, else the landing spot (the map centre for legacy colonies).
func (g *Game) home() (int, int) {
	for _, id := range g.pawnIDs {
		if bid := g.bedOf(id); bid != 0 {
			b, _ := g.bldgs.Get(bid)
			return b.X, b.Y
		}
	}
	return g.landingSpot() // setup.go
}

// bedOf is the bed a colonist has claimed (0: none): the first sleeper in
// building order, read from the cached bed list (the claimant is live).
func (g *Game) bedOf(id entity.EntityID) entity.EntityID {
	for _, bid := range bedsOf(g) {
		if b := g.bldgs.Ref(bid); b != nil && b.SlotOf(uint64(id)) >= 0 {
			return bid
		}
	}
	return 0
}

// bedsOf is every sleeping building in building order, rebuilt when the
// structure or the building count moves (the index self-check compares it
// with a fresh scan). Callers must not keep it across a building change.
func bedsOf(g *Game) []entity.EntityID {
	c := &g.vents.beds
	if list, ok := c.Get(g.structVer, len(g.bldgIDs), g.bldgs); ok {
		if checkIndex(g) && !slices.Equal(scanBeds(g, nil), list) {
			panic("bedsOf: stale bed list (structVer not bumped?)")
		}
		return list
	}
	return c.Seal(g.structVer, len(g.bldgIDs), g.bldgs, scanBeds(g, c.Reuse()))
}

func scanBeds(g *Game, into []entity.EntityID) []entity.EntityID {
	for _, id := range g.bldgIDs {
		if b := g.bldgs.Ref(id); b != nil && b.Kind.Sleeps() {
			into = append(into, id)
		}
	}
	return into
}

// ManhunterPack is the dev hook for a manhunter pack.
func (g *Game) ManhunterPack(n int) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	rageOf(g).ManhunterPack(n)
}

// releaseAnimal turns a tame animal loose.
func (g *Game) releaseAnimal(id entity.EntityID) {
	a, ok := g.animals.Get(id)
	if !ok || !a.Tame {
		return
	}
	p, _ := g.positions.Get(id)
	g.message(p.X, p.Y, "%s the %s was released into the wild.", a.Name, a.Species.Label())
	g.unbondReleased(id, &a) // bonds.go: the bonded colonist's regret
	a.Tame, a.Name, a.Target, a.Mode = false, "", 0, animal.Wander
	g.animals.Set(id, a)
}

// TameAnimal makes an animal tame on the spot (dev scenarios, fixtures).
func (g *Game) TameAnimal(id entity.EntityID, name string) {
	g.viewGen++ // view cache (indexes.go): this hook changes the state
	if a, ok := g.animals.Get(id); ok {
		a.Tame, a.Name, a.TameMark, a.Manhunter = true, name, false, 0
		g.animals.Set(id, a)
	}
}

// rageOf is the animals' fight-or-flight brain (game/rage: rage, guards,
// hunts, charges, flight and chase warnings), built on first use; it
// keeps no state of its own (a load replaces the wildlife state, and the
// next call builds it again).
func rageOf(g *Game) *rage.Rage {
	if g.wl.rage == nil {
		g.wl.rage = rage.New(rageHost{rageLook{g}, rageAct{g}})
	}
	return g.wl.rage
}

// rageHost implements rage.Host over the game: rageLook the reads, names
// and messages (World, Beasts, People), rageAct the walk, the story and
// the fight (Ground, Story, Fight).
type rageHost struct {
	rageLook
	rageAct
}

type (
	rageLook struct{ g *Game }
	rageAct  struct{ g *Game }
)

func (h rageLook) Now() uint64                                    { return uint64(h.g.clock.Now()) }
func (h rageLook) Roll(n int) int                                 { return h.g.rng.IntN(n) }
func (h rageLook) Name(id entity.EntityID) string                 { return h.g.name(id) }
func (h rageLook) CreatureName(id entity.EntityID) string         { return h.g.creatureName(id) }
func (h rageLook) KillerName(id entity.EntityID) string           { return h.g.killerName(id) }
func (h rageLook) Animal(id entity.EntityID) (animal.State, bool) { return h.g.animals.Get(id) }
func (h rageLook) Ref(id entity.EntityID) *animal.State           { return h.g.animals.Ref(id) }
func (h rageLook) SetAnimal(id entity.EntityID, a animal.State)   { h.g.animals.Set(id, a) }
func (h rageLook) Grown(a animal.State, now uint64) bool          { return h.g.grown(a, now) }
func (h rageLook) Asleep(a animal.State) bool                     { return sleeping(a) }
func (h rageLook) Hostile(a animal.State) bool                    { return h.g.hostile(a) }
func (h rageLook) IsPawn(id entity.EntityID) bool                 { return h.g.isPawn(id) }
func (h rageLook) IsRaider(id entity.EntityID) bool               { return h.g.isRaider(id) }
func (h rageLook) StandingPawn(id entity.EntityID) bool           { return h.g.standingPawn(id) }
func (h rageLook) StandingRaider(id entity.EntityID) bool         { return h.g.standingRaider(id) }
func (h rageLook) Colonist(id entity.EntityID) bool               { return slices.Contains(h.g.pawnIDs, id) }

func (h rageLook) Log(subject entity.EntityID, kind uint8, other entity.EntityID, name string) {
	h.g.logEvent(subject, logEntry{Kind: kind, Other: other, Name: name})
}

func (h rageLook) Alarm(tick uint64, x, y int, focus entity.EntityID, text string, msg msgfmt.Text) {
	h.g.post(MessageView{Tick: tick, Text: text, Msg: msg, X: x, Y: y, Danger: true, Focus: focus})
}

func (h rageLook) Threat(title string, x, y int, format string, args ...any) {
	h.g.threat(title, x, y, format, args...)
}

func (h rageLook) Pos(id entity.EntityID) (rage.Pos, bool) {
	p, ok := h.g.positions.Get(id)
	return rage.Pos(p), ok
}

func (h rageLook) Spawn(s animal.Species, x, y int) (entity.EntityID, error) {
	return h.g.spawnAnimal(s, x, y)
}

func (h rageLook) NearestColonist(p rage.Pos) entity.EntityID {
	id, _, _ := h.g.nearestColonist(Position(p))
	return id
}

// Nearest and Within: rage's kinds are the creature index's bits
// (TestRageKindsAreIndexKinds).
func (h rageAct) Nearest(x, y, r int, k rage.Kind, s rage.Sight) (entity.EntityID, rage.Pos, bool) {
	id, p, ok := h.g.nearestWhere(x, y, r, uint8(k), func(o entity.EntityID, _ Position) bool { return s.Keep(s, o) })
	return id, rage.Pos(p), ok
}

func (h rageAct) Within(x, y, r int, k rage.Kind) []entity.EntityID {
	var ids []entity.EntityID
	h.g.within(x, y, r, uint8(k), func(o entity.EntityID, _ Position) { ids = append(ids, o) })
	return ids
}

func (h rageAct) GoTo(id entity.EntityID, a animal.State, x, y int) animal.State {
	h.g.animalGoTo(id, &a, x, y)
	return a
}

func (h rageAct) PathQuery(tame bool) pathing.Query { return h.g.animalPathQuery(tame, false) }
func (h rageAct) InBounds(x, y int) bool            { return h.g.grid.InBounds(x, y) }
func (h rageAct) RevengePct(base int) int           { return h.g.revengePct(base) }
func (h rageAct) PredatorsHuntColonists() bool      { return h.g.predatorsHuntColonists() }
func (h rageAct) EdgeTile() (int, int, bool)        { return h.g.edgeTile() }

func (h rageAct) LayerReach(q pathing.Query, p rage.Pos, x, y int) bool {
	return h.g.layerReach(q, Position(p), x, y)
}

func (h rageAct) WalkableNear(x, y, r int) (int, int, bool) {
	return h.g.combat.Tiles.WalkableNear(x, y, r)
}

func (h rageAct) FireThreat(a animal.State, p rage.Pos) (rage.Pos, bool) {
	tp, ok := h.g.animalFireThreat(&a, Position(p))
	return rage.Pos(tp), ok
}

func (h rageAct) PickSpecies(keep func(animal.Species) bool) (animal.Species, bool) {
	return h.g.pickSpecies(keep)
}

func (h rageAct) Strike(attacker, target entity.EntityID, wk weapon.Kind) {
	h.g.combat.Strikes.Strike(attacker, target, wk)
}

func (h rageAct) TameHuntOn(id, target entity.EntityID, tp rage.Pos) bool {
	return h.g.tameHuntOn(id, target, Position(tp))
}
