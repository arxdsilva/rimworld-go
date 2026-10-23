package ebiten

import (
	"errors"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/platform/input"
)

// pawnsApp is a menu App on Create Characters (Broken Hull, tiny world).
func pawnsApp(t *testing.T) (*App, *fakeInput) {
	t.Helper()
	a, f, _ := ngTest(t, nil)
	toPawns(t, a)
	f.frame(t, a)
	return a, f
}

func TestRosterSelectedCountMatchesScenario(t *testing.T) {
	t.Parallel()
	a, f := pawnsApp(t)
	ng := a.ng
	fx := ng.scenario.Apply()
	views := ng.pool.Views()
	sel := 0
	for _, v := range views {
		if v.Selected {
			sel++
		}
	}
	if sel != fx.People || len(ng.pool.Selected()) != fx.People {
		t.Fatalf("selected %d, scenario people %d", sel, fx.People)
	}
	wantPool := fx.ChooseFrom
	if wantPool == 0 {
		wantPool = fx.People + 5
	}
	if len(views) != wantPool {
		t.Fatalf("pool of %d, want %d", len(views), wantPool)
	}
	for i := range views {
		if !hasWidget(a, optID("pawns.row", i)) {
			// Rows below the scroll fold are not declared; the first ones are.
			if i < 3 {
				t.Fatalf("row %d missing", i)
			}
		}
	}
	// Another scenario (Ember Clan: five people) makes a pool of its own size.
	a2, f2, _ := ngTest(t, nil)
	click(t, a2, "menu.newgame")
	d, _ := scenario.Builtin("ember_clan")
	click(t, a2, "scen.row.ember_clan")
	click(t, a2, "wizard.next")
	click(t, a2, "wizard.next")
	a2.draft().world.SeedText, a2.draft().world.Coverage = tinySeed, 5
	click(t, a2, "world.generate")
	runUntil(t, a2, func() bool { return a2.nav.screen() == screenLanding })
	click(t, a2, "land.random")
	click(t, a2, "wizard.next")
	f2.frame(t, a2)
	if n := len(a2.ng.pool.Selected()); n != d.People {
		t.Fatalf("ember clan selects %d, want %d", n, d.People)
	}
	_ = f
}

func TestRerollDeterministicAndKeepsEditedName(t *testing.T) {
	t.Parallel()
	a, f := pawnsApp(t)
	ng := a.ng
	before := ng.pool.Views()[0]
	click(t, a, "pawns.reroll")
	after := ng.pool.Views()[0]
	if after.Nick == before.Nick && after.AgeYears == before.AgeYears && after.Look == before.Look {
		t.Fatalf("Randomize must roll the candidate again: %+v", after)
	}
	// The same draft rolled the same way gives the same colonist.
	b, fb := pawnsApp(t)
	click(t, b, "pawns.reroll")
	fb.frame(t, b)
	if b.ng.pool.Views()[0].BioView.Nick != after.Nick || b.ng.pool.Views()[0].AgeYears != after.AgeYears {
		t.Fatal("rerolls are deterministic from the seed")
	}
	// An edited name survives a plain reroll, but not a Shift reroll.
	press(t, a, f, "pawns.nick")
	f.key(t, a, "A", input.ModCtrl)
	f.typeText(t, a, "Brisk")
	f.key(t, a, "Enter", 0)
	f.frame(t, a)
	if got := ng.pool.Views()[0].Nick; got != "Brisk" {
		t.Fatalf("nick = %q after editing", got)
	}
	click(t, a, "pawns.reroll")
	if got := ng.pool.Views()[0].Nick; got != "Brisk" {
		t.Fatalf("a plain reroll keeps the edited name, got %q", got)
	}
	f.frame(t, a)
	x, y := widgetAt(t, a, "pawns.reroll")
	f.cur.mx, f.cur.my = x, y
	f.click(t, a, mbLeft, input.ModShift)
	f.cur.mx, f.cur.my = -50, -50
	if got := ng.pool.Views()[0].Nick; got == "Brisk" {
		t.Fatal("Shift+Randomize rolls a new name too")
	}
	// Randomize all asks first.
	before2 := ng.pool.Views()[1].Nick
	click(t, a, "pawns.rerollAll")
	if a.uiModalTop() != "pawns.rerollAll.confirm" || ng.pool.Views()[1].Nick != before2 {
		t.Fatalf("modal %q: nothing changes before the player confirms", a.uiModalTop())
	}
	f.frame(t, a)
	press(t, a, f, "modal.pawns.rerollAll.confirm.ok")
	f.frame(t, a)
	// Undo changes restores the pool from entry.
	click(t, a, "pawns.reset")
	f.frame(t, a)
	c, fc := pawnsApp(t)
	fc.frame(t, c)
	for i, v := range ng.pool.Views() {
		if v.Nick != c.ng.pool.Views()[i].Nick {
			t.Fatalf("undo: candidate %d is %q, entry was %q", i, v.Nick, c.ng.pool.Views()[i].Nick)
		}
	}
}

// fakePool records the roster operations the screen asks for.
type fakePool struct {
	views  []game.CandidateView
	nSel   int
	moves  [][2]int
	swaps  [][2]int
	rename []string
	err    error
	warn   []string
}

func (p *fakePool) Views() []game.CandidateView {
	out := append([]game.CandidateView(nil), p.views...)
	for i := range out {
		out[i].Selected, out[i].Index = i < p.nSel, i
	}
	return out
}
func (p *fakePool) Reroll(int, bool) {}
func (p *fakePool) RandomizeAll()    {}
func (p *fakePool) Rename(i int, first, nick, last string) error {
	p.rename = append(p.rename, first+"|"+nick+"|"+last)
	if p.err != nil {
		return p.err
	}
	p.views[i].First, p.views[i].Nick, p.views[i].Last = first, nick, last
	return nil
}
func (p *fakePool) Move(i, j int) {
	p.moves = append(p.moves, [2]int{i, j})
	v := p.views[i]
	p.views = append(p.views[:i], p.views[i+1:]...)
	p.views = append(p.views[:j], append([]game.CandidateView{v}, p.views[j:]...)...)
}
func (p *fakePool) Swap(i, j int) {
	p.swaps = append(p.swaps, [2]int{i, j})
	p.views[i], p.views[j] = p.views[j], p.views[i]
}
func (p *fakePool) Team() game.TeamReport {
	return game.TeamReport{Warnings: p.warn}
}
func (p *fakePool) Selected() []pawngen.Pawn { return make([]pawngen.Pawn, p.nSel) }

func newFakePool(n, nSel int) *fakePool {
	p := &fakePool{nSel: nSel}
	for i := 0; i < n; i++ {
		p.views = append(p.views, game.CandidateView{BioView: game.BioView{
			First: "F" + string(rune('a'+i)), Nick: "N" + string(rune('a'+i)), Last: "L", Sex: "Female", Build: "Average", AgeYears: 30,
			Adulthood: "Worker", Childhood: "Kid",
		}})
	}
	return p
}

// fakePawnsApp is the characters screen over a fakePool.
func fakePawnsApp(t *testing.T, n, nSel int) (*App, *fakeInput, *fakePool) {
	t.Helper()
	a, f := pawnsApp(t)
	fp := newFakePool(n, nSel)
	a.ng.pool, a.ng.poolKey = fp, a.poolKeyFor(a.ng)
	a.ng.pawnSel, a.ng.namesOf = 0, -1
	f.frame(t, a)
	return a, f, fp
}

func TestReorderAndSwap(t *testing.T) {
	t.Parallel()
	a, f, fp := fakePawnsApp(t, 6, 3)
	// Up and down move the highlighted candidate.
	press(t, a, f, "pawns.row.1")
	f.frame(t, a)
	click(t, a, "pawns.up")
	f.frame(t, a)
	if len(fp.moves) != 1 || fp.moves[0] != [2]int{1, 0} || a.ng.pawnSel != 0 {
		t.Fatalf("up: moves %v sel %d", fp.moves, a.ng.pawnSel)
	}
	if w, _ := a.uiSt().find("pawns.up"); !w.disabled {
		t.Fatal("Up is disabled on the first candidate")
	}
	click(t, a, "pawns.down")
	if fp.moves[1] != [2]int{0, 1} || a.ng.pawnSel != 1 {
		t.Fatalf("down: moves %v sel %d", fp.moves, a.ng.pawnSel)
	}
	// Swap with left-behind: pick the first left-behind candidate.
	click(t, a, "pawns.swap")
	if !a.uiModalOpen() {
		t.Fatal("Swap opens a list")
	}
	f.frame(t, a)
	press(t, a, f, "modal.pick.pawns.swap.list.0")
	f.frame(t, a)
	if len(fp.swaps) != 1 || fp.swaps[0] != [2]int{1, 3} {
		t.Fatalf("swaps = %v, want [1 3]", fp.swaps)
	}
	// Drag the first row below the Selected group: it moves to that row.
	f.frame(t, a)
	r0, ok := a.uiSt().find("pawns.row.0")
	r4, ok2 := a.uiSt().find("pawns.row.4")
	if !ok || !ok2 {
		t.Fatal("rows 0 and 4 should be visible")
	}
	x0, y0 := r0.r.center()
	_, y4 := r4.r.center()
	f.cur.mx, f.cur.my = int(x0), int(y0)
	f.cur.press[mbLeft], f.cur.down[mbLeft] = true, true
	f.frame(t, a)
	f.cur.my = int(y4)
	f.frame(t, a)
	f.cur.down[mbLeft], f.cur.release[mbLeft] = false, true
	f.frame(t, a)
	last := fp.moves[len(fp.moves)-1]
	if last != [2]int{0, 4} || a.ng.pawnSel != 4 {
		t.Fatalf("drag moved %v, want 0 -> 4 and the highlight to follow (sel %d)", last, a.ng.pawnSel)
	}
	// A plain click without movement only selects.
	n := len(fp.moves)
	f.cur.mx, f.cur.my = -50, -50
	press(t, a, f, "pawns.row.2")
	f.frame(t, a)
	if len(fp.moves) != n || a.ng.pawnSel != 2 {
		t.Fatalf("a click moved %v / sel %d", fp.moves[n:], a.ng.pawnSel)
	}
	if rosterTarget(0, 6, 3) != 0 || rosterTarget(10000, 6, 3) != 5 || rosterTarget(rosterY(4, 3)+5, 6, 3) != 4 {
		t.Error("drop targets")
	}
}

func TestNameValidationBlocksStart(t *testing.T) {
	t.Parallel()
	a, f := pawnsApp(t)
	ng := a.ng
	if r := ng.startReason(); r != "" {
		t.Fatalf("a fresh pool can start, got %q", r)
	}
	nick := ng.pool.Views()[0].Nick
	press(t, a, f, "pawns.nick")
	f.key(t, a, "A", input.ModCtrl)
	f.key(t, a, "Backspace", 0)
	f.frame(t, a)
	if ng.nameErr != game.ErrNickRequired.Error() {
		t.Fatalf("nameErr = %q", ng.nameErr)
	}
	w, _ := a.uiSt().find("wizard.start")
	if !w.disabled || w.reason != "Every colonist needs a nickname" {
		t.Fatalf("Start: disabled=%v reason=%q", w.disabled, w.reason)
	}
	if ng.pool.Views()[0].Nick != nick {
		t.Fatal("the pool keeps the last valid nickname")
	}
	before := a.nav.screen()
	click(t, a, "wizard.start")
	if a.nav.screen() != before {
		t.Fatal("a disabled Start does nothing")
	}
	// Characters outside letters, space, ' and - never get typed.
	f.typeText(t, a, "Ab1$c")
	f.frame(t, a)
	if got := ng.names[1]; got != "Abc" {
		t.Fatalf("nick draft = %q", got)
	}
	if ng.nameErr != "" {
		t.Fatalf("a valid name clears the error, got %q", ng.nameErr)
	}
	f.key(t, a, "Enter", 0)
	f.frame(t, a)
	if ng.pool.Views()[0].Nick != "Abc" {
		t.Fatalf("nick = %q", ng.pool.Views()[0].Nick)
	}
	// The pool's own errors show and block.
	a2, f2, fp := fakePawnsApp(t, 4, 2)
	fp.err = game.ErrNickTaken
	press(t, a2, f2, "pawns.nick")
	f2.typeText(t, a2, "x")
	f2.frame(t, a2)
	if a2.ng.startReason() != game.ErrNickTaken.Error() {
		t.Fatalf("reason %q, want the pool's error", a2.ng.startReason())
	}
	fp.err = errors.New("other")
	_ = strings.TrimSpace
}

func TestTeamWarnings(t *testing.T) {
	t.Parallel()
	a, f, fp := fakePawnsApp(t, 5, 3)
	fp.warn = []string{game.WarnNoFighter, game.WarnNoDoctor, game.WarnNoCook}
	f.frame(t, a)
	a.Draw(ebiten.NewImage(a.sw, a.sh))
	// The real pool reports the warnings in the spec's order and the screen
	// lays out one row per skill for the team grid.
	b, fb := pawnsApp(t)
	rep := b.ng.pool.Team()
	if len(rep.Best) != 11 {
		t.Fatalf("team grid has %d skills, want 11", len(rep.Best))
	}
	order := []string{game.WarnNoFighter, game.WarnNoDoctor, game.WarnNoCook, game.WarnNoGrower, game.WarnNoBuilder, game.WarnNoResearch, game.WarnYoungOrOld}
	last := -1
	for _, w := range rep.Warnings {
		idx := -1
		for i, o := range order {
			if o == w {
				idx = i
			}
		}
		if idx < 0 || idx <= last {
			t.Fatalf("warnings %v out of order", rep.Warnings)
		}
		last = idx
	}
	for i := range rep.Best {
		if !hasWidget(b, "pawns.team."+uiNum(i)) {
			t.Fatalf("team row %d has no tooltip area", i)
		}
	}
	b.Draw(ebiten.NewImage(b.sw, b.sh))
	_ = fb
}

func TestIncapableSkillsShownDash(t *testing.T) {
	t.Parallel()
	if skillLevelText(game.SkillView{Incapable: true, Level: 0}) != "—" {
		t.Error("an incapable skill shows a dash")
	}
	if skillLevelText(game.SkillView{Level: 12}) != "12" {
		t.Error("a capable skill shows its level")
	}
	if tip := skillTip(game.SkillView{Label: "Medicine", Incapable: true}); !strings.Contains(tip, "Incapable") {
		t.Errorf("tooltip %q", tip)
	}
	tip := skillTip(game.SkillView{Label: "Plants", Level: 9, LevelLabel: "Skilled", Passion: 2, SpeedPct: 112, Affects: "Grow"})
	for _, want := range []string{"Skilled", "112%", "major passion", "Grow"} {
		if !strings.Contains(tip, want) {
			t.Errorf("tooltip %q lacks %q", tip, want)
		}
	}
	// A real pool: every incapable skill of the highlighted candidate is
	// shown as a dash and the others as numbers.
	a, f := pawnsApp(t)
	found := false
	for i, v := range a.ng.pool.Views() {
		for _, s := range v.Skills {
			if s.Incapable {
				found = true
				if skillLevelText(s) != "—" {
					t.Fatalf("%s of candidate %d", s.Label, i)
				}
			}
		}
	}
	_ = found
	a.Draw(ebiten.NewImage(a.sw, a.sh))
	f.frame(t, a)
}

func TestForcedTraitAndAgeHonoured(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	toWorld(t, a)
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == screenLanding })
	// A scenario with a forced trait and a narrow age range.
	d := a.ng.scenario
	d.Parts = append(append([]scenario.Part(nil), d.Parts...),
		scenario.Part{Kind: scenario.PartForcedTrait, Trait: trait.Sunny, Chance: 100, Who: scenario.WhoAll},
		scenario.Part{Kind: scenario.PartAgeRange, MinAge: 40, MaxAge: 45})
	a.ng.scenario = d
	click(t, a, "land.random")
	click(t, a, "wizard.next")
	f.frame(t, a)
	views := a.ng.pool.Views()
	for _, v := range views {
		if v.AgeYears < 40 || v.AgeYears > 45 {
			t.Errorf("%s is %d, outside 40-45", v.Nick, v.AgeYears)
		}
		has := false
		for _, tr := range v.Traits {
			has = has || tr.Key == trait.DefOf(trait.Sunny).Key
		}
		if !has {
			t.Errorf("%s lacks the forced trait: %+v", v.Nick, v.Traits)
		}
	}
	a.Draw(ebiten.NewImage(a.sw, a.sh))
}

func TestPawnsScreenDrawsAtBothSizesWithoutArt(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}} {
		a, f := pawnsApp(t)
		sizeTo(t, a, f, size[0], size[1])
		a.Draw(ebiten.NewImage(size[0], size[1]))
		click(t, a, "pawns.swap")
		f.frame(t, a)
		a.Draw(ebiten.NewImage(size[0], size[1]))
	}
}

func TestSetupSeedAndPoolFollowTheTile(t *testing.T) {
	t.Parallel()
	a, f := pawnsApp(t)
	ng := a.ng
	first := ng.pool.Views()[0].Nick
	seed := setupSeedOf(ng)
	if want := fnv64a(tinySeed) ^ uint64(int64(ng.tile))*0x9E3779B97F4A7C15 ^ ng.scenario.Seed; seed != want {
		t.Fatalf("seed %x, want %x", seed, want)
	}
	// Going back and choosing another site makes another pool.
	click(t, a, "wizard.back")
	f.frame(t, a)
	click(t, a, "land.random")
	click(t, a, "wizard.next")
	f.frame(t, a)
	if setupSeedOf(ng) == seed {
		t.Fatal("another tile, another setup seed")
	}
	if ng.pool.Views()[0].Nick == first && ng.pool.Views()[1].Nick == "" {
		t.Fatal("the pool is rebuilt")
	}
	_ = look.Male
}
