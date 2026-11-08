package hud

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/command"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
)

func (f *fakeShell) Do(i ports.Intent) { f.intents = append(f.intents, i) }

// A quest letter's footer is exactly two rows (QST-133): an offer's "View
// quest" then "Close"; a failed quest's "Close" then "View related quest".
// A plain letter has none.
func TestQuestLetterFooterOrder(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	if got := c.Right().letterFooter(game.LetterView{ID: 3, X: -1, Y: -1}); len(got) != 1 {
		t.Fatalf("a plain letter: %+v", got)
	}
	label := func(bs []uikit.ModalButton) []string {
		var out []string
		for _, b := range bs {
			out = append(out, b.Label)
		}
		return out
	}
	offer := c.Right().letterFooter(game.LetterView{ID: 4, Kind: "info", Quest: 7, X: 1, Y: 1})
	if got := label(offer); !slices.Equal(got, []string{"View quest", "Close"}) {
		t.Fatalf("an offer: %v", got)
	}
	offer[0].OnClick()
	if len(f.intents) != 1 || f.intents[0] != (ports.ShowQuest{ID: 7}) {
		t.Fatalf("intents %+v", f.intents)
	}
	failed := c.Right().letterFooter(game.LetterView{ID: 5, Kind: "caution", Quest: 8, X: -1, Y: -1})
	if got := label(failed); !slices.Equal(got, []string{"Close", "View related quest"}) {
		t.Fatalf("a failed quest: %v", got)
	}
}

// The tile of a quest letter opens its quest and dismisses the letter; a
// plain letter's tile opens the letter window (QST-131).
func TestQuestLetterClickOpensQuestAndDismisses(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	f.w.Letters = []game.LetterView{{ID: 4, Kind: "info", Title: "Quest: x", Quest: 7, X: -1, Y: -1}}
	c.Right().LetterClick("letter.4")
	if len(f.intents) != 1 || f.intents[0] != (ports.ShowQuest{ID: 7}) {
		t.Fatalf("intents %+v", f.intents)
	}
	if len(f.cmds) != 1 || f.cmds[0].Kind != command.DismissLetter || f.cmds[0].Target != 4 {
		t.Fatalf("commands %+v", f.cmds)
	}
}

// A letter that lapses shows the hours left, rounded up, in its label.
func TestLetterLapseSuffix(t *testing.T) {
	t.Parallel()
	now := uint64(3 * clock.TicksPerDay)
	if letterHoursLeft(0, now) != 0 || letterHoursLeft(now, now) != 0 || letterHoursLeft(now+1, now) != 1 ||
		letterHoursLeft(now+clock.TicksPerHour, now) != 1 || letterHoursLeft(now+clock.TicksPerHour+1, now) != 2 {
		t.Fatal("hours left are not rounded up")
	}
	r := LetterRow{Letter: game.LetterView{Title: "Wanderer"}, Hours: 5}
	if got := LetterRowLabel(r); got != "Wanderer (5h)" {
		t.Fatalf("label %q", got)
	}
	c, f := newFake()
	f.lay = hudlayout.Layout{Letters: gfx.Rect{X: 0, Y: 0, W: 200, H: 200}, LetterRows: 3}
	f.w.Letters = []game.LetterView{{ID: 1, Title: "Wanderer", Until: now + 3*clock.TicksPerHour, X: -1, Y: -1}}
	rows := c.Right().LetterRows(f.lay)
	if len(rows) != 1 || rows[0].Hours != 3 {
		t.Fatalf("rows %+v", rows)
	}
}

// A long quest letter's card text is cut to the card (QST-132): the last
// line that fits ends in an ellipsis.
func TestQuestLetterCardCutsLongText(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("A new quest wants an answer before the offer lapses. ", 20)
	box, _, lines := LayoutLetterCard(gfx.Rect{X: 1000, Y: 300, W: 38, H: 30}, "Quest: Hidden cache (5h)", body, 1920, 1080)
	if len(lines) == 0 || !strings.HasSuffix(lines[len(lines)-1], "…") {
		t.Fatalf("lines %q", lines)
	}
	if h := float32(len(lines))*float32(gfx.LineH(gfx.Small)) + float32(gfx.LineH(gfx.Label)); h > box.H {
		t.Fatalf("%d lines (%v px) overflow the %v px card", len(lines), h, box.H)
	}
}

// Alert rows wrap to two lines (QST-134): a row is lines*16+6 high, a
// one-line row keeps 22, the rows below follow, and the hit rects are the
// drawn rows.
func TestAlertRowsWrapToTwoLines(t *testing.T) {
	t.Parallel()
	c, f := newFake()
	long := strings.Repeat("blueprints waiting ", 12)
	if got := alertLines(long, 150); len(got) != 2 || !strings.HasSuffix(got[1], "…") {
		t.Fatalf("lines %q", got)
	}
	if alertRowH(1) != hudlayout.AlertRowH || alertRowH(2) != 38 {
		t.Fatalf("heights %v %v", alertRowH(1), alertRowH(2))
	}
	f.lay = hudlayout.Layout{Alerts: gfx.Rect{X: 1766, Y: 100, W: 154, H: 200}, AlertRows: 9}
	f.w.Quests = []game.QuestView{{ID: 3, Kind: 1, State: qd.Offered, AcceptBy: 3*clock.TicksPerDay + 2*clock.TicksPerHour}}
	f.w.ResearchAlert = true
	rows := c.Alerts().Rows(f.lay)
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	for i, r := range rows {
		if r.R.H != alertRowH(len(r.Lines)) {
			t.Fatalf("row %d: height %v for %d lines", i, r.R.H, len(r.Lines))
		}
	}
	if rows[1].R.Y != rows[0].R.Y+rows[0].R.H {
		t.Fatalf("row 2 at %v, row 1 ends at %v", rows[1].R.Y, rows[0].R.Y+rows[0].R.H)
	}
	for i, b := range c.Alerts().ButtonsIn(f.lay) {
		if b.Y != rows[i].R.Y || b.H != rows[i].R.H {
			t.Fatalf("button %d %+v is not row %+v", i, b, rows[i].R)
		}
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
