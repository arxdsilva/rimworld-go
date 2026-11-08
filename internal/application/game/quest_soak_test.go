package game

// quest_soak_test.go: quests over a long run through the whole game
// (QST-160): every kind offered on day 0 and accepted where it can be,
// plus whatever the cadence offers on its own; no quest outlives its timer,
// no site map is left open, and a save and load at the midpoint goes on as
// the saved game does.
//
// The default span is 20 in-game days: under -race one day of the world
// fixture takes about 2 s, so two in-game years would not fit CI's
// 10-minute budget for this package. RIMWORLD_QUEST_SOAK_DAYS=120 runs the
// two years (the measurement in the WORLD.md QST-4 section).

import (
	"os"
	"strconv"
	"testing"

	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

func TestQuestSoak(t *testing.T) {
	t.Parallel()
	days := 20
	if s := os.Getenv("RIMWORLD_QUEST_SOAK_DAYS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 2 {
			t.Fatalf("RIMWORLD_QUEST_SOAK_DAYS=%q: want a whole number of days, at least 2", s)
		}
		days = n
	}
	g := worldGame(t)
	for k := qd.KindID(1); k < qd.NumKinds; k++ {
		g.Enqueue(command.Command{Kind: command.DevOfferQuest, Index: int(k)})
	}
	g.Step(1)
	kinds := map[qd.KindID]bool{}
	for _, v := range questViews(g) {
		kinds[v.Kind] = true
		if v.State != qd.Offered {
			continue
		}
		choice := -1
		if len(v.Rewards) > 0 {
			choice = 0
		}
		g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(v.ID), Count: choice})
	}
	if len(kinds) < 4 {
		t.Fatalf("only %d kinds could be offered on day 0: %v", len(kinds), kinds)
	}
	for d := 1; d <= days; d++ {
		nextHours(g, 24)
		questsNotStuck(t, g, d)
		if d == days/2 {
			h := saveAndLoad(t, g)
			if h.StateHash() != g.StateHash() {
				t.Fatalf("day %d: state hash differs after save and load", d)
			}
			nextHours(g, 1)
			nextHours(h, 1)
			if h.StateHash() != g.StateHash() {
				t.Fatalf("day %d: a loaded game does not go on as the saved one does", d)
			}
		}
	}
	ended := 0
	for _, v := range questViews(g) {
		if v.State.Terminal() {
			ended++
		}
	}
	if ended == 0 {
		t.Fatalf("no quest ended in %d days: %+v", days, questViews(g))
	}
	if len(g.maps) != 1 {
		t.Fatalf("%d maps open with no caravan ever sent out, want only the colony", len(g.maps))
	}
}

// questsNotStuck fails when a quest is still waiting on a timer that ran
// out more than an hour ago (the hourly due pop missed it).
func questsNotStuck(t *testing.T, g *Game, day int) {
	t.Helper()
	now := uint64(g.clock.Now())
	for _, v := range questViews(g) {
		switch {
		case v.State == qd.Offered && v.AcceptBy != 0 && now > v.AcceptBy+clock.TicksPerHour:
			t.Fatalf("day %d: quest %d (%v) still offered %d ticks after its accept-by", day, v.ID, v.Kind, now-v.AcceptBy)
		case v.State == qd.Active && v.Deadline != 0 && now > v.Deadline+clock.TicksPerHour:
			t.Fatalf("day %d: quest %d (%v) still active %d ticks after its deadline", day, v.ID, v.Kind, now-v.Deadline)
		}
	}
}
