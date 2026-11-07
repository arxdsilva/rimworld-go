package quest

import (
	"errors"
	"reflect"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
)

const day = uint64(clock.TicksPerDay)

// withOptions is a quest with n reward options, a giver and a target.
func withOptions(n int) Quest {
	q := Quest{ID: 1, Giver: 1, Target: 2, Stars: 1}
	for i := range n {
		q.Rewards = append(q.Rewards, Reward{Goodwill: i + 1})
	}
	return q
}

func TestOffer_AutoAcceptIsActiveWithDeadline(t *testing.T) {
	q := Offer(withOptions(1), 100, DefOf(Stash))
	if q.State != Active || q.AcceptedAt != 100 || q.OfferedAt != 100 {
		t.Fatalf("stash offer = %+v, want Active at 100", q)
	}
	if q.Deadline != 100+16*day || q.AcceptBy != 0 || q.Chosen != -1 || q.Kind != Stash {
		t.Fatalf("stash timers = deadline %d acceptBy %d chosen %d kind %d", q.Deadline, q.AcceptBy, q.Chosen, q.Kind)
	}
}

func TestOffer_OfferedHasAcceptWindow(t *testing.T) {
	q := Offer(Quest{Kind: Camp}, 50, DefOf(Camp))
	if q.State != Offered || q.AcceptBy != 50+6*day || q.Deadline != 0 || q.AcceptedAt != 0 {
		t.Fatalf("camp offer = %+v", q)
	}
	// No completion window: a talks quest auto-accepts with no deadline.
	q = Offer(Quest{}, 7, DefOf(Talks))
	if q.State != Active || q.Deadline != 0 {
		t.Fatalf("talks offer = %+v, want Active, no deadline", q)
	}
}

func TestAccept(t *testing.T) {
	camp := DefOf(Camp)
	cases := []struct {
		name    string
		q       Quest
		choice  int
		wantErr error
	}{
		{"first of three", Offer(withOptions(3), 0, camp), 0, nil},
		{"last of three", Offer(withOptions(3), 0, camp), 2, nil},
		{"no options, -1", Offer(withOptions(0), 0, camp), -1, nil},
		{"dismissed offer", dismissed(Offer(withOptions(1), 0, camp)), 0, nil},
		{"choice too big", Offer(withOptions(3), 0, camp), 3, ErrChoice},
		{"choice negative", Offer(withOptions(3), 0, camp), -1, ErrChoice},
		{"no options, 0", Offer(withOptions(0), 0, camp), 0, ErrChoice},
		{"active", Offer(withOptions(1), 0, DefOf(Stash)), 0, ErrMove},
		{"succeeded", ended(Succeeded), 0, ErrMove},
		{"failed", ended(Failed), 0, ErrMove},
		{"expired", ended(Expired), 0, ErrMove},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Accept(c.q, 1000, c.choice, camp)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if err != nil {
				if !reflect.DeepEqual(got, c.q) {
					t.Fatalf("a refused accept changed the quest: %+v", got)
				}
				return
			}
			if got.State != Active || got.AcceptedAt != 1000 || int(got.Chosen) != c.choice {
				t.Fatalf("accepted = %+v", got)
			}
			if got.Deadline != 1000+17*day {
				t.Fatalf("deadline = %d, want accept + 17 d", got.Deadline)
			}
			if got.Dismissed != c.q.Dismissed {
				t.Fatal("accept changed Dismissed")
			}
		})
	}
}

func TestAccept_NoCompletionWindow(t *testing.T) {
	q, err := Accept(Offer(withOptions(1), 0, DefOf(Joiner)), 10, 0, DefOf(Joiner))
	if err != nil || q.Deadline != 0 {
		t.Fatalf("joiner accept = %+v, %v; want no deadline", q, err)
	}
}

func dismissed(q Quest) Quest {
	q.Dismissed = true
	return q
}

func ended(s State) Quest {
	q := withOptions(2)
	q.State, q.Reason, q.EndedAt, q.Chosen = s, Timeout, 5, 1
	return q
}

func TestEndMoves(t *testing.T) {
	offered := Offer(withOptions(2), 0, DefOf(Camp))
	active := Offer(withOptions(1), 0, DefOf(Stash))
	type move func(Quest) (Quest, error)
	type endCase struct {
		name       string
		q          Quest
		m          move
		wantErr    error
		wantState  State
		wantReason Reason
	}
	expire := func(q Quest) (Quest, error) { return Expire(q, 900) }
	succeed := func(q Quest) (Quest, error) { return Succeed(q, 900) }
	fail := func(q Quest) (Quest, error) { return Fail(q, 900, GiverHostile) }
	cases := []endCase{
		{"expire offered", offered, expire, nil, Expired, Timeout},
		{"expire active", active, expire, ErrMove, 0, 0},
		{"succeed active", active, succeed, nil, Succeeded, ReasonNone},
		{"succeed offered", offered, succeed, ErrMove, 0, 0},
		{"fail active", active, fail, nil, Failed, GiverHostile},
		{"fail offered expires", offered, fail, nil, Expired, GiverHostile},
	}
	for _, s := range []State{Succeeded, Failed, Expired} {
		for _, m := range []struct {
			n string
			m move
		}{{"expire", expire}, {"succeed", succeed}, {"fail", fail}} {
			cases = append(cases, endCase{m.n + " terminal", ended(s), m.m, ErrMove, 0, 0})
		}
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.m(c.q)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if err != nil {
				if !reflect.DeepEqual(got, c.q) {
					t.Fatalf("a refused move changed the quest: %+v", got)
				}
				return
			}
			if got.State != c.wantState || got.Reason != c.wantReason || got.EndedAt != 900 {
				t.Fatalf("got state %d reason %d ended %d", got.State, got.Reason, got.EndedAt)
			}
			if !got.State.Terminal() {
				t.Fatal("an end move left a live state")
			}
		})
	}
}

func TestFail_KeepsPaidReward(t *testing.T) {
	q := Offer(withOptions(1), 0, DefOf(Joiner))
	q, _ = Accept(q, 5, 0, DefOf(Joiner))
	q.Paid = true
	q, err := Fail(q, 10, PawnLost)
	if err != nil || !q.Paid || q.Chosen != 0 {
		t.Fatalf("fail after pay = %+v, %v; the reward must stay paid", q, err)
	}
}

func TestDismissRestore(t *testing.T) {
	for _, s := range []State{Offered, Active, Succeeded, Failed, Expired} {
		q := withOptions(1)
		q.State, q.AcceptBy, q.Deadline = s, 77, 88
		d, err := Dismiss(q)
		if err != nil || !d.Dismissed || d.State != s || d.AcceptBy != 77 || d.Deadline != 88 {
			t.Fatalf("dismiss state %d = %+v, %v", s, d, err)
		}
		if _, err := Dismiss(d); !errors.Is(err, ErrMove) {
			t.Fatalf("dismiss twice: err = %v", err)
		}
		r, err := Restore(d)
		if err != nil || !reflect.DeepEqual(r, q) {
			t.Fatalf("restore = %+v, %v; want the original", r, err)
		}
		if _, err := Restore(r); !errors.Is(err, ErrMove) {
			t.Fatalf("restore a shown quest: err = %v", err)
		}
	}
}

func TestDue(t *testing.T) {
	parts := func(ats ...uint64) []Part {
		var ps []Part
		for _, at := range ats {
			ps = append(ps, Part{Kind: PartThreat, At: at})
		}
		return ps
	}
	cases := []struct {
		name string
		q    Quest
		tick uint64
		ev   Event
		part int
		ok   bool
	}{
		{"offered", Quest{State: Offered, AcceptBy: 40, Deadline: 5, Parts: parts(3)}, 40, EvAcceptEnd, -1, true},
		{"offered without window", Quest{State: Offered}, 0, EvNone, -1, false},
		{"active deadline", Quest{State: Active, Deadline: 90, AcceptBy: 5}, 90, EvDeadline, -1, true},
		{"active part first", Quest{State: Active, Deadline: 90, Parts: parts(0, 60, 30)}, 30, EvPart, 2, true},
		{"part tie: lowest index", Quest{State: Active, Parts: parts(30, 30)}, 30, EvPart, 0, true},
		{"deadline wins a tie", Quest{State: Active, Deadline: 30, Parts: parts(30)}, 30, EvDeadline, -1, true},
		{"part, no deadline", Quest{State: Active, Parts: parts(70)}, 70, EvPart, 0, true},
		{"done part skipped", Quest{State: Active, Deadline: 90, Parts: []Part{{At: 10, Done: true}}}, 90, EvDeadline, -1, true},
		{"active without timers", Quest{State: Active, Parts: parts(0)}, 0, EvNone, -1, false},
		{"succeeded", Quest{State: Succeeded, Deadline: 9, Parts: parts(3)}, 0, EvNone, -1, false},
		{"failed", Quest{State: Failed, Deadline: 9}, 0, EvNone, -1, false},
		{"expired", Quest{State: Expired, AcceptBy: 9}, 0, EvNone, -1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tick, ev, part, ok := Due(c.q)
			if tick != c.tick || ev != c.ev || part != c.part || ok != c.ok {
				t.Fatalf("Due = %d %d %d %v, want %d %d %d %v", tick, ev, part, ok, c.tick, c.ev, c.part, c.ok)
			}
		})
	}
}

func TestFailOnRelation(t *testing.T) {
	live := Quest{State: Active, Giver: 1, Target: 2}
	cases := []struct {
		name          string
		q             Quest
		k             KindID
		giver, target faction.Relation
		want          Reason
		ok            bool
	}{
		{"request giver hostile", live, Request, faction.Hostile, faction.Neutral, GiverHostile, true},
		{"stash ignores hostile giver", live, Stash, faction.Hostile, faction.Neutral, ReasonNone, false},
		{"refugee ignores hostile giver", live, Refugee, faction.Hostile, faction.Neutral, ReasonNone, false},
		{"camp target calmed", live, Camp, faction.Neutral, faction.Neutral, TargetNotHostile, true},
		{"camp target friendly", live, Camp, faction.Friendly, faction.Friendly, TargetNotHostile, true},
		{"camp giver first", live, Camp, faction.Hostile, faction.Neutral, GiverHostile, true},
		{"camp all well", live, Camp, faction.Friendly, faction.Hostile, ReasonNone, false},
		{"rescue target calmed", live, Rescue, faction.Hostile, faction.Neutral, TargetNotHostile, true},
		{"offered fails too", Quest{State: Offered, Giver: 1}, Request, faction.Hostile, 0, GiverHostile, true},
		{"no giver", Quest{State: Active, Target: 2}, Camp, faction.Hostile, faction.Hostile, ReasonNone, false},
		{"no target", Quest{State: Active, Giver: 1}, Camp, faction.Neutral, faction.Neutral, ReasonNone, false},
		{"terminal", Quest{State: Succeeded, Giver: 1, Target: 2}, Camp, faction.Hostile, faction.Neutral, ReasonNone, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, ok := FailOnRelation(c.q, DefOf(c.k), c.giver, c.target)
			if r != c.want || ok != c.ok {
				t.Fatalf("FailOnRelation = %d %v, want %d %v", r, ok, c.want, c.ok)
			}
		})
	}
}

func TestErrorText(t *testing.T) {
	if ErrMove.Error() == "" || ErrChoice.Error() == ErrMove.Error() {
		t.Fatal("errors need distinct text")
	}
}

// FuzzQuestTransitions applies any sequence of moves: no sequence leaves
// a terminal state, a refused move changes nothing, and Chosen stays in
// range.
func FuzzQuestTransitions(f *testing.F) {
	f.Add([]byte{0, 0, 1, 2, 3})
	f.Add([]byte{5, 4, 0, 2, 0, 1, 3, 2})
	f.Add([]byte{4, 4, 1, 5, 5, 0, 3, 0, 1})
	f.Add([]byte{2, 7, 4, 0, 0, 1, 2, 3, 6, 6})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 2 {
			return
		}
		k := KindID(1 + int(data[0])%(int(NumKinds)-1))
		d := DefOf(k)
		q := Offer(withOptions(int(data[1])%4), 1, d)
		now := uint64(1)
		for _, b := range data[2:] {
			now += uint64(b)
			before := q
			next, err := step(q, now, b, d)
			if err != nil && !reflect.DeepEqual(next, before) {
				t.Fatalf("refused move %d changed %+v to %+v", b%7, before, next)
			}
			if before.State.Terminal() && (next.State != before.State || next.Reason != before.Reason || next.EndedAt != before.EndedAt) {
				t.Fatalf("move %d left terminal state %d: %+v", b%7, before.State, next)
			}
			if next.Chosen < -1 || int(next.Chosen) >= len(next.Rewards) && next.Chosen != -1 {
				t.Fatalf("Chosen %d out of range (%d options)", next.Chosen, len(next.Rewards))
			}
			if next.State.Terminal() && next.EndedAt == 0 {
				t.Fatal("a terminal quest has no EndedAt")
			}
			q = next
		}
	})
}

// step applies move b%7 to q.
func step(q Quest, now uint64, b byte, d Def) (Quest, error) {
	switch b % 7 {
	case 0:
		return Accept(q, now, int(b>>3)%5-1, d)
	case 1:
		return Expire(q, now)
	case 2:
		return Succeed(q, now)
	case 3:
		return Fail(q, now, Reason(1+int(b>>3)%5))
	case 4:
		return Dismiss(q)
	case 5:
		return Restore(q)
	}
	return q, nil
}
