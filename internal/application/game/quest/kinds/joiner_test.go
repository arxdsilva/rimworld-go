package kinds

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/entity"
)

func offerJoiner(t *testing.T, e *env) (*quest.Quests, qd.ID) {
	t.Helper()
	q := newRoot(e, Joiner{})
	id, ok := q.Offer(qd.Joiner)
	if !ok {
		t.Fatal("joiner not offered")
	}
	return q, id
}

func TestJoinerOfferShape(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerJoiner(t, e)
	n := get(t, q, id)
	if n.State != qd.Offered || n.AcceptBy != e.clock.now+7*hour || n.Deadline != 0 {
		t.Fatalf("state %v acceptBy %d deadline %d", n.State, n.AcceptBy, n.Deadline)
	}
	if n.Giver != 1 || n.Target == 1 || n.Target == 0 {
		t.Fatalf("giver %d target %d", n.Giver, n.Target)
	}
	if len(n.Rewards) != 1 || n.Rewards[0].Pawn == 0 || len(n.Rewards[0].Lines) != 0 || n.Rewards[0].Goodwill != 0 {
		t.Fatalf("rewards %+v", n.Rewards)
	}
	if len(n.Parts) != 2 || n.Parts[1].Kind != qd.PartThreat || n.Parts[1].At != 0 || n.Parts[1].Points <= 0 {
		t.Fatalf("parts %+v", n.Parts)
	}
	if len(e.pawns.walked) != 0 {
		t.Fatal("the pawn walked in before accept")
	}
}

func TestJoinerLifecycleEveryThreat(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for s := uint64(1); s < 80; s++ {
		e := newEnv()
		e.rolls.s = s
		q, id := offerJoiner(t, e)
		pawn := get(t, q, id).Rewards[0].Pawn
		if err := q.Apply(quest.Accept(id, 0)); err != nil {
			t.Fatal(err)
		}
		n := get(t, q, id)
		at := n.Parts[1].At
		if n.State != qd.Active || at < e.clock.now+hour || at > e.clock.now+6*hour {
			t.Fatalf("seed %d: state %v at %d now %d", s, n.State, at, e.clock.now)
		}
		if len(e.pawns.walked) != 1 || e.pawns.walked[0] != pawn {
			t.Fatalf("walked %v want %d", e.pawns.walked, pawn)
		}
		if len(e.threats.calls) != 0 {
			t.Fatal("threat fired at accept")
		}
		run(e, q, at+hour)
		n = get(t, q, id)
		if n.State != qd.Succeeded || !n.Parts[1].Done {
			t.Fatalf("seed %d: state %v", s, n.State)
		}
		if len(e.threats.calls) != 1 {
			t.Fatalf("calls %+v", e.threats.calls)
		}
		c := e.threats.calls[0]
		seen[c.Kind] = true
		switch c.Kind {
		case "raid":
			if c.Faction != n.Target-1 || c.Points != n.Parts[1].Points {
				t.Fatalf("raid %+v target %d", c, n.Target)
			}
		case "condition":
			d := c.Ticks / day
			if c.Cond == quest.CondNone || d < 2 || d > 5 || c.Ticks%day != 0 {
				t.Fatalf("condition %+v", c)
			}
		}
	}
	for _, k := range []string{"raid", "manhunters", "condition"} {
		if !seen[k] {
			t.Fatalf("threat %q never rolled: %v", k, seen)
		}
	}
}

func TestJoinerConditionKinds(t *testing.T) {
	t.Parallel()
	got := map[uint8]bool{}
	for s := uint64(1); s < 200 && len(got) < 3; s++ {
		e := newEnv()
		e.rolls.s = s
		q, id := offerJoiner(t, e)
		q.Apply(quest.Accept(id, 0))
		run(e, q, e.clock.now+7*hour)
		for _, c := range e.threats.calls {
			if c.Kind == "condition" {
				got[c.Cond] = true
			}
		}
	}
	for _, c := range []uint8{quest.CondHeatWave, quest.CondColdSnap, quest.CondVolcanicWinter} {
		if !got[c] {
			t.Fatalf("condition %d never started: %v", c, got)
		}
	}
}

func TestJoinerThreatFailureStillSucceeds(t *testing.T) {
	t.Parallel()
	e := newEnv()
	e.threats.fail = true
	q, id := offerJoiner(t, e)
	q.Apply(quest.Accept(id, 0))
	run(e, q, e.clock.now+7*hour)
	if n := get(t, q, id); n.State != qd.Succeeded {
		t.Fatalf("state %v", n.State)
	}
}

func TestJoinerRaidFallsBackWhenEnemyMadePeace(t *testing.T) {
	t.Parallel()
	var k Joiner
	e := newEnv()
	q := newRoot(e, k)
	_ = q
	e.gw.rel[1] = faction.Neutral
	n := qd.Quest{State: qd.Active, Target: 2, Parts: []qd.Part{{}, {Kind: qd.PartThreat, Threat: joinerRaid, Points: 300, Seed: 5}}}
	c := rootCtx(e)
	if v := k.PartDue(c, &n, 1); !v.Succeeds() {
		t.Fatal("no success")
	}
	if len(e.threats.calls) != 1 || e.threats.calls[0].Kind != "manhunters" || e.threats.calls[0].Points != 300 {
		t.Fatalf("calls %+v", e.threats.calls)
	}
	// An enemy that is still hostile but whose raid cannot start.
	e = newEnv()
	e.threats.fail = true
	n.Target = 2
	if v := k.PartDue(rootCtx(e), &n, 1); !v.Succeeds() || len(e.threats.calls) != 2 {
		t.Fatalf("calls %+v", e.threats.calls)
	}
}

// rootCtx is a Ctx over e's roles with no root behind it.
func rootCtx(e *env) quest.Ctx { return quest.Ctx{R: e.roles(), Now: e.clock.now, Tile: -1} }

func TestJoinerAcceptWindowLapses(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerJoiner(t, e)
	run(e, q, e.clock.now+8*hour)
	n := get(t, q, id)
	if n.State != qd.Expired {
		t.Fatalf("state %v", n.State)
	}
	if len(e.pawns.walked) != 0 || len(e.threats.calls) != 0 || len(e.letters.list) != 1 {
		t.Fatalf("walked %v calls %v letters %d", e.pawns.walked, e.threats.calls, len(e.letters.list))
	}
	if err := q.Apply(quest.Accept(id, 0)); err == nil {
		t.Fatal("accepted an expired offer")
	}
}

func TestJoinerGiverHostileBeforeAccept(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerJoiner(t, e)
	e.gw.rel[0] = faction.Hostile
	run(e, q, e.clock.now+hour)
	n := get(t, q, id)
	if !n.State.Terminal() || n.State == qd.Succeeded {
		t.Fatalf("state %v", n.State)
	}
	if len(e.pawns.walked) != 0 {
		t.Fatal("pawn walked in")
	}
}

func TestJoinerLosingPawnChangesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q, id := offerJoiner(t, e)
	q.Apply(quest.Accept(id, 0))
	pawn := get(t, q, id).Rewards[0].Pawn
	e.pawns.dead = append(e.pawns.dead, 1)
	q.Signal(quest.Signal{Kind: quest.SigPawnDied, Pawn: entity.EntityID(pawn)})
	if n := get(t, q, id); n.State != qd.Active {
		t.Fatalf("state %v", n.State)
	}
	run(e, q, e.clock.now+7*hour)
	if n := get(t, q, id); n.State != qd.Succeeded {
		t.Fatalf("state %v", n.State)
	}
}

func TestJoinerGates(t *testing.T) {
	t.Parallel()
	e := newEnv()
	q := newRoot(e, Joiner{})
	e.world.threats = false
	if _, ok := q.Offer(qd.Joiner); ok {
		t.Fatal("offered with quest threats off")
	}
	e.world.threats = true
	e.world.colonists = 12
	if _, ok := q.Offer(qd.Joiner); ok {
		t.Fatal("offered at 12 colonists")
	}
	e.world.colonists = 3
	e.world.factions[0].Defeated = true
	if _, ok := q.Offer(qd.Joiner); ok {
		t.Fatal("offered with only a defeated friendly faction")
	}
	e.world.factions[0].Defeated = false
	e.world.factions[0].CanBeFriendly = false
	if _, ok := q.Offer(qd.Joiner); ok {
		t.Fatal("offered without a friendly-capable faction")
	}
	// Hostile factions never give.
	e.world.factions[0].CanBeFriendly = true
	e.world.factions[0].Relation = faction.Hostile
	if _, ok := q.Offer(qd.Joiner); ok {
		t.Fatal("hostile giver")
	}
	var k Joiner
	if k.Build(rootCtx(e), &qd.Quest{}) {
		t.Fatal("built with no giver")
	}
}

func TestJoinerNoEnemyMeansNoRaid(t *testing.T) {
	t.Parallel()
	for s := uint64(1); s < 60; s++ {
		e := newEnv()
		e.rolls.s = s
		e.world.factions = e.world.factions[:1]
		e.gw.rel = e.gw.rel[:1]
		q, id := offerJoiner(t, e)
		n := get(t, q, id)
		if n.Target != 0 || n.Parts[1].Threat == joinerRaid {
			t.Fatalf("target %d threat %d", n.Target, n.Parts[1].Threat)
		}
	}
}

func TestJoinerDeterministic(t *testing.T) {
	t.Parallel()
	mk := func() qd.Quest {
		e := newEnv()
		e.rolls.s = 99
		q, id := offerJoiner(t, e)
		q.Apply(quest.Accept(id, 0))
		return get(t, q, id)
	}
	if a, b := mk(), mk(); !reflect.DeepEqual(a, b) {
		t.Fatalf("%+v != %+v", a, b)
	}
}

func TestJoinerGuards(t *testing.T) {
	t.Parallel()
	var k Joiner
	c := rootCtx(newEnv())
	n := qd.Quest{}
	k.Accepted(c, &n) // no parts: nothing
	k.Ended(c, &n)
	if k.Signal(c, &n, quest.Signal{}) != quest.VerdictNone {
		t.Fatal("signal")
	}
	if k.PartDue(c, &n, 1) != quest.VerdictNone || k.PartDue(c, &n, 0) != quest.VerdictNone {
		t.Fatal("part due on empty quest")
	}
	n.Parts = []qd.Part{{Kind: qd.PartPawn}, {Kind: qd.PartThreat}}
	if k.PartDue(c, &n, 0) != quest.VerdictNone {
		t.Fatal("pawn part fired a threat")
	}
}

func TestJoinerDetail(t *testing.T) {
	t.Parallel()
	var k Joiner
	keys := func(q qd.Quest) []string {
		var out []string
		for _, l := range k.Detail(q) {
			out = append(out, l.Key)
		}
		return out
	}
	parts := []qd.Part{{}, {Kind: qd.PartThreat}}
	done := []qd.Part{{}, {Kind: qd.PartThreat, Done: true}}
	cases := []struct {
		q    qd.Quest
		want []string
	}{
		{qd.Quest{State: qd.Offered, Giver: 1, Parts: parts}, []string{"quest.joiner.about", "quest.joiner.warning"}},
		{qd.Quest{State: qd.Active, Giver: 1, Parts: parts}, []string{"quest.joiner.about", "quest.joiner.coming"}},
		{qd.Quest{State: qd.Succeeded, Giver: 1, Parts: done}, []string{"quest.joiner.about", "quest.joiner.arrived"}},
		{qd.Quest{State: qd.Expired, Giver: 1}, []string{"quest.joiner.about"}},
	}
	for _, c := range cases {
		if got := keys(c.q); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%v: %v want %v", c.q.State, got, c.want)
		}
	}
	if l := k.Detail(cases[0].q)[0]; len(l.Args) != 1 || l.Args[0] != "1" {
		t.Fatalf("args %v", l.Args)
	}
}
