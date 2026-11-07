package kinds

import (
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	qd "rimworld-go/internal/domain/quest"
)

func oreEnv() (*env, *quest.Quests) {
	e := newEnv()
	e.sites.deposits[33] = trade.Line{Kind: item.Steel, Qty: 2300}
	return e, newRoot(e, OreLump{})
}

func TestOreLumpRefusesWithoutDeposit(t *testing.T) {
	t.Parallel()
	_, q := oreEnv()
	if _, ok := q.Offer(qd.OreLump); ok {
		t.Fatal("cadence offer accepted")
	}
	if _, ok := q.OfferAt(qd.OreLump, 34); ok {
		t.Fatal("offered without a deposit")
	}
	var k OreLump
	if k.Build(quest.Ctx{Tile: -1}, &qd.Quest{}) {
		t.Fatal("built without a tile")
	}
	e := newEnv()
	e.sites.deposits[5] = trade.Line{Kind: item.Steel}
	if _, ok := newRoot(e, OreLump{}).OfferAt(qd.OreLump, 5); ok {
		t.Fatal("empty lump offered")
	}
}

func TestOreLumpArrivalSucceeds(t *testing.T) {
	t.Parallel()
	e, q := oreEnv()
	id, ok := q.OfferAt(qd.OreLump, 33)
	if !ok {
		t.Fatal("not offered")
	}
	n := get(t, q, id)
	if n.State != qd.Active || n.Deadline != e.clock.now+30*day || len(n.Rewards) != 0 || n.Giver != 0 {
		t.Fatalf("offer %+v", n)
	}
	if len(n.Parts) != 1 || n.Parts[0].Kind != qd.PartSite || n.Parts[0].Tile != 33 || n.Parts[0].Lines[0].Qty != 2300 {
		t.Fatalf("parts %+v", n.Parts)
	}
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 34, Caravan: 1})
	if get(t, q, id).State != qd.Active {
		t.Fatal("wrong tile ended it")
	}
	run(e, q, e.clock.now+5*day)
	q.Signal(quest.Signal{Kind: quest.SigCaravanArrived, Tile: 33, Caravan: 1})
	n = get(t, q, id)
	if n.State != qd.Succeeded || !n.Parts[0].Done {
		t.Fatalf("after arrival %+v", n)
	}
	if len(e.sites.removed) != 0 {
		t.Fatalf("removed sites %v", e.sites.removed)
	}
	var k OreLump
	if k.Signal(quest.Ctx{}, &n, quest.Signal{Kind: quest.SigCaravanArrived, Tile: 33}) != quest.VerdictNone {
		t.Fatal("second arrival counted")
	}
	if k.Signal(quest.Ctx{}, &qd.Quest{}, quest.Signal{Kind: quest.SigCaravanArrived}) != quest.VerdictNone {
		t.Fatal("no parts")
	}
	k.Accepted(quest.Ctx{}, &n)
	k.Ended(quest.Ctx{}, &n)
	if k.PartDue(quest.Ctx{}, &n, 0) != quest.VerdictNone {
		t.Fatal("part due")
	}
}

func TestOreLumpTimeoutFails(t *testing.T) {
	t.Parallel()
	e, q := oreEnv()
	id, _ := q.OfferAt(qd.OreLump, 33)
	run(e, q, e.clock.now+29*day)
	if get(t, q, id).State != qd.Active {
		t.Fatal("ended early")
	}
	run(e, q, e.clock.now+2*day)
	n := get(t, q, id)
	if n.State != qd.Failed || n.Reason != qd.Timeout {
		t.Fatalf("state %v reason %v", n.State, n.Reason)
	}
}

func TestOreLumpDeterministicAndDetail(t *testing.T) {
	t.Parallel()
	_, a := oreEnv()
	_, b := oreEnv()
	ia, _ := a.OfferAt(qd.OreLump, 33)
	ib, _ := b.OfferAt(qd.OreLump, 33)
	na, _ := a.Get(ia)
	nb, _ := b.Get(ib)
	if na.Seed != nb.Seed || na.Stars != nb.Stars || na.Deadline != nb.Deadline {
		t.Fatal("same seed, different quest")
	}
	var k OreLump
	d := k.Detail(na)
	if len(d) != 2 || d[0].Key != "quest.orelump.lump" || d[0].Args[1] != "2300" || d[1].Args[0] != "33" {
		t.Fatalf("detail %+v", d)
	}
	if k.Detail(qd.Quest{}) != nil {
		t.Fatal("detail of empty quest")
	}
}
