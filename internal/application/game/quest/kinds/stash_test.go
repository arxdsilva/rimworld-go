package kinds

import (
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
)

func offerStash(t *testing.T, e *env) (*quest.Quests, qd.ID) {
	t.Helper()
	q := newRoot(e, Stash{})
	id, ok := q.Offer(qd.Stash)
	if !ok {
		t.Fatal("stash not offered")
	}
	return q, id
}

func TestStashOfferIsActiveWithTheGoodsAsItsReward(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerStash(t, e)
	n := get(t, q, id)
	if n.State != qd.Active || n.Deadline != e.clock.now+16*day || n.Target == 0 || n.Target == 1 {
		t.Fatalf("quest = %+v", n)
	}
	if f := e.world.factions[n.Target-1]; f.Relation != faction.Hostile {
		t.Fatalf("guard faction %d is not hostile", n.Target)
	}
	if n.Giver != 1 {
		t.Fatalf("giver = %d, want the only non-hostile faction (ref 1)", n.Giver)
	}
	if len(n.Rewards) != 1 || len(n.Rewards[0].Lines) == 0 || n.Rewards[0].Goodwill != 0 || n.Rewards[0].Pawn != 0 {
		t.Fatalf("rewards = %+v, want one items option", n.Rewards)
	}
	if len(n.Parts) != 1 || n.Parts[0].Kind != qd.PartSite || n.Parts[0].Tile != 40 || n.Parts[0].Points <= 0 {
		t.Fatalf("parts = %+v", n.Parts)
	}
	if len(e.sites.placed) != 0 || len(e.builder.goods) != 0 {
		t.Fatal("nothing is laid before the site opens")
	}
	d := q.View()[0].Detail
	if len(d) < 3 || d[0].Key != "quest.stash.site" || d[1].Key != "quest.stash.deadline" || d[2].Key != "quest.stash.item" {
		t.Fatalf("detail = %+v", d)
	}
}

func TestStashRefusedWithoutThreatsOrGuards(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.world.threats = false
	if _, ok := newRoot(e, Stash{}).Offer(qd.Stash); ok {
		t.Fatal("offered with quest threats off")
	}
	e = newEnv()
	e.world.factions[1].Defeated, e.world.factions[2].Settlements = true, nil
	if _, ok := newRoot(e, Stash{}).Offer(qd.Stash); ok {
		t.Fatal("offered without a hostile faction to guard it")
	}
	e = newEnv()
	e.world.near = -1
	if _, ok := newRoot(e, Stash{}).Offer(qd.Stash); ok {
		t.Fatal("offered without a free tile")
	}
	e = newEnv()
	for i := range e.world.factions {
		e.world.factions[i].Relation = faction.Hostile
	}
	q, id := offerStash(t, e)
	if get(t, q, id).Giver != 0 {
		t.Fatal("a giver without a non-hostile faction")
	}
}

func TestStashOpenLaysGoodsAndGuardsAndSucceeds(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerStash(t, e)
	n := get(t, q, id)
	openSite(e, q, id, 40, 7)
	n = get(t, q, id)
	if n.State != qd.Succeeded || !n.Parts[0].Done {
		t.Fatalf("after the open: %+v", n)
	}
	b := e.builder
	if len(b.goods) != 1 || !b.goods[0].Forbid || len(b.goods[0].Lines) != len(n.Rewards[0].Lines) ||
		b.goods[0].X != 75 || b.goods[0].Y != 75 {
		t.Fatalf("goods = %+v", b.goods)
	}
	if len(b.camps) != 1 || len(b.camps[0].Posts) != stashRing || len(b.camps[0].Pieces) != 0 || len(b.camps[0].Loot) != 0 {
		t.Fatalf("camps = %+v", b.camps)
	}
	if len(b.arms[0]) == 0 || len(b.arms[0]) > stashGuardMax || n.Parts[0].Count != len(b.arms[0]) {
		t.Fatalf("arms = %v, count %d", b.arms[0], n.Parts[0].Count)
	}
	if len(e.items.pods) != 0 || len(e.items.into) != 0 {
		t.Fatal("an at-site reward drops nothing")
	}
	if n.Paid != true {
		t.Fatal("paid")
	}
	var k Stash
	if k.Signal(quest.Ctx{}, &n, quest.Signal{Kind: quest.SigSiteOpened, Tile: 40}) != quest.VerdictNone {
		t.Fatal("a finished quest reacts")
	}
}

func TestStashOpenOnAnotherTileDoesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerStash(t, e)
	openSite(e, q, id, 41, 7)
	if get(t, q, id).State != qd.Active || len(e.builder.goods) != 0 {
		t.Fatal("a foreign tile laid the site")
	}
}

func TestStashSucceedsWhenTheBuilderRefusesGuards(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerStash(t, e)
	e.builder.refuse = true
	openSite(e, q, id, 40, 7)
	n := get(t, q, id)
	if n.State != qd.Succeeded || len(e.builder.goods) != 1 || n.Parts[0].Count != 0 {
		t.Fatalf("goods are down so it succeeds: %+v goods %d", n, len(e.builder.goods))
	}
}

func TestStashWithoutGoodsFails(t *testing.T) {
	t.Parallel()
	e := newEnv()
	var k Stash
	n := qd.Quest{State: qd.Active, Parts: []qd.Part{{Kind: qd.PartSite, Tile: 40}}, Rewards: []qd.Reward{{}}}
	c := quest.Ctx{R: e.roles()}
	v := k.Signal(c, &n, quest.Signal{Kind: quest.SigSiteOpened, Tile: 40})
	if !v.Fails() || v.Reason() != qd.SiteLost || len(e.builder.goods) != 0 {
		t.Fatalf("verdict %v, goods %d", v, len(e.builder.goods))
	}
	if k.Detail(qd.Quest{}) != nil {
		t.Fatal("detail of an empty quest")
	}
}

func TestStashTimesOutAndLosesItsSite(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerStash(t, e)
	run(e, q, e.clock.now+15*day)
	if get(t, q, id).State != qd.Active {
		t.Fatal("ended early")
	}
	run(e, q, e.clock.now+2*day)
	if n := get(t, q, id); n.State != qd.Failed || n.Reason != qd.Timeout {
		t.Fatalf("after 16 days: %+v", n)
	}
	e = newEnv()
	q, id = offerStash(t, e)
	q.Signal(quest.Signal{Kind: quest.SigSiteLost, Tile: 40})
	if n := get(t, q, id); n.State != qd.Failed || n.Reason != qd.SiteLost {
		t.Fatalf("site lost: %+v", n)
	}
	e = newEnv()
	q, id = offerStash(t, e)
	q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: id, Map: 3})
	if n := get(t, q, id); n.State != qd.Failed {
		t.Fatalf("map closed unlaid: %+v", n)
	}
}

func TestStashDeterministic(t *testing.T) {
	t.Parallel()
	a, b := newEnv(), newEnv()
	qa, ia := offerStash(t, a)
	qb, ib := offerStash(t, b)
	openSite(a, qa, ia, 40, 7)
	openSite(b, qb, ib, 40, 7)
	na, nb := get(t, qa, ia), get(t, qb, ib)
	if na.Seed != nb.Seed || na.Target != nb.Target || len(na.Rewards[0].Lines) != len(nb.Rewards[0].Lines) ||
		len(a.builder.arms[0]) != len(b.builder.arms[0]) {
		t.Fatal("same seed, different stash")
	}
}

func TestStashIgnoresTheCensusAndOtherSignals(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerStash(t, e)
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 40, Caravan: 1})
	q.Signal(quest.Signal{Kind: quest.SigSiteCensus, Quest: id, Count: 3})
	var k Stash
	n := get(t, q, id)
	if n.State != qd.Active || k.PartDue(quest.Ctx{}, &n, 0) != quest.VerdictNone {
		t.Fatalf("quest = %+v", n)
	}
	k.Accepted(quest.Ctx{}, &n)
	k.Ended(quest.Ctx{}, &n)
}
