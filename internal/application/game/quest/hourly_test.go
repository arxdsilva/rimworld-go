package quest

import (
	"slices"
	"strconv"
	"testing"

	qd "rimworld-go/internal/domain/quest"
)

func TestHeapPopsInOrder(t *testing.T) {
	var h dueHeap
	r := &fakeRolls{s: 3}
	var want []entry
	for range 200 {
		e := entry{Tick: uint64(r.Intn(50)), ID: qd.ID(r.Intn(20)), Ev: qd.Event(r.Intn(3)), Part: r.Intn(3)}
		h.push(e)
		want = append(want, e)
	}
	slices.SortFunc(want, func(a, b entry) int {
		if a.before(b) {
			return -1
		}
		if b.before(a) {
			return 1
		}
		return 0
	})
	for i, w := range want {
		if got := h.pop(); got != w {
			t.Fatalf("pop %d = %+v, want %+v", i, got, w)
		}
	}
	if _, ok := h.top(); ok {
		t.Fatal("heap not empty")
	}
}

// activeQuests builds a root with n active quests whose part timers fall
// one per hour from hour 1, each re-armed n hours on (so one is due every
// hour for ever).
func activeQuests(tb testing.TB, n int) (*env, *Quests, *stubKind) {
	e := newEnv()
	k := newStub(qd.Stash)
	k.def.AutoAccept, k.def.DurationTicks = true, 0
	k.rearm = uint64(n) * hour
	s := &Save{Next: qd.ID(n)}
	for i := 1; i <= n; i++ {
		s.Quests = append(s.Quests, qd.Quest{ID: qd.ID(i), Kind: qd.Stash, State: qd.Active, Giver: 1, Chosen: -1,
			Parts: []qd.Part{{Kind: qd.PartSite, Tile: i}, {Kind: qd.PartThreat, Tile: -1, At: uint64(i) * hour}}})
	}
	q := New(e.roles(), []Kind{k})
	q.Import(s)
	return e, q, k
}

func TestHourlyTouchesOnlyDueEntries(t *testing.T) {
	needDomain(t)
	e, q, k := activeQuests(t, 100)
	q.Hourly(0) // the first snapshot checks every live quest once
	if k.partCalls != 0 {
		t.Fatal("a part fired before its hour")
	}
	v := q.Version()
	for h := uint64(1); h <= 10; h++ {
		reads := e.gw.reads
		q.Hourly(h * hour)
		if k.partCalls != int(h) {
			t.Fatalf("hour %d: %d part calls, want %d", h, k.partCalls, h)
		}
		if e.gw.reads-reads != len(e.gw.rel) {
			t.Fatalf("the relation diff read %d relations for %d factions", e.gw.reads-reads, len(e.gw.rel))
		}
	}
	if q.Version()-v != 10 {
		t.Fatalf("version moved %d times for 10 due parts", q.Version()-v)
	}
	// Due entries only: the heap holds one live entry per quest, not more.
	if len(q.heap) > 100+10 {
		t.Fatalf("heap grew to %d", len(q.heap))
	}
	// Quiet hour: no part due and no relation change moves nothing.
	k.rearm = 0
	q2 := New(e.roles(), []Kind{k})
	q2.Hourly(0)
	v2 := q2.Version()
	q2.Hourly(hour)
	if q2.Version() != v2 {
		t.Fatal("an empty hour moved the version")
	}
}

func BenchmarkQuestHourly(b *testing.B) {
	for _, n := range []int{10, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			needDomain(b)
			_, q, _ := activeQuests(b, n)
			q.Hourly(0)
			now := uint64(0)
			b.ReportAllocs()
			for b.Loop() {
				now += hour
				q.Hourly(now)
			}
		})
	}
}
