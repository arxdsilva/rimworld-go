package kinds

// fakes_test.go: one set of recording fakes for every quest.Roles role,
// shared by the kinds' tests (lead-owned: a kind worker asks the lead for
// a change here rather than editing it). Tests drive a kind through the
// real root: newRoot, then Offer/Apply/Signal/Hourly.

import (
	"slices"
	"strconv"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
)

const (
	hour = clock.TicksPerHour
	day  = clock.TicksPerDay
)

// fakeRolls is a seeded splitmix stream; script values are drawn first.
type fakeRolls struct {
	s      uint64
	script []int
}

func (r *fakeRolls) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *fakeRolls) Intn(n int) int {
	if len(r.script) > 0 {
		v := r.script[0]
		r.script = r.script[1:]
		return v % n
	}
	return int(r.next() % uint64(n))
}

func (r *fakeRolls) Seed() uint64 { return r.next() | 1 }

type fakeWorld struct {
	landed            uint64
	colonists, points int
	scale, reward     int
	threats           bool
	factions          []quest.FactionView
	// near is NearTile's answer (-1: none free).
	near int
}

func (w *fakeWorld) Landed() uint64                { return w.landed }
func (w *fakeWorld) Colonists() int                { return w.colonists }
func (w *fakeWorld) Points() int                   { return w.points }
func (w *fakeWorld) ThreatScalePct() int           { return w.scale }
func (w *fakeWorld) RewardPct() int                { return w.reward }
func (w *fakeWorld) QuestThreats() bool            { return w.threats }
func (w *fakeWorld) Factions() []quest.FactionView { return w.factions }
func (w *fakeWorld) NearTile(seed uint64, lo, hi int) (int, bool) {
	return w.near, w.near >= 0
}

type fakeClock struct{ now uint64 }

func (c *fakeClock) Now() uint64 { return c.now }

type fakeSites struct {
	placed  []quest.Site
	removed []int
	refuse  bool
	// deposits are the scanner's lumps by tile.
	deposits map[int]trade.Line
}

func (s *fakeSites) Place(x quest.Site) bool {
	if s.refuse {
		return false
	}
	s.placed = append(s.placed, x)
	return true
}
func (s *fakeSites) Remove(tile int, id qd.ID) { s.removed = append(s.removed, tile) }

// Settlement is 100 + f when f has a settlement in the world's views.
func (s *fakeSites) Settlement(f int, seed uint64) (int, bool) { return 100 + f, true }

func (s *fakeSites) Deposit(tile int) (trade.Line, bool) { l, ok := s.deposits[tile]; return l, ok }

type fakePawns struct {
	walked []uint64
	joined []entity.EntityID
	dead   []entity.EntityID
	// social is BestSocial's level (-1: the caravan has nobody).
	social int
	xp     map[skill.ID]int
}

func (p *fakePawns) Preview(seed uint64) quest.PawnPreview {
	return quest.PawnPreview{Name: "pawn" + strconv.FormatUint(seed%1000, 10), Age: 20 + int(seed%40)}
}
func (p *fakePawns) WalkIn(seed uint64) (entity.EntityID, bool) {
	p.walked = append(p.walked, seed)
	return entity.EntityID(seed), true
}
func (p *fakePawns) Join(id entity.EntityID) bool  { p.joined = append(p.joined, id); return true }
func (p *fakePawns) Alive(id entity.EntityID) bool { return !slices.Contains(p.dead, id) }
func (p *fakePawns) BestSocial(c uint64) (entity.EntityID, int, bool) {
	return entity.EntityID(c * 10), p.social, c != 0 && p.social >= 0
}
func (p *fakePawns) GrantXP(id entity.EntityID, s skill.ID, xp int) { p.xp[s] += xp }

type fakeItems struct {
	pods [][]trade.Line
	into map[uint64][]trade.Line
	// has: the caravan carries what is asked; taken lists what Take took.
	has   bool
	taken [][]trade.Line
	// at is CaravanAt's answer per caravan (missing: moving).
	at map[uint64]int
}

func (it *fakeItems) Pod(l []trade.Line, seed uint64) bool {
	it.pods = append(it.pods, slices.Clone(l))
	return true
}
func (it *fakeItems) IntoCaravan(c uint64, l []trade.Line) bool {
	it.into[c] = append(it.into[c], l...)
	return true
}
func (it *fakeItems) Has(c uint64, l []trade.Line) bool { return it.has }
func (it *fakeItems) Take(c uint64, l []trade.Line) bool {
	if it.has {
		it.taken = append(it.taken, slices.Clone(l))
	}
	return it.has
}
func (it *fakeItems) CaravanAt(c uint64) (int, bool) { t, ok := it.at[c]; return t, ok }

// threatCall is one Threats call: Kind "raid", "manhunters", "condition"
// or "ambush".
type threatCall struct {
	Kind      string
	Faction   int
	Points    int
	Cond      uint8
	Ticks     uint64
	Caravan   uint64
	AtTick    uint64
	Succeeded bool
}

type fakeThreats struct {
	clock *fakeClock
	calls []threatCall
	fail  bool
}

func (t *fakeThreats) add(c threatCall) bool {
	c.AtTick, c.Succeeded = t.clock.now, !t.fail
	t.calls = append(t.calls, c)
	return !t.fail
}
func (t *fakeThreats) Raid(f, points int, seed uint64) bool {
	return t.add(threatCall{Kind: "raid", Faction: f, Points: points})
}
func (t *fakeThreats) Manhunters(points int, seed uint64) bool {
	return t.add(threatCall{Kind: "manhunters", Points: points})
}
func (t *fakeThreats) Condition(kind uint8, ticks uint64) bool {
	return t.add(threatCall{Kind: "condition", Cond: kind, Ticks: ticks})
}
func (t *fakeThreats) Ambush(c uint64, f, p int, s uint64) bool {
	return t.add(threatCall{Kind: "ambush", Caravan: c, Faction: f, Points: p})
}

type fakeLetters struct{ list []quest.Letter }

func (l *fakeLetters) Post(x quest.Letter) { l.list = append(l.list, x) }

type fakeGoodwill struct {
	rel      []faction.Relation
	adjusted [][2]int // faction index, delta
}

func (g *fakeGoodwill) Adjust(f, d int)                 { g.adjusted = append(g.adjusted, [2]int{f, d}) }
func (g *fakeGoodwill) Relation(f int) faction.Relation { return g.rel[f] }
func (g *fakeGoodwill) Count() int                      { return len(g.rel) }

// builtGoods is one Builder.Goods call.
type builtGoods struct {
	Lines  []trade.Line
	X, Y   int
	Forbid bool
}

// builtPawn is one Builder.Downed or Held call.
type builtPawn struct {
	Seed       uint64
	Faction    int
	X, Y       int
	Held       bool
	ID         entity.EntityID
	Spawned    bool
	AtTick     uint64
	MapCurrent bool
}

// fakeBuilder records what a kind laid; refuse makes every spawn fail.
// current is false outside SigSiteOpened (the test sets it), so a kind
// calling the builder at the wrong time is caught.
type fakeBuilder struct {
	clock   *fakeClock
	current bool
	refuse  bool
	camps   []mapgen.Compound
	arms    [][]item.Kind
	goods   []builtGoods
	pawns   []builtPawn
	next    entity.EntityID
}

func (b *fakeBuilder) Size() (int, int) { return 150, 150 }
func (b *fakeBuilder) Camp(c mapgen.Compound, f int, arms []item.Kind, seed uint64) int {
	if b.refuse {
		return 0
	}
	b.camps, b.arms = append(b.camps, c), append(b.arms, slices.Clone(arms))
	return len(arms)
}
func (b *fakeBuilder) Goods(l []trade.Line, x, y int, forbid bool) {
	b.goods = append(b.goods, builtGoods{slices.Clone(l), x, y, forbid})
}
func (b *fakeBuilder) spawn(seed uint64, f, x, y int, held bool) (entity.EntityID, bool) {
	b.next++
	p := builtPawn{Seed: seed, Faction: f, X: x, Y: y, Held: held, ID: 1000 + b.next, Spawned: !b.refuse, AtTick: b.clock.now, MapCurrent: b.current}
	b.pawns = append(b.pawns, p)
	if b.refuse {
		return 0, false
	}
	return p.ID, true
}
func (b *fakeBuilder) Downed(seed uint64, f, x, y int) (entity.EntityID, bool) {
	return b.spawn(seed, f, x, y, false)
}
func (b *fakeBuilder) Held(seed uint64, f, x, y int) (entity.EntityID, bool) {
	return b.spawn(seed, f, x, y, true)
}

// env is one set of fakes: faction 0 neutral and friendly-capable with
// settlement tile 5, faction 1 hostile and friendly-capable (a talks
// giver) with tile 7, faction 2 hostile and never friendly (pirates).
type env struct {
	world   *fakeWorld
	clock   *fakeClock
	sites   *fakeSites
	pawns   *fakePawns
	items   *fakeItems
	threats *fakeThreats
	letters *fakeLetters
	gw      *fakeGoodwill
	rolls   *fakeRolls
	builder *fakeBuilder
}

func newEnv() *env {
	c := &fakeClock{now: 30 * day}
	return &env{
		world: &fakeWorld{colonists: 3, points: 500, scale: 100, reward: 100, threats: true, near: 40,
			factions: []quest.FactionView{
				{Index: 0, Relation: faction.Neutral, CanBeFriendly: true, Settlements: []int{5}},
				{Index: 1, Relation: faction.Hostile, Goodwill: -80, CanBeFriendly: true, Settlements: []int{7}},
				{Index: 2, Relation: faction.Hostile, Goodwill: -100, Settlements: []int{9}},
			}},
		clock: c, sites: &fakeSites{deposits: map[int]trade.Line{}}, pawns: &fakePawns{social: 8, xp: map[skill.ID]int{}},
		items:   &fakeItems{into: map[uint64][]trade.Line{}, at: map[uint64]int{}},
		threats: &fakeThreats{clock: c}, letters: &fakeLetters{},
		gw:    &fakeGoodwill{rel: []faction.Relation{faction.Neutral, faction.Hostile, faction.Hostile}},
		rolls: &fakeRolls{s: 7}, builder: &fakeBuilder{clock: c},
	}
}

func (e *env) roles() quest.Roles {
	return quest.Roles{World: e.world, Clock: e.clock, Sites: e.sites, Pawns: e.pawns, Items: e.items,
		Threats: e.threats, Letters: e.letters, Goodwill: e.gw, Rolls: e.rolls, Builder: e.builder}
}

// newRoot is a quests root that knows only kind k, over e's fakes.
func newRoot(e *env, k quest.Kind) *quest.Quests { return quest.New(e.roles(), []quest.Kind{k}) }

// get is quest id (fails the test when missing).
func get(t *testing.T, q *quest.Quests, id qd.ID) qd.Quest {
	t.Helper()
	n, ok := q.Get(id)
	if !ok {
		t.Fatalf("quest %d missing", id)
	}
	return n
}

// openSite delivers SigSiteOpened for quest id on tile t as the game does:
// the site map (id map) is current while the signal is delivered.
func openSite(e *env, q *quest.Quests, id qd.ID, t int, m uint32) {
	e.builder.current = true
	q.Signal(quest.Signal{Kind: quest.SigSiteOpened, Quest: id, Tile: t, Map: m})
	e.builder.current = false
}

// run advances e's clock hour by hour to tick until, calling Hourly.
func run(e *env, q *quest.Quests, until uint64) {
	for e.clock.now < until {
		e.clock.now += hour
		q.Hourly(e.clock.now)
	}
}

func TestKindsAllHasEveryKindOnce(t *testing.T) {
	t.Parallel()
	seen := map[qd.KindID]bool{}
	for _, k := range All() {
		d := k.Def()
		if d.Kind == qd.KindNone || d.Kind >= qd.NumKinds || seen[d.Kind] {
			t.Fatalf("kind %d is out of range or repeated", d.Kind)
		}
		seen[d.Kind] = true
	}
	if len(seen) != int(qd.NumKinds)-1 {
		t.Fatalf("%d kinds, want %d", len(seen), qd.NumKinds-1)
	}
}
