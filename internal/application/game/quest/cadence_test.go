package quest

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"

	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
)

// run drives q hour by hour from tick from to tick to, with Daily at each
// midnight before that hour's Hourly, as the game will (QST-160).
func run(q *Quests, e *env, from, to uint64) {
	for now := from; now <= to; now += hour {
		e.clock.now = now
		if now%day == 0 {
			q.Daily(now)
		}
		q.Hourly(now)
	}
}

// offersPerWindow counts the quests offered in each window from landing.
func offersPerWindow(q *Quests, landed uint64, windows int) []int {
	n := make([]int, windows)
	for _, v := range q.View() {
		if w := qd.WindowOf(v.OfferedAt - landed); w < windows {
			n[w]++
		}
	}
	return n
}

func TestCadenceOneOfferPerWindowAndLostWindowNotMadeUp(t *testing.T) {
	needDomain(t)
	e := newEnv()
	a := newStub(qd.Camp)
	b := newStub(qd.Request)
	b.def.Weight = 30
	q := New(e.roles(), []Kind{a, b})
	const windows = 30
	end := uint64(windows*qd.WindowDays*day) - 1
	lostFrom, lostTo := uint64(5*qd.WindowDays*day), uint64(6*qd.WindowDays*day)
	for now := uint64(0); now <= end; now += hour {
		off := now >= lostFrom && now < lostTo
		a.off, b.off = off, off
		e.clock.now = now
		if now%day == 0 {
			q.Daily(now)
		}
		q.Hourly(now)
	}
	got := offersPerWindow(q, 0, windows)
	for w, n := range got {
		want := 1
		if w == 5 {
			want = 0
		}
		if n != want {
			t.Fatalf("window %d had %d offers, want %d (all: %v)", w, n, want, got)
		}
	}
	for _, v := range q.View() {
		if v.OfferedAt < qd.MinDay*day {
			t.Fatalf("offer at tick %d, before day %d", v.OfferedAt, qd.MinDay)
		}
	}
	if a.built == 0 || b.built == 0 {
		t.Fatalf("the weighted pick never chose a kind: %d/%d", a.built, b.built)
	}
	if e.letters.of(LetterOffer) != windows-1 {
		t.Fatalf("%d offer letters", e.letters.of(LetterOffer))
	}
}

func TestCadenceLateFirstWindowIsLost(t *testing.T) {
	needDomain(t)
	e := newEnv()
	e.rolls.script = []int{0, 0} // window 0 at day 6, window 1 at day 10
	a := newStub(qd.Camp)
	q := New(e.roles(), []Kind{a})
	run(q, e, 9*day, 12*day)
	got := offersPerWindow(q, 0, 2)
	if got[0] != 0 || got[1] != 1 {
		t.Fatalf("offers per window = %v, want [0 1]", got)
	}
	if v := q.View(); v[0].OfferedAt != 10*day {
		t.Fatalf("offered at %d", v[0].OfferedAt)
	}
	// Before landing nothing is scheduled.
	e2 := newEnv()
	e2.world.landed = 5 * day
	q2 := New(e2.roles(), []Kind{newStub(qd.Camp)})
	q2.Daily(day)
	if q2.Export() != nil {
		t.Fatal("scheduled before landing")
	}
}

func TestPickSkipsViolentAndFullColony(t *testing.T) {
	needDomain(t)
	e := newEnv()
	e.world.threats = false
	e.world.colonists = 12
	violent := newStub(qd.Camp)
	violent.def.Violent = true
	joiner := newStub(qd.Joiner)
	joiner.def.Joins = true
	zero := newStub(qd.OreLump)
	zero.def.Weight = 0
	q := New(e.roles(), []Kind{violent, joiner, zero})
	if k, ok := q.pick(q.ctx(0)); ok {
		t.Fatalf("picked kind %d", k)
	}
	e.world.threats = true
	if k, ok := q.pick(q.ctx(0)); !ok || k != qd.Camp {
		t.Fatalf("picked %d %v, want the camp", k, ok)
	}
	e.world.colonists = 3
	e.world.threats = false
	if k, ok := q.pick(q.ctx(0)); !ok || k != qd.Joiner {
		t.Fatalf("picked %d %v, want the joiner", k, ok)
	}
}

func TestCadenceIsDeterministic(t *testing.T) {
	needDomain(t)
	export := func() []byte {
		e := newEnv()
		k := newStub(qd.Camp)
		k.rewards = []qd.Reward{{Goodwill: 5}, {Lines: []trade.Line{{Kind: 1, Qty: 2}}}}
		q := New(e.roles(), []Kind{k, newStub(qd.Request)})
		run(q, e, 0, 100*day)
		return gobBytes(t, q.Export())
	}
	if a, b := export(), export(); !bytes.Equal(a, b) {
		t.Fatal("two runs from one seed differ")
	}
}

func gobBytes(t *testing.T, s *Save) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(s); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExportImportRoundTrip(t *testing.T) {
	needDomain(t)
	build := func() (*env, *Quests, *stubKind) {
		e := newEnv()
		k := newStub(qd.Camp)
		k.rewards = []qd.Reward{{Goodwill: 5}, {Lines: []trade.Line{{Kind: 1, Qty: 2}}, Value: 20}}
		k.partIn = 3 * day
		return e, New(e.roles(), []Kind{k}), k
	}
	e, q, _ := build()
	run(q, e, 0, 40*day) // a few offers, some expired
	id := offerOne(t, e, q, qd.Camp, 40*day)
	if err := q.Apply(Accept(id, 1)); err != nil {
		t.Fatal(err)
	}
	q.Apply(Dismiss(id))
	q.Apply(SetPref(1, false))
	s := q.Export()
	raw := gobBytes(t, s)
	var back Save
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&back); err != nil {
		t.Fatal(err)
	}
	e2, q2, _ := build()
	*e2.rolls = *e.rolls
	v := q2.Version()
	q2.Import(&back)
	if q2.Version() == v {
		t.Fatal("Import did not move the version")
	}
	if !bytes.Equal(gobBytes(t, q2.Export()), raw) {
		t.Fatal("export after import differs")
	}
	if !reflect.DeepEqual(q.View(), q2.View()) {
		t.Fatal("views differ after the round trip")
	}
	// Both go on identically: timers, offers and relation checks.
	e.gw.rel[0], e2.gw.rel[0] = faction.Hostile, faction.Hostile
	run(q, e, 40*day+hour, 80*day)
	run(q2, e2, 40*day+hour, 80*day)
	if !bytes.Equal(gobBytes(t, q.Export()), gobBytes(t, q2.Export())) {
		t.Fatal("the loaded root diverged")
	}
	// The export shares nothing with the root.
	s.Quests[0].Parts[0].Tile = -99
	if n, _ := q.Get(s.Quests[0].ID); n.Parts[0].Tile == -99 {
		t.Fatal("Export aliases the root's parts")
	}
}

func TestImportNilAndEmptyExport(t *testing.T) {
	e := newEnv()
	q := New(e.roles(), []Kind{newStub(qd.Camp)})
	if q.Export() != nil {
		t.Fatal("a new root exports state")
	}
	q.Import(&Save{Quests: []qd.Quest{{ID: 3, State: qd.Succeeded}, {ID: 2, State: qd.Expired}, {ID: 3}, {}}, Next: 1})
	if v := q.View(); len(v) != 2 || v[0].ID != 2 || v[1].ID != 3 {
		t.Fatalf("import sorted and de-duplicated: %+v", v)
	}
	if s := q.Export(); s.Next != 3 {
		t.Fatalf("next = %d, want the highest id", s.Next)
	}
	q.Import(nil)
	if q.Export() != nil || len(q.View()) != 0 {
		t.Fatal("Import(nil) left state")
	}
}
