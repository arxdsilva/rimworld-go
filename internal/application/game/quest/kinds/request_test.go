package kinds

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
)

// offerRequest offers a request and accepts it with reward option 0.
func offerRequest(t *testing.T, e *env) (*quest.Quests, qd.Quest) {
	t.Helper()
	q := newRoot(e, Request{})
	id, ok := q.Offer(qd.Request)
	if !ok {
		t.Fatal("request not offered")
	}
	return q, get(t, q, id)
}

func accept(t *testing.T, q *quest.Quests, id qd.ID) {
	t.Helper()
	if err := q.Apply(quest.Accept(id, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestRequestBuildAndSuccess(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, n := offerRequest(t, e)
	if n.State != qd.Offered || n.Giver != 1 || len(n.Parts) != 1 || n.Parts[0].Kind != qd.PartDelivery {
		t.Fatalf("bad offer %+v", n)
	}
	p := n.Parts[0]
	if p.Tile != 100 || len(p.Lines) != 1 || p.Lines[0].Qty < 1 || len(n.Rewards) == 0 {
		t.Fatalf("bad part %+v rewards %d", p, len(n.Rewards))
	}
	lines := qd.Value(p.Lines)
	if lines < 100 || lines > 7000 {
		t.Fatalf("requested value %d", lines)
	}
	accept(t, q, n.ID)
	// Arriving alone does nothing; so does Fulfil from a moving caravan.
	e.items.has = true
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: p.Tile, Caravan: 4})
	if err := q.Apply(quest.Fulfil(n.ID, 4)); err != nil || get(t, q, n.ID).State != qd.Active {
		t.Fatalf("moving caravan fulfilled: %v", err)
	}
	// At the wrong tile, or without the goods: nothing changes.
	e.items.at[4] = 99
	_ = q.Apply(quest.Fulfil(n.ID, 4))
	e.items.at[4], e.items.has = p.Tile, false
	_ = q.Apply(quest.Fulfil(n.ID, 4))
	if get(t, q, n.ID).State != qd.Active || len(e.items.taken) != 0 || len(e.gw.adjusted) != 0 {
		t.Fatal("a failed hand-over changed something")
	}
	e.items.has = true
	if err := q.Apply(quest.Fulfil(n.ID, 4)); err != nil {
		t.Fatal(err)
	}
	g := get(t, q, n.ID)
	if g.State != qd.Succeeded || !g.Paid || !g.Parts[0].Done {
		t.Fatalf("not succeeded: %+v", g)
	}
	if len(e.items.taken) != 1 {
		t.Fatalf("taken %v", e.items.taken)
	}
	if !reflect.DeepEqual(e.items.taken[0], p.Lines) {
		t.Fatalf("taken %v want %v", e.items.taken[0], p.Lines)
	}
	found := false
	for _, a := range e.gw.adjusted {
		found = found || a == [2]int{0, 12}
	}
	if !found {
		t.Fatalf("no +12 goodwill: %v", e.gw.adjusted)
	}
	if opt := g.Rewards[g.Chosen]; len(opt.Lines) > 0 && len(e.items.into[4]) == 0 {
		t.Fatal("item reward not into the caravan")
	}
	// A later signal on the ended quest changes nothing.
	_ = q.Apply(quest.Fulfil(n.ID, 4))
}

func TestRequestFailPaths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		do   func(e *env, q *quest.Quests, n qd.Quest)
		want qd.Reason
	}{
		{"deadline", func(e *env, q *quest.Quests, n qd.Quest) { run(e, q, n.Deadline+hour) }, qd.Timeout},
		{"hostile", func(e *env, q *quest.Quests, n qd.Quest) {
			e.gw.rel[0] = faction.Hostile
			run(e, q, e.clock.now+2*hour)
		}, qd.GiverHostile},
		{"site lost", func(e *env, q *quest.Quests, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigSiteLost, Tile: n.Parts[0].Tile})
		}, qd.SiteLost},
		{"defeated", func(e *env, q *quest.Quests, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigFactionDefeated, Faction: n.Giver})
		}, qd.SiteLost},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv()
			q, n := offerRequest(t, e)
			accept(t, q, n.ID)
			n = get(t, q, n.ID)
			tc.do(e, q, n)
			g := get(t, q, n.ID)
			if g.State != qd.Failed || g.Reason != tc.want || g.Paid {
				t.Fatalf("state %v reason %v paid %v", g.State, g.Reason, g.Paid)
			}
		})
	}
}

func TestRequestOfferExpires(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, n := offerRequest(t, e)
	run(e, q, n.AcceptBy+hour)
	if g := get(t, q, n.ID); g.State != qd.Expired {
		t.Fatalf("state %v", g.State)
	}
}

func TestRequestNoGiver(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.world.factions[0].Defeated = true
	q := newRoot(e, Request{})
	if _, ok := q.Offer(qd.Request); ok {
		t.Fatal("offered with no giver")
	}
	e.world.factions[0].Defeated = false
	e.world.factions[0].Settlements = nil
	if (Request{}).CanOffer(quest.Ctx{R: e.roles()}) {
		t.Fatal("offered without a settlement")
	}
	if (Request{}).Build(quest.Ctx{R: e.roles()}, &qd.Quest{}) {
		t.Fatal("built without a giver")
	}
}

func TestRequestDeterministic(t *testing.T) {
	t.Parallel()
	a, b := newEnv(), newEnv()
	_, x := offerRequest(t, a)
	_, y := offerRequest(t, b)
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("same seed, different quests:\n%+v\n%+v", x, y)
	}
}

func TestRequestScalesAndDetail(t *testing.T) {
	t.Parallel()
	lo, hi := newEnv(), newEnv()
	lo.world.points, hi.world.points = 100, 5000
	_, a := offerRequest(t, lo)
	_, b := offerRequest(t, hi)
	if qd.Value(a.Parts[0].Lines) > qd.Value(b.Parts[0].Lines) {
		t.Fatalf("value does not grow with points: %d %d", qd.Value(a.Parts[0].Lines), qd.Value(b.Parts[0].Lines))
	}
	d := (Request{}).Detail(b)
	if len(d) != 4 || d[0].Key != "quest.request.item" || d[3].Args[0] != quest.Faction(1) {
		t.Fatalf("detail %+v", d)
	}
	if (Request{}).Detail(qd.Quest{}) != nil {
		t.Fatal("detail of an empty quest")
	}
	k := Request{}
	k.Accepted(quest.Ctx{}, &b)
	k.Ended(quest.Ctx{}, &b)
	if k.PartDue(quest.Ctx{}, &b, 0) != quest.VerdictNone {
		t.Fatal("part due")
	}
}
