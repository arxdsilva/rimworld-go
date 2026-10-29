package combat

import (
	"fmt"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeHost is a small open map for combat's unit tests: every role, kept
// in plain maps, recording what combat asked of it.
type fakeHost struct {
	w, h                 int
	now                  uint64
	pos                  map[entity.EntityID][2]int
	animals              map[entity.EntityID]animal.State
	weapons              map[entity.EntityID]weapon.Kind
	pawns                map[entity.EntityID]bool // standing colonists
	raiders              map[entity.EntityID]bool // raiders (value: standing)
	drafted              map[entity.EntityID]bool
	pacific              map[entity.EntityID]bool
	animalIDs, raiderIDs []entity.EntityID

	rock  map[[2]int]bool
	objs  map[[2]int]entity.EntityID
	bldgs map[entity.EntityID]construction.Building

	jobs    map[entity.EntityID]job.Record
	started []job.Record
	interr  int
	noPath  bool

	rolls  []int
	droll  []int
	hurts  []string
	damage map[entity.EntityID]int
	traces []projectile.Trace
	struck map[entity.EntityID]entity.EntityID
	hits   map[entity.EntityID]int
	stains int
	logs   []string
	msgs   []string
	bursts int
	fires  int
	frames []construction.Kind
	dug    int
	learn  int

	wired   bool
	powered map[entity.EntityID]bool
	gen     uint64
	defense []entity.EntityID

	calm      map[entity.EntityID]uint64
	cornered  bool
	dropped   int
	checks    int
	givenUp   bool
	cleared   int
	abandoned int
	route     []pathing.Point
	noRoute   bool
	blocked   map[[2]int]bool
}

func newFake(w, h int) *fakeHost {
	return &fakeHost{
		w: w, h: h, now: 1000,
		pos: map[entity.EntityID][2]int{}, animals: map[entity.EntityID]animal.State{},
		weapons: map[entity.EntityID]weapon.Kind{}, pawns: map[entity.EntityID]bool{},
		raiders: map[entity.EntityID]bool{}, drafted: map[entity.EntityID]bool{}, pacific: map[entity.EntityID]bool{},
		rock: map[[2]int]bool{}, objs: map[[2]int]entity.EntityID{}, bldgs: map[entity.EntityID]construction.Building{},
		jobs: map[entity.EntityID]job.Record{}, damage: map[entity.EntityID]int{},
		struck: map[entity.EntityID]entity.EntityID{}, hits: map[entity.EntityID]int{},
		powered: map[entity.EntityID]bool{}, calm: map[entity.EntityID]uint64{}, blocked: map[[2]int]bool{},
	}
}

func (f *fakeHost) combat() *Combat {
	return New(f, Roles{Power: f, Flight: f, Hunts: f, Breach: f})
}

func (f *fakeHost) pawn(id entity.EntityID, x, y int, wk weapon.Kind) {
	f.pos[id], f.pawns[id], f.weapons[id] = [2]int{x, y}, true, wk
}

func (f *fakeHost) raider(id entity.EntityID, x, y int, wk weapon.Kind) {
	f.pos[id], f.raiders[id], f.weapons[id] = [2]int{x, y}, true, wk
	f.raiderIDs = append(f.raiderIDs, id)
}

func (f *fakeHost) beast(id entity.EntityID, x, y int, a animal.State) {
	f.pos[id], f.animals[id] = [2]int{x, y}, a
	f.animalIDs = append(f.animalIDs, id)
}

func (f *fakeHost) build(id entity.EntityID, k construction.Kind, x, y int) {
	f.bldgs[id] = construction.Building{Kind: k, X: x, Y: y, Stuff: item.Steel}
	f.objs[[2]int{x, y}] = id
}

// Creatures
func (f *fakeHost) Now() uint64 { return f.now }
func (f *fakeHost) Pos(id entity.EntityID) (int, int, bool) {
	p, ok := f.pos[id]
	return p[0], p[1], ok
}
func (f *fakeHost) Animal(id entity.EntityID) (animal.State, bool) {
	a, ok := f.animals[id]
	return a, ok
}
func (f *fakeHost) Weapon(id entity.EntityID) weapon.Kind  { return f.weapons[id] }
func (f *fakeHost) StandingPawn(id entity.EntityID) bool   { return f.pawns[id] || f.raiders[id] } // raiders are named humanlikes too
func (f *fakeHost) StandingRaider(id entity.EntityID) bool { return f.raiders[id] }
func (f *fakeHost) RaiderOn(t, id entity.EntityID) (int, int, bool) {
	j := f.jobs[t]
	if !f.raiders[t] || j.Kind != job.Attack || j.Target != id {
		return 0, 0, false
	}
	p := f.pos[t]
	return p[0], p[1], true
}
func (f *fakeHost) Hostile(a animal.State) bool     { return a.Mode == animal.Attack && f.pawns[a.Target] }
func (f *fakeHost) AnimalIDs() []entity.EntityID    { return f.animalIDs }
func (f *fakeHost) RaiderIDs() []entity.EntityID    { return f.raiderIDs }
func (f *fakeHost) Drafted(id entity.EntityID) bool { return f.drafted[id] }
func (f *fakeHost) Name(id entity.EntityID) string  { return fmt.Sprintf("p%d", id) }
func (f *fakeHost) Near(x, y, r int, who Who, buf []Seen) []Seen {
	for id, p := range f.pos {
		_, beast := f.animals[id]
		_, raider := f.raiders[id]
		kind := Pawns
		switch {
		case beast:
			kind = Animals
		case raider:
			kind = Raiders
		}
		if who&kind != 0 && weapon.Distance(x, y, p[0], p[1]) <= r {
			buf = append(buf, Seen{ID: id, X: p[0], Y: p[1]})
		}
	}
	return buf
}

// Ground
func (f *fakeHost) InBounds(x, y int) bool { return x >= 0 && y >= 0 && x < f.w && y < f.h }
func (f *fakeHost) Rock(x, y int) bool     { return f.rock[[2]int{x, y}] }
func (f *fakeHost) Object(x, y int) (entity.EntityID, construction.Kind) {
	id := f.objs[[2]int{x, y}]
	return id, f.bldgs[id].Kind
}
func (f *fakeHost) Building(id entity.EntityID) (construction.Building, bool) {
	b, ok := f.bldgs[id]
	return b, ok
}
func (f *fakeHost) AnyBuilt(k construction.Kind) bool {
	for _, b := range f.bldgs {
		if b.Kind == k {
			return true
		}
	}
	return false
}
func (f *fakeHost) RemoveBuilding(id entity.EntityID) {
	b := f.bldgs[id]
	delete(f.objs, [2]int{b.X, b.Y})
	delete(f.bldgs, id)
}
func (f *fakeHost) PlaceFrame(k construction.Kind, _ item.Kind, _ construction.Facing, _, _ int) {
	f.frames = append(f.frames, k)
}
func (f *fakeHost) DigRock(_, _, dmg int) { f.dug += dmg }

// Skills
func (f *fakeHost) HitChance(_ entity.EntityID, _ bool, base int) int { return base }
func (f *fakeHost) Learn(entity.EntityID, bool)                       { f.learn++ }
func (f *fakeHost) AimTicks(_ entity.EntityID, cooldown int) int      { return cooldown }
func (f *fakeHost) CanViolence(id entity.EntityID) bool               { return !f.pacific[id] }
func (f *fakeHost) FightsBack(id entity.EntityID) bool {
	return f.weapons[id] != weapon.Fists && !f.pacific[id]
}

// Jobs
func (f *fakeHost) Job(id entity.EntityID) (job.Record, bool) { j, ok := f.jobs[id]; return j, ok }
func (f *fakeHost) SetJob(id entity.EntityID, j job.Record)   { f.jobs[id] = j }
func (f *fakeHost) StartJob(id entity.EntityID, j job.Record) bool {
	f.started = append(f.started, j)
	f.jobs[id] = j
	return true
}
func (f *fakeHost) Interrupt(entity.EntityID, job.FailReason) { f.interr++ }
func (f *fakeHost) PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool) {
	if f.noPath {
		return nil, false
	}
	return []pathing.Point{{X: x, Y: y}}, true
}

// Harm
func (f *fakeHost) Hurt(target entity.EntityID, dmg int, by entity.EntityID) {
	f.hurts = append(f.hurts, fmt.Sprintf("%d-%d-%d", target, dmg, by))
}
func (f *fakeHost) HurtAs(target entity.EntityID, dmg int, by entity.EntityID, k body.WoundKind, _ health.Cause) {
	f.hurts = append(f.hurts, fmt.Sprintf("%d-%d-%d-%d", target, dmg, by, k))
}
func (f *fakeHost) DamageBuilding(id entity.EntityID, dmg int)          { f.damage[id] += dmg }
func (f *fakeHost) Stain(entity.EntityID, int, int, int, int, int)      { f.stains++ }
func (f *fakeHost) Trace(t projectile.Trace)                            { f.traces = append(f.traces, t) }
func (f *fakeHost) Struck(attacker, target entity.EntityID, _ uint64)   { f.struck[attacker] = target }
func (f *fakeHost) Hit(target entity.EntityID, _ uint64, _, _, dmg int) { f.hits[target] += dmg }

// Events
func (f *fakeHost) LogStrike(a, t entity.EntityID, _ weapon.Kind, hit bool, _ int) {
	f.logs = append(f.logs, fmt.Sprintf("strike %d %d %v", a, t, hit))
}
func (f *fakeHost) Fled(id, threat entity.EntityID) {
	f.logs = append(f.logs, fmt.Sprintf("fled %d %d", id, threat))
}
func (f *fakeHost) Shot(t entity.EntityID, hit bool, _ int, by string) {
	f.logs = append(f.logs, fmt.Sprintf("shot %d %v %s", t, hit, by))
}
func (f *fakeHost) Trapped(v entity.EntityID, dmg int) {
	f.logs = append(f.logs, fmt.Sprintf("trap %d %d", v, dmg))
}
func (f *fakeHost) Message(_, _ int, text string) { f.msgs = append(f.msgs, text) }
func (f *fakeHost) Burst(int, int, uint64)        { f.bursts++ }
func (f *fakeHost) Ignite(int, int)               { f.fires++ }

// Dice: queued rolls, 0 once they run out (a sure hit).
func (f *fakeHost) Roll(int) int {
	if len(f.rolls) == 0 {
		return 0
	}
	r := f.rolls[0]
	f.rolls = f.rolls[1:]
	return r
}
func (f *fakeHost) DefenseRoll(int) int {
	if len(f.droll) == 0 {
		return 0
	}
	r := f.droll[0]
	f.droll = f.droll[1:]
	return r
}

// Power
func (f *fakeHost) Wired() bool                     { return f.wired }
func (f *fakeHost) Powered(id entity.EntityID) bool { return f.powered[id] }
func (f *fakeHost) PowerGen() uint64                { return f.gen }
func (f *fakeHost) AppendDefense(buf []entity.EntityID) []entity.EntityID {
	return append(buf, f.defense...)
}

// Flight
func (f *fakeHost) FleeGoal(_ entity.EntityID, x, y, tx, ty int) (int, int, bool) {
	if f.cornered {
		return 0, 0, false
	}
	return x + sign(x-tx)*8, y + sign(y-ty)*8, true
}
func (f *fakeHost) DropForFlight(entity.EntityID, job.Record) { f.dropped++ }
func (f *fakeHost) CheckThreats(entity.EntityID, int, int, int, int, entity.EntityID, int) {
	f.checks++
}
func (f *fakeHost) Calm(id entity.EntityID) (uint64, bool)   { u, ok := f.calm[id]; return u, ok }
func (f *fakeHost) SetCalm(id entity.EntityID, until uint64) { f.calm[id] = until }
func (f *fakeHost) ForgetCalm(id entity.EntityID)            { delete(f.calm, id) }

// Hunts
func (f *fakeHost) ClearHuntOut(entity.EntityID) { f.cleared++ }
func (f *fakeHost) HuntGivenUp(entity.EntityID, job.Record, int, int, uint64) bool {
	return f.givenUp
}
func (f *fakeHost) AbandonHunt(entity.EntityID, job.Record) { f.abandoned++ }

// Breaches
func (f *fakeHost) BreachRoute(entity.EntityID, int, int) ([]pathing.Point, bool) {
	return f.route, !f.noRoute
}
func (f *fakeHost) Query(entity.EntityID) pathing.Query { return fakeQuery{f} }

type fakeQuery struct{ f *fakeHost }

func (q fakeQuery) Width() int  { return q.f.w }
func (q fakeQuery) Height() int { return q.f.h }
func (q fakeQuery) Walkable(x, y int) bool {
	_, obj := q.f.objs[[2]int{x, y}]
	return !q.f.blocked[[2]int{x, y}] && !obj && !q.f.rock[[2]int{x, y}]
}

var _ Host = (*fakeHost)(nil)

func entityID(v uint64) entity.EntityID { return entity.EntityID(v) }
