package hud

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
)

func (f *fakeShell) Do(i ports.Intent) { f.intents = append(f.intents, i) }

// A quest letter's opened window has a View quest row that sends ShowQuest;
// a plain letter has none (QST-133).
func TestQuestLetterViewQuestRow(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	if got := c.Right().letterFooter(game.LetterView{ID: 3, X: -1, Y: -1}); len(got) != 1 {
		t.Fatalf("a plain letter: %+v", got)
	}
	rows := c.Right().letterFooter(game.LetterView{ID: 4, Kind: "info", Quest: 7, X: -1, Y: -1})
	if len(rows) != 2 || rows[1].ID != "quest" || rows[1].Label != "View quest" || !rows[1].Closes {
		t.Fatalf("a quest letter: %+v", rows)
	}
	rows[1].OnClick()
	if len(f.intents) != 1 || f.intents[0] != (ports.ShowQuest{ID: 7}) {
		t.Fatalf("intents %+v", f.intents)
	}
	failed := c.Right().letterFooter(game.LetterView{ID: 5, Kind: "caution", Quest: 8, X: -1, Y: -1})
	if failed[1].Label != "View related quest" {
		t.Fatalf("a failed quest: %+v", failed)
	}
}

func TestQuestLetterTint(t *testing.T) {
	t.Parallel()
	if letterTint(game.LetterView{Kind: "info", Quest: 1}) == LetterColor("info") {
		t.Fatal("a quest offer shares the info frame")
	}
	if letterTint(game.LetterView{Kind: "info"}) != LetterColor("info") || letterTint(game.LetterView{Kind: "caution", Quest: 1}) != LetterColor("caution") {
		t.Fatal("a plain letter or a failed quest changed colour")
	}
}

// The alert is a pure function of AcceptBy and now: at the one-day mark it
// is off one tick before and on at it, and off again once the time is up.
func TestQuestExpiringLeftBoundary(t *testing.T) {
	t.Parallel()
	const by = 5 * clock.TicksPerDay
	for _, c := range []struct {
		now uint64
		ok  bool
	}{{by - clock.TicksPerDay - 1, false}, {by - clock.TicksPerDay, false}, {by - clock.TicksPerDay + 1, true}, {by - 1, true}, {by, false}, {by + 1, false}} {
		if _, ok := QuestExpiringLeft(by, c.now); ok != c.ok {
			t.Fatalf("now %d: shown %v, want %v", c.now, ok, c.ok)
		}
	}
	// no flap: walking the clock across the threshold flips once on, once off
	flips, prev := 0, false
	for now := uint64(by - 2*clock.TicksPerDay); now <= by+10; now += 7 {
		_, on := QuestExpiringLeft(by, now)
		if on != prev {
			flips++
		}
		prev = on
	}
	if flips != 2 {
		t.Fatalf("%d flips across the window, want 2", flips)
	}
}

func TestQuestExpiringAlert(t *testing.T) {
	t.Parallel()
	now := uint64(10 * clock.TicksPerDay)
	w := game.WorldView{Quests: []game.QuestView{
		{ID: 1, Kind: qd.Request, State: qd.Offered, AcceptBy: now + 3*clock.TicksPerDay},      // far
		{ID: 2, Kind: qd.Request, State: qd.Active, AcceptBy: now + 100},                       // taken
		{ID: 3, Kind: qd.Request, State: qd.Offered, AcceptBy: now + 5*clock.TicksPerHour + 1}, // 6 h
		{ID: 4, Kind: qd.Request, State: qd.Offered, AcceptBy: now + 2*clock.TicksPerHour},     // 2 h: soonest
		{ID: 5, Kind: qd.Request, State: qd.Offered, AcceptBy: now - 5},                        // lapsed
	}}
	al, ok := QuestExpiringAlertIn(en, w, now)
	if !ok || al.Key != "questexpiring.4" || !strings.Contains(al.Label, "2 h") {
		t.Fatalf("alert %+v ok %v", al, ok)
	}
	if AlertExplain(al.Key) == AlertExplain("nope") {
		t.Fatal("no explanation")
	}
	w.Quests = w.Quests[:2]
	if _, ok := QuestExpiringAlertIn(en, w, now); ok {
		t.Fatal("alert up with no offer near its end")
	}
}

func TestQuestExpiringAlertClickShowsQuest(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	c.Alerts().Focus("alert.questexpiring.9")
	if len(f.intents) != 1 || f.intents[0] != (ports.ShowQuest{ID: 9}) {
		t.Fatalf("intents %+v", f.intents)
	}
}
