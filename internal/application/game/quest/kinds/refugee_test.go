package kinds

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/quest"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/entity"
)

// openedRefugee offers the quest, opens its site on map 7 and returns it.
func openedRefugee(t *testing.T, e *env) (*quest.Quests, qd.ID, qd.Quest) {
	t.Helper()
	q := newRoot(e, Refugee{})
	id, ok := q.Offer(qd.Refugee)
	if !ok {
		t.Fatal("refugee not offered")
	}
	n := get(t, q, id)
	openSite(e, q, id, n.Parts[0].Tile, 7)
	return q, id, get(t, q, id)
}

func TestRefugeeOffer(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q := newRoot(e, Refugee{})
	id, ok := q.Offer(qd.Refugee)
	if !ok {
		t.Fatal("not offered")
	}
	n := get(t, q, id)
	if n.State != qd.Active || n.Deadline != e.clock.now+10*day || n.Giver != 0 {
		t.Fatalf("offer %+v", n)
	}
	if len(n.Parts) != 2 || n.Parts[0].Kind != qd.PartSite || n.Parts[0].Tile != 40 ||
		n.Parts[1].Kind != qd.PartPawn || n.Parts[1].Pawn != 0 || n.Parts[1].Seed == 0 {
		t.Fatalf("parts %+v", n.Parts)
	}
	if len(n.Rewards) != 1 || n.Rewards[0].Pawn != n.Parts[1].Seed {
		t.Fatalf("rewards %+v", n.Rewards)
	}
	if n.Target == 0 || n.Parts[0].Count < 2 || n.Parts[0].Count > 5 {
		t.Fatalf("guards: target %d count %d", n.Target, n.Parts[0].Count)
	}
	if len(e.builder.pawns) != 0 || len(e.builder.camps) != 0 || len(e.pawns.walked) != 0 {
		t.Fatal("something was laid before the site opened")
	}
}

func TestRefugeeGates(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q := newRoot(e, Refugee{})
	e.world.threats = false
	if _, ok := q.Offer(qd.Refugee); ok {
		t.Fatal("offered with threats off")
	}
	e.world.threats, e.world.colonists = true, 12
	if _, ok := q.Offer(qd.Refugee); ok {
		t.Fatal("offered at 12 colonists")
	}
	e.world.colonists, e.world.near = 3, -1
	if _, ok := q.Offer(qd.Refugee); ok {
		t.Fatal("offered with no free tile")
	}
	// No hostile faction: no guards.
	e = newEnv()
	e.world.factions = e.world.factions[:1]
	e.gw.rel = e.gw.rel[:1]
	q = newRoot(e, Refugee{})
	id, ok := q.Offer(qd.Refugee)
	if !ok {
		t.Fatal("not offered without enemies")
	}
	if n := get(t, q, id); n.Target != 0 || n.Parts[0].Count != 0 {
		t.Fatalf("guards without enemies: %+v", n)
	}
}

func TestRefugeeOpenLaysThePersonAndGuards(t *testing.T) {
	t.Parallel()
	e := newEnv()
	_, _, n := openedRefugee(t, e)
	if len(e.builder.pawns) != 1 {
		t.Fatalf("pawns %+v", e.builder.pawns)
	}
	b := e.builder.pawns[0]
	if b.Held || !b.MapCurrent || b.Faction != 0 || b.Seed != n.Parts[1].Seed || b.X != 75 || b.Y != 75 {
		t.Fatalf("downed call %+v", b)
	}
	if n.Parts[1].Pawn != uint64(b.ID) || n.Parts[0].Map != 7 {
		t.Fatalf("parts %+v", n.Parts)
	}
	if len(e.builder.camps) != 1 || len(e.builder.arms[0]) != n.Parts[0].Count || len(e.builder.camps[0].Posts) == 0 {
		t.Fatalf("camps %+v arms %+v", e.builder.camps, e.builder.arms)
	}
}

func TestRefugeeOpenTwiceLaysOnce(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id, _ := openedRefugee(t, e)
	openSite(e, q, id, 40, 8)
	if len(e.builder.pawns) != 1 || get(t, q, id).Parts[0].Map != 7 {
		t.Fatalf("pawns %+v", e.builder.pawns)
	}
}

func TestRefugeeHelpJoinsAndSucceeds(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id, n := openedRefugee(t, e)
	pid := entity.EntityID(n.Parts[1].Pawn)
	if err := q.Apply(quest.Fulfil(id, 0)); err != nil {
		t.Fatal(err)
	}
	n = get(t, q, id)
	if n.State != qd.Succeeded || !n.Parts[1].Done || !n.Paid {
		t.Fatalf("after help %+v", n)
	}
	if !reflect.DeepEqual(e.pawns.joined, []entity.EntityID{pid}) || len(e.pawns.walked) != 0 {
		t.Fatalf("joined %v walked %v", e.pawns.joined, e.pawns.walked)
	}
}

func TestRefugeeHelpBeforeOpenOrAfterDeathChangesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q := newRoot(e, Refugee{})
	id, _ := q.Offer(qd.Refugee)
	q.Apply(quest.Fulfil(id, 0))
	if get(t, q, id).State != qd.Active || len(e.pawns.joined) != 0 {
		t.Fatal("helped a pawn that is not there")
	}
	openSite(e, q, id, 40, 7)
	e.pawns.dead = append(e.pawns.dead, e.builder.pawns[0].ID)
	q.Apply(quest.Fulfil(id, 0))
	if get(t, q, id).State != qd.Active || len(e.pawns.joined) != 0 {
		t.Fatal("helped a dead pawn")
	}
}

func TestRefugeeJoinedByCareSucceeds(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id, n := openedRefugee(t, e)
	q.Signal(quest.Signal{Kind: quest.SigPawnJoined, Pawn: entity.EntityID(n.Parts[1].Pawn) + 1})
	if get(t, q, id).State != qd.Active {
		t.Fatal("another pawn joined")
	}
	q.Signal(quest.Signal{Kind: quest.SigPawnJoined, Pawn: entity.EntityID(n.Parts[1].Pawn)})
	if n = get(t, q, id); n.State != qd.Succeeded || !n.Paid || len(e.pawns.walked) != 0 {
		t.Fatalf("%+v", n)
	}
}

func TestRefugeeFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		do   func(e *env, q *quest.Quests, id qd.ID, n qd.Quest)
		want qd.Reason
	}{
		{"dies", func(e *env, q *quest.Quests, id qd.ID, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigPawnDied, Pawn: entity.EntityID(n.Parts[1].Pawn)})
		}, qd.PawnLost},
		{"leaves", func(e *env, q *quest.Quests, id qd.ID, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigPawnLeft, Pawn: entity.EntityID(n.Parts[1].Pawn)})
		}, qd.PawnLost},
		{"map closes", func(e *env, q *quest.Quests, id qd.ID, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: id, Map: 7})
		}, qd.SiteLost},
		{"site lost", func(e *env, q *quest.Quests, id qd.ID, n qd.Quest) {
			q.Signal(quest.Signal{Kind: quest.SigSiteLost, Tile: n.Parts[0].Tile})
		}, qd.SiteLost},
		{"deadline", func(e *env, q *quest.Quests, id qd.ID, n qd.Quest) {
			run(e, q, n.Deadline)
		}, qd.Timeout},
	}
	for _, c := range cases {
		e := newEnv()
		q, id, n := openedRefugee(t, e)
		c.do(e, q, id, n)
		if got := get(t, q, id); got.State != qd.Failed || got.Reason != c.want {
			t.Fatalf("%s: %+v", c.name, got)
		}
	}
}

func TestRefugeeIgnoresOtherSignals(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id, n := openedRefugee(t, e)
	q.Signal(quest.Signal{Kind: quest.SigPawnDied, Pawn: entity.EntityID(n.Parts[1].Pawn) + 5})
	q.Signal(quest.Signal{Kind: quest.SigMapClosed, Quest: id, Map: 99})
	q.Signal(quest.Signal{Kind: quest.SigSiteCensus, Quest: id, Map: 7, Count: 3})
	if get(t, q, id).State != qd.Active {
		t.Fatal("ended by an unrelated signal")
	}
	var k Refugee
	var empty qd.Quest
	if k.Signal(quest.Ctx{}, &empty, quest.Signal{Kind: quest.SigSiteOpened}) != quest.VerdictNone ||
		k.PartDue(quest.Ctx{}, &empty, 0) != quest.VerdictNone || k.Detail(empty) != nil {
		t.Fatal("empty quest reacts")
	}
	k.Accepted(quest.Ctx{}, &empty)
	k.Ended(quest.Ctx{}, &empty)
	// A map id of 0 opens nothing.
	if k.Signal(quest.Ctx{}, &n, quest.Signal{Kind: quest.SigSiteOpened}) != quest.VerdictNone {
		t.Fatal("opened map 0")
	}
}

func TestRefugeeBuilderRefusesFailsTheQuest(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.builder.refuse = true
	q := newRoot(e, Refugee{})
	id, _ := q.Offer(qd.Refugee)
	openSite(e, q, id, 40, 7)
	if n := get(t, q, id); n.State != qd.Failed || n.Reason != qd.SiteLost || n.Parts[1].Pawn != 0 {
		t.Fatalf("%+v", n)
	}
}

func TestRefugeeDeterministic(t *testing.T) {
	t.Parallel()
	mk := func() qd.Quest {
		e := newEnv()
		e.rolls.s = 21
		_, _, n := openedRefugee(t, e)
		return n
	}
	if a, b := mk(), mk(); !reflect.DeepEqual(a, b) {
		t.Fatalf("%+v != %+v", a, b)
	}
}

func TestRefugeeDetail(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id, n := openedRefugee(t, e)
	var k Refugee
	keys := func(n qd.Quest) []string {
		var out []string
		for _, l := range k.Detail(n) {
			out = append(out, l.Key)
		}
		return out
	}
	if got := keys(n); !reflect.DeepEqual(got, []string{"quest.refugee.site", "quest.refugee.deadline", "quest.refugee.pawn", "quest.refugee.waiting"}) {
		t.Fatalf("%v", got)
	}
	if l := k.Detail(n)[2]; len(l.Args) != 2 || l.Args[0] == "" {
		t.Fatalf("pawn line %+v", l)
	}
	fresh := n
	fresh.Parts = append([]qd.Part(nil), n.Parts...)
	fresh.Parts[1].Pawn = 0
	if got := keys(fresh); got[len(got)-1] != "quest.refugee.guards" {
		t.Fatalf("%v", got)
	}
	q.Apply(quest.Fulfil(id, 0))
	if got := keys(get(t, q, id)); got[len(got)-1] != "quest.refugee.helped" {
		t.Fatalf("%v", got)
	}
}
