package game

// quest_host_test.go: the quests root inside the game (QST-1): commands,
// letters, timers, the read model, saves and determinism. The kinds land
// in QST-2, so these tests use deskKind, a request-like kind of their own.

import (
	"reflect"
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// deskKind is offered by the first human faction that is not hostile and
// pays silver; it has the Request kind's rules (an accept window, a
// choice of rewards) and does nothing once active.
type deskKind struct{}

func deskGiver(c quest.Ctx) int {
	for _, f := range c.R.World.Factions() {
		if f.Relation != faction.Hostile && !f.Defeated && f.CanBeFriendly {
			return f.Index
		}
	}
	return -1
}

func (deskKind) Def() qd.Def               { return qd.DefOf(qd.Request) }
func (deskKind) CanOffer(c quest.Ctx) bool { return deskGiver(c) >= 0 }
func (deskKind) Build(c quest.Ctx, q *qd.Quest) bool {
	f := deskGiver(c)
	q.Giver = f + 1
	q.Rewards = c.Rewards(*q, []qd.PoolItem{{Kind: item.Silver}}, 0, false)
	return f >= 0
}
func (deskKind) Accepted(quest.Ctx, *qd.Quest)                           {}
func (deskKind) Signal(quest.Ctx, *qd.Quest, quest.Signal) quest.Verdict { return quest.VerdictNone }
func (deskKind) PartDue(quest.Ctx, *qd.Quest, int) quest.Verdict         { return quest.VerdictNone }
func (deskKind) Ended(quest.Ctx, *qd.Quest)                              {}
func (deskKind) Detail(qd.Quest) []quest.Line                            { return nil }

// questScene is a landed setup colony whose quests root knows deskKind,
// with one human faction neutral.
func questScene(t *testing.T) *Game {
	t.Helper()
	g := commitSetup(t)
	for i := range g.ng.factions {
		if f := &g.ng.factions[i]; faction.DefOf(f.Type).CanBeFriendly && f.Type.Human() {
			f.Relation, f.Defeated, f.Goodwill = faction.Neutral, false, 0
			break
		}
	}
	g.worldS().quests = quest.New(questHost{g}.roles(), []quest.Kind{deskKind{}})
	return g
}

// questView is quest id in the read model.
func questView(t *testing.T, g *Game, id qd.ID) QuestView {
	t.Helper()
	for _, v := range g.World().Quests {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("quest %d is not in the read model: %+v", id, g.World().Quests)
	return QuestView{}
}

func offerDesk(t *testing.T, g *Game) qd.ID {
	t.Helper()
	id, ok := questOf(g).Offer(qd.Request)
	if !ok {
		t.Fatal("fixture: the desk quest is offered")
	}
	return id
}

func TestQuestCommandsAcceptDismissRestore(t *testing.T) {
	t.Parallel()
	g := questScene(t)
	id := offerDesk(t, g)
	v := questView(t, g, id)
	if v.State != qd.Offered || len(v.Rewards) == 0 || v.Giver == 0 || v.AcceptBy <= v.OfferedAt {
		t.Fatalf("an offer with rewards, a giver and an accept window: %+v", v)
	}
	var letter *LetterView
	for _, l := range g.letterViews() {
		if l.Quest == uint32(id) {
			letter = &l
		}
	}
	if letter == nil || letter.Title == "" {
		t.Fatalf("the offer posts a letter about it: %+v", g.letterViews())
	}
	step := func(k command.Kind, choice int) QuestView {
		g.Enqueue(command.Command{Kind: k, Index: int(id), Count: choice})
		g.Step(1)
		return questView(t, g, id)
	}
	if v := step(command.DismissQuest, 0); !v.Dismissed {
		t.Fatal("dismissed")
	}
	if v := step(command.RestoreQuest, 0); v.Dismissed {
		t.Fatal("restored")
	}
	if v := step(command.AcceptQuest, len(v.Rewards)-1); v.State != qd.Active || v.Chosen != len(v.Rewards)-1 || v.AcceptedAt == 0 {
		t.Fatalf("accepted with the last option: %+v", v)
	}
	if v := step(command.AcceptQuest, 0); v.State != qd.Active || v.Chosen != len(v.Rewards)-1 {
		t.Fatalf("a second accept changes nothing: %+v", v)
	}
	g.Enqueue(command.Command{Kind: command.DismissQuest, Index: int(id) + 99}) // unknown: refused, said
	g.Step(1)
}

func TestQuestOfferExpiresAndLeavesHistory(t *testing.T) {
	t.Parallel()
	g := questScene(t)
	id := offerDesk(t, g)
	letters := len(g.letterViews())
	by := questView(t, g, id).AcceptBy
	questOf(g).Hourly(by - 1)
	if v := questView(t, g, id); v.State != qd.Offered {
		t.Fatalf("still offered a tick before its window ends: %+v", v)
	}
	questOf(g).Hourly(by)
	if v := questView(t, g, id); v.State != qd.Expired || v.EndedAt != by {
		t.Fatalf("expired at the end of its window, kept as history: %+v", v)
	}
	if len(g.letterViews()) != letters {
		t.Fatal("an expiry posts no letter (Q36)")
	}
	g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(id)})
	g.Step(1)
	if v := questView(t, g, id); v.State != qd.Expired {
		t.Fatalf("an expired offer cannot be accepted: %+v", v)
	}
}

func TestQuestsSurviveSaveAndLoad(t *testing.T) {
	t.Parallel()
	g := questScene(t)
	a, b := offerDesk(t, g), offerDesk(t, g)
	g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: int(a)})
	g.Enqueue(command.Command{Kind: command.DismissQuest, Index: int(b)})
	g.Step(1)
	h := saveAndLoad(t, g)
	if got, want := h.StateHash(), g.StateHash(); got != want {
		t.Fatalf("state hash %#x after load, want %#x", got, want)
	}
	if got, want := h.World().Quests, g.World().Quests; !reflect.DeepEqual(got, want) {
		t.Fatalf("quests after load:\n%+v\nwant\n%+v", got, want)
	}
	g.Step(2 * clock.TicksPerHour)
	h.Step(2 * clock.TicksPerHour)
	if h.StateHash() != g.StateHash() {
		t.Fatal("a loaded game goes on as the saved one does")
	}
}

func TestLegacyColonyHasNoQuests(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 20})
	g.Enqueue(command.Command{Kind: command.AcceptQuest, Index: 1})
	g.Step(clock.TicksPerDay + 1)
	if questOf(g) != nil || g.World().Quests != nil || g.worldSt != nil && g.worldSt.quests != nil {
		t.Fatal("a legacy colony has no quests (decision 17)")
	}
}

// questYears runs g's quests root alone over years of hours from landing,
// with the day's cadence at midnight, and returns the read model.
func questYears(g *Game, years int) []QuestView {
	q, start := questOf(g), g.dayOff()
	for t := start; t < start+uint64(years)*clock.TicksPerYear; t += clock.TicksPerHour {
		q.Hourly(t)
		if t%clock.TicksPerDay == 0 {
			q.Daily(t)
		}
	}
	return questViews(g)
}

func TestQuestGeneratorDeterministic(t *testing.T) {
	t.Parallel()
	a, b := questYears(questScene(t), 2), questYears(questScene(t), 2)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same colony makes the same offers")
	}
	// one offer per 10-day window from the first (QST-020): 2 years of 60
	// days is 12 windows; the first can be lost to a landing mid-window.
	if n := len(a); n < 11 || n > 12 {
		t.Fatalf("%d offers in 2 years, want 11 or 12", n)
	}
	for i := 1; i < len(a); i++ {
		if gap := a[i].OfferedAt - a[i-1].OfferedAt; gap >= 2*qd.WindowDays*clock.TicksPerDay {
			t.Fatalf("offers %d and %d are %d ticks apart", i-1, i, gap)
		}
	}
}

// TestQuestPointsIgnoreTheStorytellerRoll: a quest's points never take the
// storyteller's 50-150 % roll (QST-028); only the raid formula itself.
func TestQuestPointsIgnoreTheStorytellerRoll(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	h := questHost{g}
	want := h.Points()
	if want <= 0 {
		t.Fatalf("points %d, want some", want)
	}
	for _, pct := range []int{50, 100, 150} {
		g.ng.plan.PointsPct = pct
		if got := h.Points(); got != want {
			t.Fatalf("with the storyteller at %d %%, quest points %d, want %d", pct, got, want)
		}
	}
}
