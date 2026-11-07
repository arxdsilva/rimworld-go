package quest

import (
	"slices"
	"strconv"
	"testing"

	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

const (
	hour = clock.TicksPerHour
	day  = clock.TicksPerDay
)

// needDomain skips a test that needs the domain's transition and cadence
// bodies while internal/domain/quest still holds D-dom's placeholders
// (QST-1 step 1).
func needDomain(t testing.TB) {
	t.Helper()
	q := qd.Offer(qd.Quest{ID: 1}, 100, qd.Def{AcceptTicks: 10})
	_, _, _, due := qd.Due(q)
	if q.State != qd.Offered || q.AcceptBy != 110 || !due || qd.WindowOf(qd.WindowDays*day) != 1 {
		t.Skip("needs D-dom's internal/domain/quest bodies (QST-1 step 1): the domain still returns placeholders")
	}
}

// fakeRolls is a seeded splitmix stream; script values are drawn first.
type fakeRolls struct {
	s      uint64
	script []int
	draws  int
}

func (r *fakeRolls) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *fakeRolls) Intn(n int) int {
	r.draws++
	if len(r.script) > 0 {
		v := r.script[0]
		r.script = r.script[1:]
		return v % n
	}
	return int(r.next() % uint64(n))
}

func (r *fakeRolls) Seed() uint64 { r.draws++; return r.next() | 1 }

type fakeWorld struct {
	landed               uint64
	colonists, points    int
	scale, reward        int
	threats              bool
	factions             []FactionView
	nearTiles, landCalls int
}

func (w *fakeWorld) Landed() uint64          { w.landCalls++; return w.landed }
func (w *fakeWorld) Colonists() int          { return w.colonists }
func (w *fakeWorld) Points() int             { return w.points }
func (w *fakeWorld) ThreatScalePct() int     { return w.scale }
func (w *fakeWorld) RewardPct() int          { return w.reward }
func (w *fakeWorld) QuestThreats() bool      { return w.threats }
func (w *fakeWorld) Factions() []FactionView { return w.factions }
func (w *fakeWorld) NearTile(seed uint64, lo, hi int) (int, bool) {
	w.nearTiles++
	return lo + int(seed%uint64(hi-lo+1)), true
}

type fakeClock struct{ now uint64 }

func (c *fakeClock) Now() uint64 { return c.now }

type fakeSites struct {
	placed  []Site
	removed []int
}

func (s *fakeSites) Place(x Site) bool                         { s.placed = append(s.placed, x); return true }
func (s *fakeSites) Remove(tile int, id qd.ID)                 { s.removed = append(s.removed, tile) }
func (s *fakeSites) Settlement(f int, seed uint64) (int, bool) { return 100 + f, true }

type fakePawns struct {
	walked []uint64
	joined []entity.EntityID
	dead   []entity.EntityID
	xp     int
}

func (p *fakePawns) Preview(seed uint64) PawnPreview {
	return PawnPreview{Name: "pawn" + strconv.FormatUint(seed%1000, 10), Age: 20 + int(seed%40)}
}
func (p *fakePawns) WalkIn(seed uint64) (entity.EntityID, bool) {
	p.walked = append(p.walked, seed)
	return entity.EntityID(seed), true
}
func (p *fakePawns) Join(id entity.EntityID) bool  { p.joined = append(p.joined, id); return true }
func (p *fakePawns) Alive(id entity.EntityID) bool { return !slices.Contains(p.dead, id) }
func (p *fakePawns) BestSocial(caravan uint64) (entity.EntityID, int, bool) {
	return entity.EntityID(caravan * 10), 8, caravan != 0
}
func (p *fakePawns) GrantXP(id entity.EntityID, s skill.ID, xp int) { p.xp += xp }

type fakeItems struct {
	pods    [][]trade.Line
	into    []uint64
	has     bool
	took    int
	podSeed []uint64
}

func (it *fakeItems) Pod(l []trade.Line, seed uint64) bool {
	it.pods = append(it.pods, l)
	it.podSeed = append(it.podSeed, seed)
	return true
}
func (it *fakeItems) IntoCaravan(c uint64, l []trade.Line) bool {
	it.into = append(it.into, c)
	return true
}
func (it *fakeItems) Has(c uint64, l []trade.Line) bool  { return it.has }
func (it *fakeItems) Take(c uint64, l []trade.Line) bool { it.took++; return it.has }

type fakeThreats struct{ raids, manhunters, conditions, ambushes int }

func (t *fakeThreats) Raid(f, points int, seed uint64) bool     { t.raids++; return true }
func (t *fakeThreats) Manhunters(points int, seed uint64) bool  { t.manhunters++; return true }
func (t *fakeThreats) Condition(kind uint8, ticks uint64) bool  { t.conditions++; return true }
func (t *fakeThreats) Ambush(c uint64, f, p int, s uint64) bool { t.ambushes++; return true }

type fakeLetters struct{ list []Letter }

func (l *fakeLetters) Post(x Letter) { l.list = append(l.list, x) }

// of counts the letters of kind k.
func (l *fakeLetters) of(k LetterKind) int {
	n := 0
	for _, x := range l.list {
		if x.Kind == k {
			n++
		}
	}
	return n
}

type fakeGoodwill struct {
	rel      []faction.Relation
	adjusted [][2]int
	reads    int
}

func (g *fakeGoodwill) Adjust(f, d int) { g.adjusted = append(g.adjusted, [2]int{f, d}) }
func (g *fakeGoodwill) Relation(f int) faction.Relation {
	g.reads++
	return g.rel[f]
}
func (g *fakeGoodwill) Count() int { return len(g.rel) }

// env is one set of fakes.
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
}

func newEnv() *env {
	return &env{
		world: &fakeWorld{colonists: 3, points: 500, scale: 100, reward: 100, threats: true,
			factions: []FactionView{{Index: 0, CanBeFriendly: true, Settlements: []int{5}}, {Index: 1, Relation: faction.Hostile}}},
		clock: &fakeClock{}, sites: &fakeSites{}, pawns: &fakePawns{}, items: &fakeItems{},
		threats: &fakeThreats{}, letters: &fakeLetters{},
		gw:    &fakeGoodwill{rel: []faction.Relation{faction.Neutral, faction.Hostile}},
		rolls: &fakeRolls{s: 7},
	}
}

func (e *env) roles() Roles {
	return Roles{World: e.world, Clock: e.clock, Sites: e.sites, Pawns: e.pawns, Items: e.items,
		Threats: e.threats, Letters: e.letters, Goodwill: e.gw, Rolls: e.rolls}
}

// stubKind is a configurable Kind. Build gives the quest a site at tile
// (and a pawn part when pawn != 0); Accepted adds a threat part partIn
// ticks ahead (when set); a caravan arriving at the site succeeds; the
// bound pawn dying fails it (PawnLost).
type stubKind struct {
	def           qd.Def
	off           bool
	giver, target int
	tile          int
	pawn          uint64
	rewards       []qd.Reward
	partIn, rearm uint64
	partVerdict   Verdict
	build         func(c Ctx, q *qd.Quest) bool
	built         int
	accepted      int
	ended         int
	partCalls     int
	signals       map[qd.ID]int
	lastSig       Signal
}

func newStub(k qd.KindID) *stubKind {
	return &stubKind{def: qd.Def{Kind: k, Weight: 10, AcceptTicks: 6 * day, DurationTicks: 17 * day},
		giver: 1, tile: 42, signals: map[qd.ID]int{}}
}

func (s *stubKind) Def() qd.Def         { return s.def }
func (s *stubKind) CanOffer(c Ctx) bool { return !s.off }
func (s *stubKind) Build(c Ctx, q *qd.Quest) bool {
	s.built++
	if s.build != nil {
		return s.build(c, q)
	}
	q.Giver, q.Target = s.giver, s.target
	q.Parts = []qd.Part{{Kind: qd.PartSite, Tile: s.tile, Seed: c.Seed()}}
	if s.pawn != 0 {
		q.Parts = append(q.Parts, qd.Part{Kind: qd.PartPawn, Tile: -1, Pawn: s.pawn})
	}
	q.Rewards = slices.Clone(s.rewards)
	return true
}
func (s *stubKind) Accepted(c Ctx, q *qd.Quest) {
	s.accepted++
	if s.partIn > 0 {
		q.Parts = append(q.Parts, qd.Part{Kind: qd.PartThreat, Tile: -1, At: c.Now + s.partIn})
	}
}
func (s *stubKind) Signal(c Ctx, q *qd.Quest, sig Signal) Verdict {
	s.signals[q.ID]++
	s.lastSig = sig
	switch sig.Kind {
	case SigCaravanArrived, SigFulfil:
		return VerdictSucceed
	case SigPawnDied:
		return VerdictFail(qd.PawnLost)
	}
	return VerdictNone
}
func (s *stubKind) PartDue(c Ctx, q *qd.Quest, part int) Verdict {
	s.partCalls++
	if s.rearm > 0 {
		q.Parts[part].At += s.rearm
	}
	return s.partVerdict
}
func (s *stubKind) Ended(c Ctx, q *qd.Quest) { s.ended++ }
func (s *stubKind) Detail(q qd.Quest) []Line {
	return []Line{{Key: "quest.stub.detail", Args: []string{strconv.Itoa(int(q.ID))}}}
}

// stateOf is quest id's state (fails the test when missing).
func stateOf(t *testing.T, q *Quests, id qd.ID) qd.Quest {
	t.Helper()
	n, ok := q.Get(id)
	if !ok {
		t.Fatalf("quest %d missing", id)
	}
	return n
}
