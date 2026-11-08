package windows

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/pawn/skill"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
)

// The fake shell names factions 1 and 2 (the Quests window's foot and args).
func (s *fakeShell) FactionName(ref int) string {
	if ref == 1 || ref == 2 {
		return fmt.Sprintf("Faction %d", ref)
	}
	return ""
}

// TestQuestsLayoutMatchesTheTokens (QST-120..125, 139): the list column is
// 351 wide with three equal 32-px tabs, the search field and the
// preferences button sit 6 px apart under the list, and the detail pane
// is 607 wide, 16 px right of the column, filling the body.
func TestQuestsLayoutMatchesTheTokens(t *testing.T) {
	t.Parallel()
	body := gfx.Rect{X: 18, Y: 18, W: hudlayout.QuestsW - 2*hudlayout.WindowPad, H: hudlayout.QuestsH - 2*hudlayout.WindowPad}
	l := questsLayout(body)
	for i, r := range l.Tabs {
		if r.H != hudlayout.QuestsTabH || r.W != hudlayout.QuestsListW/3 || r.Y != body.Y || r.X != body.X+float32(i)*r.W {
			t.Fatalf("tab %d %+v", i, r)
		}
	}
	if l.List.W != hudlayout.QuestsListW || l.List.Y != body.Y+hudlayout.QuestsTabH || l.List.Bottom()+questUnderGap != l.Search.Y {
		t.Fatalf("list %+v search %+v", l.List, l.Search)
	}
	if uikit.FieldInner(l.Search).H < float32(gfx.LineH(gfx.Body)) || l.Search.H < hudlayout.QuestSearchH || l.Search.Bottom()+questUnderGap != l.Prefs.Y || l.Prefs.H != hudlayout.QuestPrefsBtnH || l.Prefs.Bottom() != body.Bottom() {
		t.Fatalf("search %+v prefs %+v body bottom %v", l.Search, l.Prefs, body.Bottom())
	}
	if l.Pane.X != body.X+hudlayout.QuestsListW+hudlayout.QuestsGap || l.Pane.W != hudlayout.QuestsDetailW || l.Pane.H != body.H || l.Pane.Right() != body.Right() {
		t.Fatalf("pane %+v in body %+v", l.Pane, body)
	}
	small := questsLayout(gfx.Rect{W: 300, H: 60}) // a squeezed window: nothing negative
	if small.List.H < 0 || small.Pane.W < 0 {
		t.Fatalf("squeezed %+v", small)
	}
}

// TestRewardPanelHeightFollowsChipLines (QST-128): 44 / 72 / 100 for 1..3
// lines, one chip pitch more per line after.
func TestRewardPanelHeightFollowsChipLines(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]float32{0: 44, 1: 44, 2: 72, 3: 100, 4: 128} {
		if got := rewardPanelH(n); got != want {
			t.Errorf("rewardPanelH(%d) = %v, want %v", n, got, want)
		}
	}
}

// TestRewardChipsWrapInsideThePanel (QST-128): chips start 11 px right of
// the 98-px Accept button (10 px in without one), sit 4 px apart on 28-px
// lines, wrap before the panel's right pad, and the panel grows with them.
func TestRewardChipsWrapInsideThePanel(t *testing.T) {
	t.Parallel()
	p := layoutReward([]float32{150, 150, 150, 150, 900}, 0, 100, 600, true)
	if p.Panel.W != hudlayout.QuestRewardW || p.Accept.W != hudlayout.QuestAcceptW || p.Accept.H != p.Panel.H {
		t.Fatalf("panel %+v accept %+v", p.Panel, p.Accept)
	}
	c := p.Chips
	if c[0].X != hudlayout.QuestAcceptW+questAcceptGap || c[0].Y != 100+questChipPad || c[0].H != questChipH {
		t.Fatalf("first chip %+v", c[0])
	}
	if c[1].X != c[0].Right()+questChipGap || c[1].Y != c[0].Y {
		t.Fatalf("second chip %+v after %+v", c[1], c[0])
	}
	if c[2].Y != c[0].Y+questChipH+questChipGap || c[2].X != c[0].X {
		t.Fatalf("the third chip does not wrap: %+v", c[2])
	}
	right := p.Panel.Right() - questChipPad
	for i, r := range c {
		if r.Right() > right {
			t.Fatalf("chip %d %+v leaves the panel (right %v)", i, r, right)
		}
	}
	if p.Panel.H != rewardPanelH(3) {
		t.Fatalf("3 lines in a %v panel", p.Panel.H)
	}
	q := layoutReward([]float32{80}, 0, 0, 600, false)
	if !q.Accept.Empty() || q.Chips[0].X != questChipPad || q.Panel.H != hudlayout.QuestRewardH1 {
		t.Fatalf("accepted panel %+v chips %+v", q, q.Chips)
	}
}

// TestQuestTabSortsEveryState (QST-130): offers under Available, running
// quests under Active, ended or dismissed ones under Historical.
func TestQuestTabSortsEveryState(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s    qd.State
		dism bool
		want int
	}{
		{qd.Offered, false, questsAvail}, {qd.Active, false, questsActive}, {qd.Succeeded, false, questsHistory},
		{qd.Failed, false, questsHistory}, {qd.Expired, false, questsHistory}, {qd.Offered, true, questsHistory}, {qd.Active, true, questsHistory},
	} {
		if got := questTab(c.s, c.dism); got != c.want {
			t.Errorf("questTab(%v, %v) = %d, want %d", c.s, c.dism, got, c.want)
		}
	}
}

// TestQuestArgsFormatByKind: every Arg kind turns into readable text.
func TestQuestArgsFormatByKind(t *testing.T) {
	t.Parallel()
	sh, _ := newFake()
	now := uint64(clock.TicksPerDay)
	for _, c := range []struct {
		a    quest.Arg
		want string
	}{
		{quest.Num(12), "12"},
		{quest.Text("Ana"), "Ana"},
		{quest.Faction(2), "Faction 2"},
		{quest.Faction(9), "an unknown faction"},
		{quest.Tick(now + 3*clock.TicksPerDay), "in 3.0 days"},
		{quest.Tick(now + 5*clock.TicksPerHour), "in 5 hours"},
		{quest.Tick(clock.TicksPerHour * 14), "on " + historyDate(clock.TicksPerHour*14)},
		{quest.Tile(431), "tile 431"},
		{quest.Item(item.Wood), "Wood"},
	} {
		if got := questArg(sh, c.a, now); got != c.want {
			t.Errorf("questArg(%+v) = %q, want %q", c.a, got, c.want)
		}
	}
	got := questLine(sh, quest.Line{Key: "quest.stash.item", Args: []quest.Arg{quest.Item(item.Steel), quest.Num(40)}}, now)
	if got != "Inside: Steel, 40 of them." {
		t.Fatalf("stash line %q", got)
	}
	if got := questLine(sh, quest.Line{Key: "quest.nobody.knows", Args: []quest.Arg{quest.Num(3)}}, now); got != "quest.nobody.knows 3" {
		t.Fatalf("unknown key shows %q", got)
	}
}

// TestEveryQuestKeyHasOurWords: each "quest.<kind>.<part>" key the kinds
// emit has a format here (a new key fails until it gets one).
func TestEveryQuestKeyHasOurWords(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("../../../../application/game/quest/kinds/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no kind sources: %v", err)
	}
	re := regexp.MustCompile(`"(quest\.[a-z]+\.[a-z]+)"`)
	sh, _ := newFake()
	seen := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			seen++
			args := []any{"a", "b"}
			if s, ok := questLineKnown(sh, m[1], args); !ok || s == "" || strings.Contains(s, "%!") {
				t.Errorf("%s: key %s has no words (%q)", filepath.Base(f), m[1], s)
			}
		}
	}
	if seen < 30 {
		t.Fatalf("found only %d keys", seen)
	}
}

// TestQuestTimesAndTint (QST-123, 130): an offer's time left, urgent under
// a day; history rows tinted by outcome, none when dismissed.
func TestQuestTimesAndTint(t *testing.T) {
	t.Parallel()
	sh, _ := newFake()
	now := uint64(10 * clock.TicksPerDay)
	offer := game.QuestView{State: qd.Offered, AcceptBy: now + 6*clock.TicksPerDay}
	if left, ok := questLeft(offer, now); !ok || questShort(sh, left) != "6d" || questUrgent(offer, now) {
		t.Fatalf("6-day offer: %v %v %q urgent %v", left, ok, questShort(sh, left), questUrgent(offer, now))
	}
	offer.AcceptBy = now + 22*clock.TicksPerHour
	if left, _ := questLeft(offer, now); questShort(sh, left) != "22h" || !questUrgent(offer, now) {
		t.Fatalf("22-hour offer %q not urgent", questShort(sh, left))
	}
	if _, ok := questLeft(game.QuestView{State: qd.Failed, Deadline: now + 5}, now); ok {
		t.Fatal("an ended quest has a timer")
	}
	if c, ok := questTint(game.QuestView{State: qd.Failed}); !ok || c != uikit.AlertDangerBg {
		t.Fatalf("failed tint %v %v", c, ok)
	}
	for _, s := range []qd.State{qd.Succeeded, qd.Expired} {
		if c, ok := questTint(game.QuestView{State: s}); !ok || c != uikit.AlertWarnBg {
			t.Fatalf("%v tint %v %v", s, c, ok)
		}
	}
	if _, ok := questTint(game.QuestView{State: qd.Failed, Dismissed: true}); ok {
		t.Fatal("a dismissed row is tinted")
	}
	lines := questTimeLines(sh, game.QuestView{State: qd.Failed, Reason: qd.Timeout, EndedAt: now - 2*clock.TicksPerDay}, now)
	if len(lines) != 1 || lines[0] != "Failed 2.0 days ago: time ran out" {
		t.Fatalf("failed lines %q", lines)
	}
}

// questFixture is a world with one offer of two options, one active and
// one failed quest.
func questFixture() game.WorldView {
	return game.WorldView{
		Quests: []game.QuestView{
			{ID: 3, Kind: qd.Stash, State: qd.Offered, Stars: 2, Giver: 1, AcceptBy: 6 * clock.TicksPerDay, Chosen: -1, Tile: 77,
				Rewards: []quest.RewardView{
					{Lines: []trade.Line{{Kind: item.Steel, Qty: 80}}, Value: 160},
					{Goodwill: 20, HasPawn: true, Pawn: quest.PawnPreview{Name: "Ana", Age: 31, Backstory: "Courier"}},
				},
				Detail: []quest.Line{{Key: "quest.stash.site", Args: []quest.Arg{quest.Tile(77)}}}},
			{ID: 4, Kind: qd.Request, State: qd.Active, Giver: 2, Chosen: -1, Tile: -1},
			{ID: 5, Kind: qd.Camp, State: qd.Failed, Reason: qd.GiverHostile, Chosen: -1, Tile: -1},
		},
		NoGoodwill: []int{2},
	}
}

// questsOpen opens the window on the fixture and declares it once.
func questsOpen(t *testing.T) (*fakeShell, *Set) {
	t.Helper()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.world = questFixture()
	w.Quests().Launch()
	if w.KitID() != QuestsModalID {
		t.Fatalf("window %q open", w.KitID())
	}
	sh.kit.Begin()
	w.Declare()
	return sh, w
}

// TestQuestsWindowSendsItsCommands: the Accept buttons send AcceptQuest
// with the option, Dismiss sends DismissQuest, the links send OpenWorld
// and SwitchMap; the window changes no state itself.
func TestQuestsWindowSendsItsCommands(t *testing.T) {
	t.Parallel()
	sh, w := questsOpen(t)
	for _, id := range []string{ids.QuestsAvail, ids.QuestsActive, ids.QuestsHistory, ids.QuestsSearch, ids.QuestsPrefs, ids.QuestRow + "3"} {
		if _, ok := sh.kit.Find(id); !ok {
			t.Fatalf("no widget %q", id)
		}
	}
	if b, _ := sh.kit.Find(ids.QuestsAvail); b.Label != "Available (1)" {
		t.Fatalf("tab label %q", b.Label)
	}
	for _, c := range []struct {
		id   string
		want command.Command
	}{
		{ids.QuestAccept + "3.1", command.Command{Kind: command.AcceptQuest, Index: 3, Count: 1}},
		{ids.QuestAccept + "3.0", command.Command{Kind: command.AcceptQuest, Index: 3, Count: 0}},
		{ids.QuestDismiss + "3", command.Command{Kind: command.DismissQuest, Index: 3}},
	} {
		sh.cmds = nil
		if !sh.kit.Click(c.id) {
			t.Fatalf("no button %q", c.id)
		}
		if len(sh.cmds) != 1 || sh.cmds[0] != c.want {
			t.Fatalf("%s sent %+v, want %+v", c.id, sh.cmds, c.want)
		}
	}
	if !sh.kit.Click(ids.QuestJump + "3") {
		t.Fatal("no site link")
	}
	if !slices.Contains(sh.intents, ports.Intent(ports.OpenWorld{Select: "tile:77"})) {
		t.Fatalf("site link sent %+v", sh.intents)
	}
	w.Quests().Launch()
	sh.kit.Begin()
	w.Declare()
	if !sh.kit.Click(ids.QuestColony + "3") {
		t.Fatal("no colony link")
	}
	if last := sh.intents[len(sh.intents)-1]; last != (ports.SwitchMap{Map: homeMap}) || w.KitOpen() {
		t.Fatalf("colony link sent %+v, window open %v", last, w.KitOpen())
	}
}

// TestQuestsWindowTabsSearchAndRestore: Historical lists the failed quest
// with a Restore button once it is dismissed; the search filters by name.
func TestQuestsWindowTabsSearchAndRestore(t *testing.T) {
	t.Parallel()
	sh, w := questsOpen(t)
	sh.world.Quests[2].Dismissed = true
	sh.kit.Click(ids.QuestsHistory)
	sh.kit.Begin()
	w.Declare()
	if w.quests.tab != questsHistory {
		t.Fatalf("tab %d", w.quests.tab)
	}
	sh.cmds = nil
	if !sh.kit.Click(ids.QuestRestore + "5") {
		t.Fatal("no restore button on the dismissed quest")
	}
	if len(sh.cmds) != 1 || sh.cmds[0] != (command.Command{Kind: command.RestoreQuest, Index: 5}) {
		t.Fatalf("restore sent %+v", sh.cmds)
	}
	title := func(q game.QuestView) string { return questTitle(sh, q) }
	all := sh.world.Quests
	if got := questRows(all, questsAvail, title, strings.ToUpper(quest.KindName(qd.Stash)[:4])); len(got) != 1 {
		t.Fatalf("search kept %d rows", len(got))
	}
	if got := questRows(all, questsAvail, title, "zzz"); len(got) != 0 {
		t.Fatalf("search zzz kept %d rows", len(got))
	}
	if questPick(nil, 3) != -1 || questPick(all[:2], 4) != 1 || questPick(all[:2], 99) != 0 {
		t.Fatal("questPick")
	}
}

// TestQuestsDetailScrollResetsOnANewPick (QST-125).
func TestQuestsDetailScrollResetsOnANewPick(t *testing.T) {
	t.Parallel()
	sh, w := questsOpen(t)
	sh.kit.Scroll[questsDetailID] = 40
	sh.kit.Click(ids.QuestsActive)
	sh.kit.Begin()
	w.Declare()
	if w.quests.shown != 4 || sh.kit.Scroll[questsDetailID] != 0 {
		t.Fatalf("shown %d scroll %v", w.quests.shown, sh.kit.Scroll[questsDetailID])
	}
}

// TestQuestPrefsRowsAndToggle (QST-135): one row per named faction with a
// table row, ticked unless NoGoodwill holds it; a click sends SetRewardPref.
func TestQuestPrefsRowsAndToggle(t *testing.T) {
	t.Parallel()
	table := []Faction{{Name: "Faction 1"}, {Name: "Faction 2"}}
	rows := prefRows(func(ref int) string { return []string{"", "Faction 1", "Faction 2", "Hidden"}[min(ref, 4)%4] }, table, []int{2})
	if len(rows) != 2 || !rows[0].accepted || rows[1].accepted || rows[1].ref != 2 {
		t.Fatalf("rows %+v", rows)
	}
	f, _ := newFake()
	sh := facShell{f, 0}
	w := New(prefShell{sh})
	f.env.Game = fakeReader{}
	f.world = questFixture()
	w.Quests().Launch()
	w.Quests().openPrefs()
	f.kit.Begin()
	f.kit.RunModals(nil)
	if !slices.ContainsFunc(f.kit.Modals, func(m uikit.Modal) bool { return m.ID == QuestPrefsModalID }) {
		t.Fatal("the dialog is not open")
	}
	f.cmds = nil
	if !f.kit.Click(ids.QuestPref + "2") {
		t.Fatal("no toggle for faction 2")
	}
	f.kit.Begin()
	f.kit.RunModals(nil)
	if len(f.cmds) != 1 || f.cmds[0] != (command.Command{Kind: command.SetRewardPref, Index: 2, Enabled: true}) {
		t.Fatalf("toggle sent %+v", f.cmds)
	}
}

// prefShell is a fake shell whose Factions table has factions 1 and 2.
type prefShell struct{ facShell }

func (prefShell) Factions() []Faction {
	return []Faction{{Name: "Faction 1", Standing: "Neutral"}, {Name: "Faction 2", Standing: "Hostile"}}
}

// TestQuestRunsColourTheArgs (QST-127): a detail line's runs come from its
// typed arguments: the giver warm, another faction in its relation colour,
// numbers highlighted, the words between plain; the text is the line's.
func TestQuestRunsColourTheArgs(t *testing.T) {
	t.Parallel()
	sh, _ := newFake()
	now := uint64(clock.TicksPerDay)
	l := quest.Line{Key: "quest.rescue.site", Args: []quest.Arg{quest.Tile(12), quest.Faction(2)}}
	rs := questRuns(sh, l, 1, now)
	if runsText(rs) != questLine(sh, l, now) || runsText(rs) != "Someone is locked up at tile 12, held by Faction 2." {
		t.Fatalf("runs %q, line %q", runsText(rs), questLine(sh, l, now))
	}
	roles := map[string]runRole{}
	for _, r := range rs {
		roles[r.s] = r.role
	}
	if roles["Faction 2"] != runFaction || roles["tile 12"] != runPlain || roles["Someone is locked up at "] != runPlain {
		t.Fatalf("roles %v", roles)
	}
	giver := questRuns(sh, quest.Line{Key: "quest.camp.giver", Args: []quest.Arg{quest.Faction(1)}}, 1, now)
	if giver[0].role != runGiver || giver[0].s != "Faction 1" {
		t.Fatalf("giver runs %+v", giver)
	}
	num := questRuns(sh, quest.Line{Key: "quest.camp.standing", Args: []quest.Arg{quest.Num(3), quest.Num(8)}}, 1, now)
	if len(num) != 5 || num[1].role != runNumber || num[3].role != runNumber || num[3].s != "8" {
		t.Fatalf("number runs %+v", num)
	}
	facs := []Faction{{Name: "Faction 2", Color: uikit.Danger}}
	if runColor(rs[3], sh.FactionName, facs) != uikit.Danger || runColor(giver[0], sh.FactionName, facs) != uikit.Accent ||
		runColor(num[1], sh.FactionName, facs) != uikit.Heading || runColor(rs[0], sh.FactionName, facs) != uikit.Text {
		t.Fatal("a role has the wrong colour")
	}
	unk := questRuns(sh, quest.Line{Key: "quest.nobody.knows", Args: []quest.Arg{quest.Faction(2)}}, 1, now)
	if runsText(unk) != "quest.nobody.knows Faction 2" || unk[2].role != runFaction {
		t.Fatalf("unknown key runs %+v", unk)
	}
}

// TestRunsWrapInsideTheWidth (QST-127): coloured runs wrap at spaces inside
// the width, a word split over two runs ("Faction 2" then ".") stays on one
// line, and a word wider than the line is cut, never drawn past it.
func TestRunsWrapInsideTheWidth(t *testing.T) {
	t.Parallel()
	st := gfx.Body
	rs := []questRun{{s: "A traveller from "}, {s: "People of the Quorhollow", role: runGiver}, {s: ". They ask to live with us and bring "}, {s: "12", role: runNumber}, {s: " goats."}}
	for _, maxW := range []float32{60, 140, 260, 555} {
		spans, lines := layoutRuns(rs, st, maxW)
		var b strings.Builder
		last := -1
		for i, sp := range spans {
			if r := sp.x + gfx.TextW(sp.s, st); r > maxW+0.5 && gfx.TextW(sp.s, st) <= maxW {
				t.Fatalf("w %v: span %q ends at %v", maxW, sp.s, r)
			}
			if sp.line < last || sp.line >= lines {
				t.Fatalf("w %v: span %d on line %d of %d", maxW, i, sp.line, lines)
			}
			if sp.s == "." && i > 0 && spans[i-1].line != sp.line {
				t.Fatalf("w %v: the full stop left its word", maxW)
			}
			last = sp.line
			b.WriteString(sp.s)
		}
		if got, want := b.String(), strings.ReplaceAll(runsText(rs), " ", ""); got != want {
			t.Fatalf("w %v: spans spell %q, want %q", maxW, got, want)
		}
		if plain := len(gfx.WrapPx(runsText(rs), st, float64(maxW))); maxW >= 140 && lines != plain || maxW == 60 && lines < 4 {
			t.Fatalf("w %v: %d lines", maxW, lines)
		}
	}
	long, lines := layoutRuns([]questRun{{s: strings.Repeat("w", 80), role: runFaction}}, st, 100)
	for _, sp := range long {
		if sp.x+gfx.TextW(sp.s, st) > 100.5 {
			t.Fatalf("cut word part %q ends past the line", sp.s)
		}
	}
	if lines < 2 {
		t.Fatalf("an 80-rune word fits %d line", lines)
	}
}

// TestPawnChipShowsItsCard (QST-137): the pawn reward's chip makes room for
// a 16-px portrait and has no tooltip; its card is 214 wide, 1 px under the
// chip at the mouse, with three blocks a line and a blank apart (the best
// skill's label highlighted), and flips above the chip near the bottom.
func TestPawnChipShowsItsCard(t *testing.T) {
	t.Parallel()
	sh, w := questsOpen(t)
	q := sh.world.Quests[0]
	q.Rewards[1].Pawn.Best, q.Rewards[1].Pawn.BestLevel = skill.Medicine, 9
	d := w.Quests().layoutDetail(q, hudlayout.QuestRewardW)
	var chip *questChip
	for i := range d.chips {
		if d.chips[i].pawn != nil {
			chip = &d.chips[i]
		}
	}
	if chip == nil || chip.label != "Ana joins" || chip.tip != "" {
		t.Fatalf("pawn chip %+v", chip)
	}
	if min := gfx.TextW(chip.label, gfx.Body) + 2*questChipTextX + pawnChipIcon + pawnChipIconGo; chip.r.W < min {
		t.Fatalf("pawn chip %v wide, want room for the portrait (%v)", chip.r.W, min)
	}
	if wd, ok := sh.kit.Find("quest.chip.3.3"); !ok || wd.Label != "Ana joins" || wd.Tip != "" {
		t.Fatalf("pawn chip widget %+v (%v)", wd, ok)
	}
	txt := pawnCard(sh, *chip.pawn)
	if txt.head != "Ana, new colonist" || txt.skillLabel != "Best skill:" || txt.skill != "Medicine (level 9)" || txt.story != "Courier, 31 years old" {
		t.Fatalf("card text %+v", txt)
	}
	at := gfx.Rect{X: 400, Y: 300, W: 99, H: 24}
	box, lines := layoutPawnCard(txt, at, 450, 1280, 720)
	lh := float32(gfx.LineH(gfx.Body))
	if box.X != 450 || box.Y != at.Bottom()+1 || box.W != pawnCardW || box.H != 2*pawnCardPadY+5*lh {
		t.Fatalf("card box %+v", box)
	}
	ys := map[string]float32{}
	for _, l := range lines {
		ys[l.s] = l.y
		if l.x+gfx.TextW(l.s, gfx.Body) > pawnCardW-pawnCardPadX+0.5 {
			t.Fatalf("line %q leaves the card", l.s)
		}
		if l.s == "Best skill:" && l.c != uikit.Accent {
			t.Fatal("the best skill's label is not highlighted")
		}
	}
	if ys["Ana, new colonist"] != pawnCardPadY || ys["Best skill:"] != pawnCardPadY+2*lh || ys["Medicine (level 9)"] != ys["Best skill:"] || ys["Courier, 31 years old"] != pawnCardPadY+4*lh {
		t.Fatalf("block tops %v", ys)
	}
	low, _ := layoutPawnCard(txt, gfx.Rect{X: 1200, Y: 680, W: 99, H: 24}, 1250, 1280, 720)
	if low.Bottom() > 680 || low.Right() > 1280-pawnCardEdge {
		t.Fatalf("card near the corner %+v", low)
	}
}

// TestQuestPrefsDialogIsFrameless (QST-135): no title bar and no footer;
// the X sits 22 x 25 in the frame's top-right corner and closes the
// dialog; each row's checkbox sits at its measured column.
func TestQuestPrefsDialogIsFrameless(t *testing.T) {
	t.Parallel()
	f, _ := newFake()
	w := New(prefShell{facShell{f, 0}})
	f.env.Game = fakeReader{}
	f.world = questFixture()
	w.Quests().Launch()
	w.Quests().openPrefs()
	f.kit.Begin()
	f.kit.RunModals(nil)
	m := f.kit.Modals[len(f.kit.Modals)-1]
	if m.ID != QuestPrefsModalID || !m.Frameless || len(m.Buttons) != 0 || m.W != hudlayout.QuestPrefsW || m.H != hudlayout.QuestPrefsH {
		t.Fatalf("dialog %+v", m)
	}
	sw, shh := f.Size()
	frame, _, _, _ := uikit.ModalGeometry(sw, shh, m)
	x, ok := f.kit.Find(QuestPrefsCloseID)
	if !ok || x.R.W != questPrefXW || x.R.H != questPrefXH || x.R.Right() != frame.Right()-questPrefXInset || x.R.Y != frame.Y+questPrefXInset {
		t.Fatalf("X %+v in frame %+v (%v)", x.R, frame, ok)
	}
	if c, ok := f.kit.Find(ids.QuestPref + "1"); !ok || c.R.X != frame.X+hudlayout.WindowPad+questPrefCheckX {
		t.Fatalf("checkbox %+v in frame %+v", c.R, frame)
	}
	if !f.kit.Click(QuestPrefsCloseID) {
		t.Fatal("no X")
	}
	f.kit.Begin()
	f.kit.RunModals(nil)
	if f.kit.Top() == QuestPrefsModalID {
		t.Fatal("the X left the dialog open")
	}
}
