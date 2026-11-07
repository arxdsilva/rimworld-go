package quest

import (
	"errors"
	"testing"

	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
)

// offerOne offers one quest of stub's kind at tick now.
func offerOne(t *testing.T, e *env, q *Quests, k qd.KindID, now uint64) qd.ID {
	t.Helper()
	e.clock.now = now
	id, ok := q.Offer(k)
	if !ok {
		t.Fatalf("Offer(%d) refused", k)
	}
	return id
}

func TestOfferAcceptSucceedPaysOnce(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Camp)
	k.rewards = []qd.Reward{{Lines: []trade.Line{{Kind: 3, Qty: 5}}, Value: 50}, {Goodwill: 12}}
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Camp, 1000)

	if n := stateOf(t, q, id); n.State != qd.Offered || n.AcceptBy != 1000+6*day || n.Stars < 1 || n.Points != 500 {
		t.Fatalf("offer = %+v", n)
	}
	if got := e.letters.list; len(got) != 1 || got[0].Kind != LetterOffer || got[0].Tile != 42 {
		t.Fatalf("letters = %+v", got)
	}
	// Two options: two Accept rows, then View and Close.
	acts := e.letters.list[0].Actions
	if len(acts) != 4 || acts[0].Kind != ActAccept || acts[1].Choice != 1 || acts[2].Kind != ActView || acts[3].Kind != ActClose {
		t.Fatalf("offer actions = %+v", acts)
	}
	e.clock.now = 2000
	if err := q.Apply(Accept(id, 1)); err != nil {
		t.Fatal(err)
	}
	if n := stateOf(t, q, id); n.State != qd.Active || n.Chosen != 1 || n.Paid || k.accepted != 1 {
		t.Fatalf("accepted = %+v (Accepted ran %d)", n, k.accepted)
	}
	q.Signal(Signal{Kind: SigCaravanArrived, Tile: 42, Caravan: 9})
	n := stateOf(t, q, id)
	if n.State != qd.Succeeded || !n.Paid || k.ended != 1 {
		t.Fatalf("succeeded = %+v, ended %d", n, k.ended)
	}
	if len(e.gw.adjusted) != 1 || e.gw.adjusted[0] != [2]int{0, 12} || len(e.items.pods) != 0 {
		t.Fatalf("paid goodwill %v pods %v", e.gw.adjusted, e.items.pods)
	}
	if e.letters.of(LetterSucceeded) != 1 {
		t.Fatalf("no success letter: %+v", e.letters.list)
	}
	// Terminal: no longer indexed, no more signals, no second payment.
	q.Signal(Signal{Kind: SigCaravanArrived, Tile: 42})
	if k.signals[id] != 1 || len(e.gw.adjusted) != 1 {
		t.Fatalf("a terminal quest got a signal (%d) or paid again", k.signals[id])
	}
	if err := q.Apply(Accept(id, 0)); err == nil {
		t.Fatal("accepting a succeeded quest worked")
	}
}

func TestSucceedPaysItemsHomeOrIntoCaravan(t *testing.T) {
	needDomain(t)
	e := newEnv()
	home := newStub(qd.Stash)
	home.def.AutoAccept, home.def.AcceptTicks = true, 0
	home.rewards = []qd.Reward{{Lines: []trade.Line{{Kind: 2, Qty: 1}}}}
	req := newStub(qd.Request)
	req.def.Delivery = qd.IntoCaravan
	req.tile = 77
	req.rewards = []qd.Reward{{Lines: []trade.Line{{Kind: 4, Qty: 2}}}}
	site := newStub(qd.OreLump)
	site.def.Delivery, site.def.AutoAccept = qd.AtSite, true
	site.tile = 88
	site.rewards = []qd.Reward{{Lines: []trade.Line{{Kind: 5, Qty: 9}}, SiteLoot: true}}
	q := New(e.roles(), []Kind{home, req, site})

	a := offerOne(t, e, q, qd.Stash, 10)
	if n := stateOf(t, q, a); n.State != qd.Active || home.accepted != 1 {
		t.Fatalf("auto-accept = %+v", n)
	}
	if acts := e.letters.list[0].Actions; len(acts) != 2 || acts[0].Kind != ActView {
		t.Fatalf("an active quest's letter offers accept: %+v", acts)
	}
	q.Signal(Signal{Kind: SigCaravanArrived, Tile: 42, Caravan: 3})
	if len(e.items.pods) != 1 || len(e.items.into) != 0 {
		t.Fatalf("home delivery: pods %v into %v", e.items.pods, e.items.into)
	}

	b := offerOne(t, e, q, qd.Request, 20)
	if err := q.Apply(Accept(b, 0)); err != nil {
		t.Fatal(err)
	}
	if err := q.Apply(Fulfil(b, 31)); err != nil {
		t.Fatal(err)
	}
	if n := stateOf(t, q, b); n.State != qd.Succeeded || len(e.items.into) != 1 || e.items.into[0] != 31 {
		t.Fatalf("into caravan: %+v into %v", n, e.items.into)
	}
	if req.lastSig.Kind != SigFulfil || req.lastSig.Caravan != 31 {
		t.Fatalf("fulfil reached the kind as %+v", req.lastSig)
	}

	c := offerOne(t, e, q, qd.OreLump, 30)
	q.Signal(Signal{Kind: SigCaravanArrived, Tile: 88, Caravan: 4})
	if n := stateOf(t, q, c); n.State != qd.Succeeded || !n.Paid || len(e.items.pods) != 1 || len(e.items.into) != 1 {
		t.Fatalf("at-site reward moved items: %+v pods %d into %d", n, len(e.items.pods), len(e.items.into))
	}
}

func TestFailTimeoutPostsLetter(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Camp)
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Camp, 0)
	if err := q.Apply(Accept(id, -1)); err != nil {
		t.Fatal(err)
	}
	q.Hourly(17*day - hour)
	if stateOf(t, q, id).State != qd.Active {
		t.Fatal("failed before the deadline")
	}
	q.Hourly(17 * day)
	n := stateOf(t, q, id)
	if n.State != qd.Failed || n.Reason != qd.Timeout || n.EndedAt != 17*day || k.ended != 1 {
		t.Fatalf("timeout = %+v ended %d", n, k.ended)
	}
	last := e.letters.list[len(e.letters.list)-1]
	if last.Kind != LetterFailed || last.Reason != qd.Timeout || last.Quest != id || len(last.Actions) != 2 {
		t.Fatalf("failure letter = %+v", last)
	}
}

func TestGiverHostileFailsOnlyKindsThatSaySo(t *testing.T) {
	needDomain(t)
	e := newEnv()
	strict := newStub(qd.Request)
	strict.def.FailOnGiverHostile = true
	lax := newStub(qd.Stash)
	offered := newStub(qd.Joiner)
	offered.def.FailOnGiverHostile = true
	q := New(e.roles(), []Kind{strict, lax, offered})
	a := offerOne(t, e, q, qd.Request, 0)
	b := offerOne(t, e, q, qd.Stash, 0)
	c := offerOne(t, e, q, qd.Joiner, 0)
	for _, id := range []qd.ID{a, b} {
		if err := q.Apply(Accept(id, -1)); err != nil {
			t.Fatal(err)
		}
	}
	q.Hourly(hour) // takes the snapshot: nothing changed
	if stateOf(t, q, a).State != qd.Active || stateOf(t, q, c).State != qd.Offered {
		t.Fatal("the first snapshot failed a quest")
	}
	letters := len(e.letters.list)
	e.gw.rel[0] = faction.Hostile
	q.Hourly(2 * hour)
	if n := stateOf(t, q, a); n.State != qd.Failed || n.Reason != qd.GiverHostile {
		t.Fatalf("strict kind = %+v", n)
	}
	if n := stateOf(t, q, b); n.State != qd.Active {
		t.Fatalf("lax kind failed: %+v", n)
	}
	// An offer from a hostile giver expires with the reason: no letter.
	if n := stateOf(t, q, c); n.State != qd.Expired || n.Reason != qd.GiverHostile {
		t.Fatalf("offered quest = %+v", n)
	}
	if got := len(e.letters.list) - letters; got != 1 || e.letters.list[letters].Reason != qd.GiverHostile {
		t.Fatalf("letters after the change: %+v", e.letters.list[letters:])
	}
	// No change: no relation re-check fails anything else.
	q.Hourly(3 * hour)
	if stateOf(t, q, b).State != qd.Active {
		t.Fatal("lax kind failed later")
	}
}

func TestTargetNoLongerHostileFails(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Camp)
	k.def.HasTarget = true
	k.target = 2 // faction index 1, hostile
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Camp, 0)
	if err := q.Apply(Accept(id, -1)); err != nil {
		t.Fatal(err)
	}
	q.Hourly(hour)
	if stateOf(t, q, id).State != qd.Active {
		t.Fatal("failed while the target is hostile")
	}
	e.gw.rel[1] = faction.Neutral
	q.Hourly(2 * hour)
	if n := stateOf(t, q, id); n.State != qd.Failed || n.Reason != qd.TargetNotHostile {
		t.Fatalf("target calm = %+v", n)
	}
	// A kind whose target is already calm is not offered at all.
	if _, ok := q.Offer(qd.Camp); ok {
		t.Fatal("offered a camp against a faction that is not hostile")
	}
}

func TestKindSignalFails(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Refugee)
	k.def.AutoAccept = true
	k.pawn = 77
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Refugee, 0)
	q.Signal(Signal{Kind: SigPawnDied, Pawn: 78})
	if k.signals[id] != 0 {
		t.Fatal("a signal for another pawn reached the quest")
	}
	q.Signal(Signal{Kind: SigPawnDied, Pawn: 77})
	n := stateOf(t, q, id)
	if n.State != qd.Failed || n.Reason != qd.PawnLost || k.ended != 1 {
		t.Fatalf("pawn died = %+v", n)
	}
	if e.letters.list[len(e.letters.list)-1].Reason != qd.PawnLost {
		t.Fatal("the failure letter does not name the reason")
	}
}

func TestOfferExpiresWithoutLetter(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Joiner)
	k.def.AcceptTicks = 7 * hour
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Joiner, 0)
	q.Hourly(6 * hour)
	if stateOf(t, q, id).State != qd.Offered {
		t.Fatal("expired early")
	}
	q.Hourly(7 * hour)
	n := stateOf(t, q, id)
	if n.State != qd.Expired || n.Reason != qd.Timeout || k.ended != 1 {
		t.Fatalf("expiry = %+v", n)
	}
	if len(e.letters.list) != 1 {
		t.Fatalf("expiry posted a letter: %+v", e.letters.list)
	}
}

func TestDismissRestoreAndAcceptDismissed(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Camp)
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Camp, 0)
	v0 := q.Version()
	if err := q.Apply(Dismiss(id)); err != nil {
		t.Fatal(err)
	}
	if n := stateOf(t, q, id); !n.Dismissed || n.State != qd.Offered || n.AcceptBy != 6*day {
		t.Fatalf("dismissed = %+v", n)
	}
	if q.Version() == v0 || !q.View()[0].Dismissed {
		t.Fatal("dismiss did not reach the read model")
	}
	if err := q.Apply(Accept(id, -1)); err != nil {
		t.Fatalf("accepting a dismissed offer: %v", err)
	}
	if n := stateOf(t, q, id); n.State != qd.Active || !n.Dismissed {
		t.Fatalf("accepted dismissed = %+v", n)
	}
	if err := q.Apply(Restore(id)); err != nil {
		t.Fatal(err)
	}
	if stateOf(t, q, id).Dismissed {
		t.Fatal("restore left it dismissed")
	}
}

func TestOnAcceptRewardIsNeverTakenBack(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Joiner)
	k.def.RewardAt = qd.OnAccept
	k.def.FailOnGiverHostile = true
	k.rewards = []qd.Reward{{Pawn: 555}}
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Joiner, 0)
	if v := q.View(); len(v[0].Rewards) != 1 || !v[0].Rewards[0].HasPawn || v[0].Rewards[0].Pawn.Name != "pawn555" {
		t.Fatalf("pawn preview = %+v", v[0].Rewards)
	}
	if err := q.Apply(Accept(id, 0)); err != nil {
		t.Fatal(err)
	}
	if n := stateOf(t, q, id); !n.Paid || len(e.pawns.walked) != 1 || e.pawns.walked[0] != 555 {
		t.Fatalf("on-accept pay: %+v walked %v", n, e.pawns.walked)
	}
	q.Hourly(hour)
	e.gw.rel[0] = faction.Hostile
	q.Hourly(2 * hour)
	if n := stateOf(t, q, id); n.State != qd.Failed || !n.Paid || len(e.pawns.walked) != 1 {
		t.Fatalf("after failure: %+v walked %v", n, e.pawns.walked)
	}
}

func TestPartTimersFireOnceUnlessRearmed(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Joiner)
	k.def.AutoAccept, k.def.DurationTicks = true, 0
	k.partIn = 5 * hour
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Joiner, 0)
	q.Hourly(4 * hour)
	if k.partCalls != 0 {
		t.Fatal("part fired early")
	}
	q.Hourly(5 * hour)
	q.Hourly(6 * hour)
	if k.partCalls != 1 || !stateOf(t, q, id).Parts[1].Done {
		t.Fatalf("part calls %d, parts %+v", k.partCalls, stateOf(t, q, id).Parts)
	}
	// A re-armed part fires again at its new tick; a verdict ends it.
	k2 := newStub(qd.Talks)
	k2.def.AutoAccept, k2.def.DurationTicks = true, 0
	k2.partIn, k2.rearm, k2.partVerdict = hour, 2*hour, VerdictNone
	q2 := New(e.roles(), []Kind{k2})
	id2 := offerOne(t, e, q2, qd.Talks, 0)
	for h := uint64(1); h <= 5; h++ {
		q2.Hourly(h * hour)
	}
	if k2.partCalls != 3 { // hours 1, 3, 5
		t.Fatalf("re-armed part fired %d times", k2.partCalls)
	}
	k2.partVerdict = VerdictFail(qd.SiteLost)
	q2.Hourly(7 * hour)
	if n := stateOf(t, q2, id2); n.State != qd.Failed || n.Reason != qd.SiteLost {
		t.Fatalf("part verdict = %+v", n)
	}
}

func TestApplyErrors(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Camp)
	k.rewards = []qd.Reward{{Goodwill: 3}}
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Camp, 0)
	cases := []struct {
		c    Cmd
		want error
	}{
		{Accept(99, 0), ErrNoQuest},
		{Dismiss(99), ErrNoQuest},
		{Cmd{Op: OpNone, Quest: id}, ErrOp},
		{Cmd{Op: OpSetPref + 1, Quest: id}, ErrOp},
		{Fulfil(id, 1), qd.ErrMove},
		{Negotiate(id, 1), qd.ErrMove},
		{Accept(id, 5), qd.ErrChoice},
	}
	for _, c := range cases {
		if err := q.Apply(c.c); !errors.Is(err, c.want) {
			t.Errorf("Apply(%+v) = %v, want %v", c.c, err, c.want)
		}
	}
	if err := q.Apply(Accept(id, 0)); err != nil {
		t.Fatal(err)
	}
	if err := q.Apply(Negotiate(id, 6)); err != nil || k.lastSig.Kind != SigNegotiate || k.lastSig.Caravan != 6 {
		t.Fatalf("negotiate: %v, sig %+v", err, k.lastSig)
	}
	// A quest whose kind is gone: kept, but commands that need it fail.
	q2 := New(e.roles(), nil)
	q2.Import(q.Export())
	if err := q2.Apply(Fulfil(id, 1)); !errors.Is(err, ErrNoKind) {
		t.Fatalf("fulfil without kind = %v", err)
	}
}

func TestSignalRoutesOnlyToIndexedQuests(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Stash)
	k.def.AutoAccept = true
	q := New(e.roles(), []Kind{k})
	ids := map[int]qd.ID{}
	for tile := 1; tile <= 50; tile++ {
		k.tile = tile
		ids[tile] = offerOne(t, e, q, qd.Stash, 0)
	}
	// A non-tile signal kind ignores Tile; a zero signal reaches nobody.
	q.Signal(Signal{Kind: SigPawnDied, Tile: 7})
	q.Signal(Signal{})
	if len(k.signals) != 0 {
		t.Fatalf("signals reached %v", k.signals)
	}
	q.Signal(Signal{Kind: SigSiteOpened, Tile: 7, Map: 3})
	if len(k.signals) != 1 || k.signals[ids[7]] != 1 {
		t.Fatalf("tile 7 reached %v", k.signals)
	}
	// By faction: every quest of giver 1.
	q.Signal(Signal{Kind: SigFactionDefeated, Faction: 1})
	if len(k.signals) != 50 {
		t.Fatalf("faction signal reached %d quests", len(k.signals))
	}
	// Directly by id, and routing follows a part's new map.
	q.Signal(Signal{Kind: SigSiteCensus, Quest: ids[9]})
	if k.signals[ids[9]] != 2 {
		t.Fatalf("direct signal: %v", k.signals[ids[9]])
	}
}

func TestSignalFollowsPartsAKindChanges(t *testing.T) {
	needDomain(t)
	e := newEnv()
	k := newStub(qd.Camp)
	k.def.AutoAccept = true
	q := New(e.roles(), []Kind{k})
	id := offerOne(t, e, q, qd.Camp, 0)
	// The kind binds a map to its site when it opens.
	setMap := &mapKind{stubKind: k}
	q2 := New(e.roles(), []Kind{setMap})
	q2.Import(q.Export())
	q2.Signal(Signal{Kind: SigSiteOpened, Tile: 42, Map: 12})
	q2.Signal(Signal{Kind: SigSiteCensus, Map: 12})
	if k.signals[id] != 2 {
		t.Fatalf("census by map reached the quest %d times", k.signals[id])
	}
	q2.Signal(Signal{Kind: SigMapClosed, Map: 12})
	if n := stateOf(t, q2, id); n.State != qd.Succeeded {
		t.Fatalf("map closed: %+v", n)
	}
}

// mapKind binds the opened map to the site part and succeeds on close.
type mapKind struct{ *stubKind }

func (m *mapKind) Signal(c Ctx, q *qd.Quest, s Signal) Verdict {
	m.signals[q.ID]++
	switch s.Kind {
	case SigSiteOpened:
		q.Parts[0].Map = s.Map
	case SigMapClosed:
		return VerdictSucceed
	}
	return VerdictNone
}

func TestSetPrefAndCtx(t *testing.T) {
	e := newEnv()
	k := newStub(qd.Camp)
	q := New(e.roles(), []Kind{k})
	c := q.ctx(0)
	if !c.AcceptsGoodwill(1) || !q.AcceptsGoodwill(1) {
		t.Fatal("goodwill is accepted by default")
	}
	v := q.Version()
	if err := q.Apply(SetPref(1, false)); err != nil {
		t.Fatal(err)
	}
	if c.AcceptsGoodwill(1) || q.AcceptsGoodwill(1) || !q.AcceptsGoodwill(2) || q.Version() == v {
		t.Fatal("SetPref off did not hold")
	}
	q.Apply(SetPref(1, false)) // no change
	q.Apply(SetPref(0, false)) // no faction: ignored
	if s := q.Export(); s == nil || len(s.NoGoodwill) != 1 || s.NoGoodwill[0] != 0 {
		t.Fatalf("export = %+v", s)
	}
	q.Apply(SetPref(1, true))
	if !q.AcceptsGoodwill(1) {
		t.Fatal("SetPref on did not hold")
	}
	if c.Intn(0) != 0 || e.rolls.draws != 0 {
		t.Fatal("Intn(0) drew a roll")
	}
	if Ctx.AcceptsGoodwill(Ctx{}, 1) != true {
		t.Fatal("a bare Ctx declines goodwill")
	}
}

func TestVerdictEncoding(t *testing.T) {
	if VerdictNone.Succeeds() || VerdictNone.Fails() {
		t.Fatal("none")
	}
	if !VerdictSucceed.Succeeds() || VerdictSucceed.Fails() {
		t.Fatal("succeed")
	}
	v := VerdictFail(qd.SiteLost)
	if !v.Fails() || v.Succeeds() || v.Reason() != qd.SiteLost {
		t.Fatalf("fail = %v", v)
	}
}

func TestNewIgnoresBadKinds(t *testing.T) {
	e := newEnv()
	q := New(e.roles(), []Kind{nil, newStub(qd.KindNone), newStub(qd.NumKinds)})
	for k := range qd.NumKinds {
		if q.kinds[k] != nil {
			t.Fatalf("kind %d registered", k)
		}
	}
	if _, ok := q.Offer(qd.NumKinds); ok {
		t.Fatal("offered an unknown kind")
	}
	if _, ok := q.Offer(qd.Camp); ok {
		t.Fatal("offered an unregistered kind")
	}
	off := newStub(qd.Camp)
	off.off = true
	q = New(e.roles(), []Kind{off})
	if _, ok := q.Offer(qd.Camp); ok || off.built != 0 {
		t.Fatal("offered a kind that may not be offered")
	}
	nob := newStub(qd.Camp)
	nob.build = func(Ctx, *qd.Quest) bool { return false }
	q = New(e.roles(), []Kind{nob})
	if _, ok := q.Offer(qd.Camp); ok || q.Export() != nil {
		t.Fatal("a failed build issued a quest")
	}
}

func TestQuestWithoutKindStillRunsItsTimers(t *testing.T) {
	needDomain(t)
	e := newEnv()
	q := New(e.roles(), nil)
	q.Import(&Save{Next: 2, Quests: []qd.Quest{
		{ID: 1, Kind: qd.Camp, State: qd.Offered, AcceptBy: 2 * hour, Giver: 1, Chosen: -1,
			Parts: []qd.Part{{Kind: qd.PartSite, Tile: 4}}},
		{ID: 2, Kind: qd.Stash, State: qd.Active, Deadline: 9 * hour, Chosen: -1,
			Parts: []qd.Part{{Kind: qd.PartThreat, Tile: -1, At: 3 * hour}}},
	}})
	q.Signal(Signal{Kind: SigCaravanArrived, Tile: 4})
	run(q, e, hour, 10*hour)
	if n := stateOf(t, q, 1); n.State != qd.Expired {
		t.Fatalf("offer without kind = %+v", n)
	}
	n := stateOf(t, q, 2)
	if n.State != qd.Failed || n.Reason != qd.Timeout || !n.Parts[0].Done {
		t.Fatalf("active without kind = %+v", n)
	}
	if v := q.View(); v[0].Detail != nil || v[0].Tile != 4 || v[1].Tile != -1 {
		t.Fatalf("view = %+v", v)
	}
	if _, ok := q.Get(9); ok {
		t.Fatal("Get found a quest that does not exist")
	}
}

func TestCtxPointsAndRewards(t *testing.T) {
	e := newEnv()
	k := newStub(qd.Camp)
	k.def.Choice = true
	q := New(e.roles(), []Kind{k})
	c := q.ctx(0)
	n := qd.Quest{Kind: qd.Camp, Points: 500, Stars: 2, Giver: 1}
	for range 50 {
		if p := c.ThreatPoints(n); p < 700 || p > 1300 {
			t.Fatalf("threat points %d outside 700..1300 at 2 stars", p)
		}
	}
	pool := []qd.PoolItem{{Kind: 1}, {Kind: 2}}
	q.Apply(SetPref(1, false))
	for range 50 {
		opts := c.Rewards(n, pool, 0, false)
		if len(opts) != 1 && len(opts) != 3 {
			t.Fatalf("%d reward options", len(opts))
		}
		for _, o := range opts {
			if o.Goodwill != 0 {
				t.Fatalf("goodwill offered after the player declined it: %+v", o)
			}
		}
	}
	if (Ctx{R: e.roles()}).Rewards(n, pool, 0, false) == nil {
		t.Fatal("a bare Ctx picked no rewards")
	}
	if ErrNoQuest.Error() != string(ErrNoQuest) {
		t.Fatal("error text")
	}
}
