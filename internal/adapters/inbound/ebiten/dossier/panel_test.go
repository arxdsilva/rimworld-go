package dossier

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
)

var card = gfx.Rect{X: 20, Y: 40, W: 300, H: 360}

func TestClaimsNeedsTheCardOnAndAPawnOrAnimal(t *testing.T) {
	s := newShell(t)
	if !s.panel.Claims(selection.Pawn, 7) {
		t.Fatal("an enabled card claims a known pawn")
	}
	if s.panel.Claims(selection.Building, 7) || s.panel.Claims(selection.Pawn, 0) || s.panel.Claims(selection.Pawn, 99) {
		t.Fatal("it claims neither a building, no id nor a stale id")
	}
	s.set.HUD.Pane.Restyle = false
	if s.panel.Claims(selection.Pawn, 7) {
		t.Fatal("without the restyle the pane keeps its body")
	}
	s.set.HUD.Pane.Restyle, s.set.HUD.Enabled = true, false
	if s.panel.Claims(selection.Pawn, 7) {
		t.Fatal("a disabled card claims nothing")
	}
}

// RHUD-040, 041, 503, 504: the facts are read at the refresh rate, at once for
// a new selection, and dropped by Reset.
func TestRefreshGateAndSelectionKey(t *testing.T) {
	s := newShell(t)
	s.set.HUD.RefreshTenths = 5
	s.step(0, 0, false, card, true)
	first := s.reads
	if first == 0 {
		t.Fatal("the first Update reads the pawn")
	}
	for i := 0; i < 29; i++ { // five tenths is 30 Updates
		s.step(0, 0, false, card, true)
	}
	if s.reads != first {
		t.Fatalf("no read inside the gate: %d -> %d", first, s.reads)
	}
	s.pawn.Name = "Bo2"
	s.step(0, 0, false, card, true)
	s.step(0, 0, false, card, true)
	if s.reads == first {
		t.Fatal("the gate opens after five tenths")
	}
	n := s.reads
	s.pawn.ID = 8
	s.panel.Select(selection.Pawn, 8)
	s.step(0, 0, false, card, true)
	if s.reads == n || s.panel.d.p.ID != 8 {
		t.Fatal("a new selection reads at once")
	}
	s.panel.Select(selection.Pawn, 7) // 7 is gone: a stale id
	s.step(0, 0, false, card, true)
	if s.panel.have || s.find(true, "standing") != nil && s.panel.d.p.ID == 8 {
		t.Fatal("a stale id is dropped")
	}
	s.panel.Reset()
	if s.panel.have || s.panel.kind != selection.None || s.panel.lay[0].ok {
		t.Fatal("Reset (a map switch, the menu) drops the cache")
	}
}

func TestSettingsChangeRebuildsAtOnce(t *testing.T) {
	s := newShell(t)
	s.set.HUD.RefreshTenths = 30
	s.step(0, 0, false, card, true)
	h0 := s.find(true, "standing").h
	s.set.HUD.Text.BodySize, s.set.HUD.Text.BodyLinePct = 16, 150
	s.step(0, 0, false, card, true)
	if h1 := s.find(true, "standing").h; h1 <= h0 {
		t.Fatalf("text size and line height change the row: %v -> %v", h0, h1)
	}
}

// RHUD-038: upper right, docked and floating. Docked, only the two lines of
// who and how lie under it (text aligned left, the gear shows while the mouse is
// over the card), never a bar, a skill or a value aligned right.
func TestGearCorners(t *testing.T) {
	g := gearRect(card)
	if g.Y < card.Y || g.Y > card.Y+card.H/2 || g.Right() > card.Right() || g.X < card.X+card.W/2 {
		t.Fatalf("the gear is upper right: %+v", g)
	}
	s := newShell(t)
	s.step(0, 0, false, card, true)
	for _, it := range s.panel.lay[0].items {
		r := gfx.Rect{X: card.X + it.x, Y: card.Y + it.y, W: it.w, H: it.h}
		if !rectsMeet(r, g) {
			continue
		}
		switch it.key {
		case "standing", "identity", "body_state", "mind_state":
		default:
			t.Errorf("the docked gear covers %s", it.key)
		}
		if it.kind != kText {
			t.Errorf("the docked gear covers a %v item, %s", it.kind, it.key)
		}
	}
}

func rectsMeet(a, b gfx.Rect) bool {
	return a.X < b.Right() && b.X < a.Right() && a.Y < b.Bottom() && b.Y < a.Bottom()
}

// RHUD-036, 037: a press on the gear opens setup and nothing under it fires;
// the gear exists only while the mouse is over the card.
func TestGearOpensSetupAndShieldsWidgets(t *testing.T) {
	s := newShell(t)
	s.setTree(false, column(widget("title")))
	g := gearRect(card)
	gx, gy := int(g.X+g.W/2), int(g.Y+g.H/2)
	s.step(gx, gy, false, card, false)
	if _, ok := s.kit.Find(ids.DossierGear); !ok {
		t.Fatal("the gear is declared while the mouse is over the card")
	}
	if title := s.find(false, "title"); title == nil || !title.hit(card, false, gx, gy) {
		t.Fatal("setup: the gear lies over the name row")
	}
	s.step(gx, gy, true, card, false)
	if s.setups != 1 || len(s.tabs) != 0 {
		t.Fatalf("the press opens setup (%d) and the name under it stays quiet (%v)", s.setups, s.tabs)
	}
	s.step(900, 600, false, card, false)
	if _, ok := s.kit.Find(ids.DossierGear); ok {
		t.Fatal("no gear while the mouse is elsewhere")
	}
}

func (it *item) hit(r gfx.Rect, docked bool, x, y int) bool {
	b := body(r, docked)
	return gfx.Rect{X: b.X + it.x, Y: b.Y + it.y, W: it.w, H: it.h}.Hit(x, y)
}

// RHUD-079, 127, 139, 156, 170, 178, 183, 186, 193, 224, 285, 300, 304, 306,
// 316: each value opens the tab it belongs to.
func TestClicksOpenTheirTabs(t *testing.T) {
	cases := []struct {
		key, def string
		tab, win string
	}{
		{"title", "", "social", ""},
		{"standing", "", "bio", ""},
		{"identity", "", "bio", ""},
		{"vigour_bar", "", "health", ""},
		{"mood_bar", "", "needs", ""},
		{"hunger_bar", "", "needs", ""},
		{"rest_bar", "", "needs", ""},
		{"fun_bar", "", "needs", ""},
		{"skill", "shooting", "bio", ""},
		{"doing", "", "", ids.TabWork},
	}
	for _, c := range cases {
		s := newShell(t)
		s.pawn.Job = "hauling"
		s.setTree(true, column(config.HUDNode{Kind: c.key, Def: c.def}))
		s.step(0, 0, false, card, true)
		it := s.find(true, c.key)
		if it == nil {
			t.Fatalf("%s: no item", c.key)
		}
		x, y := s.at(it, card, true)
		s.step(x, y, true, card, true)
		if c.tab != "" && (len(s.tabs) != 1 || s.tabs[0] != c.tab) {
			t.Errorf("%s: tabs %v, want %s", c.key, s.tabs, c.tab)
		}
		if c.win != "" && (len(s.wins) != 1 || s.wins[0] != c.win) {
			t.Errorf("%s: windows %v, want %s", c.key, s.wins, c.win)
		}
	}
}

// RHUD-176, 432, 470: the mood bar has a tick at each break level, and a trait
// that moves the levels moves the ticks; other bars have none.
func TestMoodTicksFollowBreakLevels(t *testing.T) {
	s := newShell(t)
	s.setTree(true, column(widget("mood_bar"), widget("hunger_bar")))
	s.step(0, 0, false, card, true)
	m := s.find(true, "mood_bar")
	if m.nticks != 3 || m.ticks != [3]float32{0.2, 0.1, 0.05} {
		t.Fatalf("ticks %v/%d", m.ticks, m.nticks)
	}
	if s.find(true, "hunger_bar").nticks != 0 {
		t.Fatal("only the mood bar has thresholds")
	}
	s.card.BreakAt.Minor = 35
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	if got := s.find(true, "mood_bar").ticks[0]; got != 0.35 {
		t.Fatalf("a trait shift moves the tick: %v", got)
	}
}

// RHUD-171, 469: the fill colour follows the style and the fill.
func TestBarStylesColourTheFill(t *testing.T) {
	for _, st := range sheet.BarStyles() {
		s := newShell(t)
		n := config.HUDNode{Kind: "vigour_bar", BarStyle: st}
		s.setTree(true, column(n))
		s.step(0, 0, false, card, true)
		want := sheet.BarColor(st, 0.8, s.panel.pal["bar_low"], s.panel.pal["bar_main"])
		if got := s.find(true, "vigour_bar").fill; got != want {
			t.Errorf("%s: fill %v want %v", st, got, want)
		}
	}
}

// The player's colours win over the theme, key by key.
func TestColourOverrides(t *testing.T) {
	s := newShell(t)
	s.set.HUD.Colors = map[string]string{"bar_main": "#FF0000FF", "tone_good": "junk"}
	s.setTree(true, column(config.HUDNode{Kind: "vigour_bar", BarStyle: sheet.BarMainOnly}))
	s.step(0, 0, false, card, true)
	if s.find(true, "vigour_bar").fill != s.panel.pal["bar_main"] || s.panel.pal["bar_main"].R != 0xFF {
		t.Fatal("the override colours the bar")
	}
	if s.panel.pal["tone_good"].A == 0 {
		t.Fatal("a malformed colour keeps the theme's")
	}
}

// RHUD-074: an unknown element is a marker box and one hint.
func TestUnknownElementIsAMarker(t *testing.T) {
	s := newShell(t)
	s.setTree(true, column(widget("flux_gauge")))
	s.step(0, 0, false, card, true)
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	it := s.find(true, "flux_gauge")
	if it == nil || it.kind != kMarker || it.text != "flux_gauge" {
		t.Fatalf("marker %+v", it)
	}
	if len(s.hints) != 1 || !strings.Contains(s.hints[0], "flux_gauge") {
		t.Fatalf("one hint per key: %v", s.hints)
	}
}

// RHUD-083, 084, 085: the food selector lists the policies and ends with a
// manage entry; it is hidden where food cannot be set.
func TestFoodSelector(t *testing.T) {
	s := newShell(t)
	s.setTree(true, column(widget("diet_pick")))
	s.step(0, 0, false, card, true)
	it := s.find(true, "diet_pick")
	if it == nil || !strings.HasPrefix(it.text, "Food: Plain") {
		t.Fatalf("selector %+v", it)
	}
	x, y := s.at(it, card, true)
	s.step(x, y, true, card, true)
	if !s.kit.ModalOpen() {
		t.Fatal("a click opens the menu")
	}
	pols := s.policies
	menus{s.panel}.dietChoice(pols, 1)
	if len(s.cmds) != 1 || s.cmds[0].Kind != command.SetFoodPolicy || s.cmds[0].Index != 4 || s.cmds[0].Pawn != 7 {
		t.Fatalf("choosing Fancy: %+v", s.cmds)
	}
	menus{s.panel}.dietChoice(pols, len(pols))
	if len(s.wins) != 1 || s.wins[0] != ids.TabAssign {
		t.Fatalf("the last entry manages: %v", s.wins)
	}
	s.pawn.Visitor = true
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	if s.find(true, "diet_pick") != nil {
		t.Fatal("a visitor's food cannot be set")
	}
}

// RHUD-089, 092, 093: the area selector opens the game's area list (its hover
// preview and Manage row live there) and assigns, and its back takes the area's
// colour.
func TestAreaSelector(t *testing.T) {
	s := newShell(t)
	s.area = 5
	s.setTree(true, column(widget("area_pick")))
	s.step(0, 0, false, card, true)
	it := s.find(true, "area_pick")
	if it == nil || !strings.HasPrefix(it.text, "Area: Yard") {
		t.Fatalf("selector %+v", it)
	}
	want := sheet.Lerp(s.panel.pal["picker_back"], rgbaOf(10, 200, 30), 0.5)
	if it.back != want {
		t.Fatalf("back %v want %v", it.back, want)
	}
	menus{s.panel}.pickArea()
	if s.pick == nil || s.pickerCur != 5 || !s.pickerBase || s.pickerTitle != "Allowed area" {
		t.Fatalf("area list opened with cur %d base %v title %q", s.pickerCur, s.pickerBase, s.pickerTitle)
	}
	s.pick(5)
	if s.cmds[0].Kind != command.AssignArea || s.cmds[0].Index != 5 || s.cmds[0].Pawn != 7 {
		t.Fatalf("assign %+v", s.cmds)
	}
	s.area = 0
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	if got := s.find(true, "area_pick").text; !strings.HasPrefix(got, "Area: Unrestricted") {
		t.Fatalf("unrestricted: %q", got)
	}
}

// RHUD-097, 098, 101: the hour selector names this hour and sets it.
func TestHourSelector(t *testing.T) {
	s := newShell(t)
	s.setTree(true, column(widget("hour_pick")))
	s.step(0, 0, false, card, true)
	it := s.find(true, "hour_pick")
	if it == nil || !strings.HasPrefix(it.text, "Hour 14:") {
		t.Fatalf("selector %+v", it)
	}
	menus{s.panel}.hourChoice(14, 1)
	c := s.cmds[0]
	if c.Kind != command.SetSchedule || c.Index != 14 || c.Mode != 1 || c.Pawn != 7 {
		t.Fatalf("schedule %+v", c)
	}
	menus{s.panel}.hourChoice(14, hourActivities) // RHUD-100: Manage...
	if len(s.cmds) != 1 || len(s.wins) != 1 || s.wins[0] != ids.TabSchedule {
		t.Fatalf("Manage sent %+v and opened %v, want only the Schedule window", s.cmds, s.wins)
	}
}

// RHUD-212..215: a skill's colour follows its state.
func TestSkillColourStates(t *testing.T) {
	s := newShell(t)
	s.setTree(true, column(generic("skill", "shooting")))
	s.step(0, 0, false, card, true)
	love := s.find(true, "skill")
	if love.st.Color != s.panel.pal["skill_love"] || love.passIc != nil && s.panel.ic == nil {
		t.Fatalf("major passion reads skill_love: %v", love.st.Color)
	}
	s.card.Saturated[0] = true
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	if got := s.find(true, "skill").st.Color; got != s.panel.pal["skill_full"] {
		t.Fatalf("saturated reads skill_full: %v", got)
	}
	s.card.Saturated[0], s.card.ActiveSkill = false, 0
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	if got := s.find(true, "skill").st.Color; got != s.panel.pal["skill_busy"] {
		t.Fatalf("in use reads skill_busy: %v", got)
	}
}

// RHUD-451, 461, 462: the tooltip is the card's own, each line in its tone,
// after the cursor has rested.
func TestTooltipShowsAfterARest(t *testing.T) {
	s := newShell(t)
	s.setTree(true, column(widget("vigour_bar")))
	s.step(0, 0, false, card, true)
	it := s.find(true, "vigour_bar")
	x, y := s.at(it, card, true)
	for i := 0; i < tipDelay-2; i++ {
		s.step(x, y, false, card, true)
	}
	if s.panel.tip.on {
		t.Fatal("not yet")
	}
	for i := 0; i < 4; i++ {
		s.step(x, y, false, card, true)
	}
	if !s.panel.tip.on || len(s.panel.tip.rows) == 0 || s.panel.tip.box.W > tipMaxW {
		t.Fatalf("tip %+v", s.panel.tip)
	}
	s.step(0, 0, false, card, true)
	if s.panel.tip.on {
		t.Fatal("it goes with the cursor")
	}
}

// RHUD-012: the stat sheet lists every stat the card has, read only.
func TestStatSheetModal(t *testing.T) {
	s := newShell(t)
	s.step(0, 0, false, card, true)
	for i := range s.card.Stats {
		s.card.Stats[i] = -1
	}
	s.card.Stats[0], s.card.Stats[1] = 1000, 1100
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	s.panel.OpenStats()
	if !s.kit.ModalOpen() || s.kit.Top() != "dossier.stats" {
		t.Fatal("the sheet opens as a modal")
	}
	if rows := (statSheet{s.panel}).rows(); len(rows) != 2 || rows[0].value != "100%" {
		t.Fatalf("rows %+v", rows)
	}
}

// RHUD-034: the floating card has a background and a margin.
func TestFloatingCardHasAMargin(t *testing.T) {
	s := newShell(t)
	s.step(0, 0, false, card, false)
	l := &s.panel.lay[1]
	if !l.ok || l.w != card.W-2*cardPad || l.h != card.H-2*cardPad {
		t.Fatalf("floating body %v x %v", l.w, l.h)
	}
	s.paint(card, false)
	if h := s.panel.Height(false, 300); h <= 2*cardPad {
		t.Fatalf("height %v", h)
	}
}

// RHUD-317..320: animals are the player's creature or wild; a wild one has no
// selectors.
func TestAnimalTargets(t *testing.T) {
	s := newShell(t)
	s.animal.ID, s.animal.Tame, s.animal.Name, s.animal.FoodPolicy = 30, true, "Rex", "Plain"
	s.panel.Select(selection.Animal, 30)
	s.setTree(true, column(widget("title"), widget("diet_pick"), widget("area_pick"), widget("hour_pick")))
	s.step(0, 0, false, card, true)
	if s.find(true, "diet_pick") == nil || s.find(true, "area_pick") == nil || s.find(true, "hour_pick") != nil {
		t.Fatal("a tame animal gets food and area, not the hour")
	}
	s.animal.Tame = false
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	if s.find(true, "diet_pick") != nil || s.find(true, "area_pick") != nil {
		t.Fatal("a wild animal has no selectors")
	}
}

func TestDefaultTreesDrawForEveryTarget(t *testing.T) {
	for _, docked := range []bool{true, false} {
		s := newShell(t)
		s.step(0, 0, false, card, docked)
		if l := s.panel.lay[modeIdx(docked)]; !l.ok || len(l.items) == 0 {
			t.Fatalf("docked=%v: default layout is empty", docked)
		}
		s.paint(card, docked)
	}
}

// TestTimingOnlyWhenAsked: nobody asking means no clock and no numbers; asking
// fills the numbers from the next draw on, and another selection drops them.
func TestTimingOnlyWhenAsked(t *testing.T) {
	s := newShell(t)
	card := gfx.Rect{X: 0, Y: 0, W: 400, H: 460}
	img := ebiten.NewImage(404, 464)
	s.step(0, 0, false, card, true)
	s.panel.Draw(img, card)
	if s.panel.timer.seen || len(s.panel.timer.widgetMS) != 0 {
		t.Fatal("the card measured itself with nobody asking")
	}
	if got := s.panel.TimingLines(); got != nil {
		t.Fatalf("lines before a measured draw: %v", got)
	}
	s.step(0, 0, false, card, true)
	s.panel.Draw(img, card)
	lines := s.panel.TimingLines()
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "card: draw ") {
		t.Fatalf("timing lines = %v", lines)
	}
	if len(lines) > timingShown+1 {
		t.Fatalf("%d lines, the list holds %d", len(lines), timingShown+1)
	}
	if !s.panel.timer.asked {
		t.Fatal("reading the lines should ask for the next frame")
	}
	s.panel.Draw(img, card)
	if s.panel.timer.asked {
		t.Fatal("a draw should use up the request, so a closed overlay stops the clock")
	}
	s.panel.Select(selection.Animal, 99)
	if s.panel.timer.seen {
		t.Fatal("another selection keeps the old numbers")
	}
}

// RHUD-057: a stored layout the card can't use as it stands says so, once.
func TestBadStoredLayoutWarnsOnce(t *testing.T) {
	s := newShell(t)
	bad := column(widget("mood_bar"))
	bad.Kids[0].Kids = []config.HUDNode{widget("food_bar")} // a widget cannot hold anything
	s.setTree(true, bad)
	s.step(0, 0, false, card, true)
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	n := 0
	for _, h := range s.hints {
		if strings.Contains(h, "saved card layout has a problem") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("hints %q: want the problem named once", s.hints)
	}
}

func TestDroppedLayoutWarns(t *testing.T) {
	s := newShell(t)
	s.set.HUD.Layouts.Dropped = true
	s.panel.Invalidate()
	s.step(0, 0, false, card, true)
	if len(s.hints) != 1 || !strings.Contains(s.hints[0], "too big") {
		t.Fatalf("hints %q", s.hints)
	}
}

// The docked card's targets: a colonist, a prisoner, a tame and a wild animal.
func (s *fakeShell) target(w sheet.Who) {
	s.animal = game.AnimalView{}
	s.pawn.Prisoner = false
	switch w {
	case sheet.WhoColonist, sheet.WhoStranger:
		s.pawn.Prisoner = w == sheet.WhoStranger
		s.panel.Select(selection.Pawn, 7)
	default:
		s.animal = game.AnimalView{ID: 30, Name: "Rex", Tame: w == sheet.WhoPrisoner, FoodPolicy: "Plain", Health: 90}
		s.panel.Select(selection.Animal, 30)
	}
	s.panel.Invalidate()
}

// keys is what the docked layout placed inside a body h tall.
func (s *fakeShell) keys(h float32) []string {
	var out []string
	for _, it := range s.panel.lay[0].items {
		if it.y+it.h <= h+0.5 {
			out = append(out, it.key+"@"+strconv.Itoa(int(it.y)))
		}
	}
	return out
}

// The compact docked card shows every row for every target at 1280x720 and UI
// scales 1, 1.5 and 2 (1280x720, 853x480 and 640x360 logical): what a very
// tall pane would show, all inside the pane the card gets, with no warning.
func TestDockedCardDropsNothing(t *testing.T) {
	for _, scr := range [][2]int{{1280, 720}, {853, 480}, {640, 360}} {
		for w := sheet.WhoColonist; w <= sheet.WhoWild; w++ {
			s := newShell(t)
			s.sw, s.sh = scr[0], scr[1]
			s.target(w)
			d := sheet.DockedFor(s.set.HUD, s.sh)
			body := gfx.Rect{X: 12, Y: 40, W: float32(d.Pane.TabW*d.Pane.MinTabs - 24), H: float32(d.Pane.Height - sheet.DockMargin)}
			tall := body
			tall.H = 2000
			s.step(0, 0, false, tall, true)
			want := s.keys(tall.H)
			s.step(0, 0, false, body, true)
			got := s.keys(body.H)
			if len(want) == 0 || !slices.Equal(got, want) {
				t.Errorf("%v target %c: the pane shows\n%v\nnot\n%v", scr, w.Letter(), got, want)
			}
			if l := s.panel.lay[0]; l.height > body.H {
				t.Errorf("%v target %c: %v tall in %v", scr, w.Letter(), l.height, body.H)
			}
			if len(s.hints) != 0 {
				t.Errorf("%v target %c: %v", scr, w.Letter(), s.hints)
			}
		}
	}
}

// A saved layout taller than the card is cut at the card's bottom, and the card
// says so once.
func TestTooTallLayoutWarnsOnce(t *testing.T) {
	s := newShell(t)
	rows := make([]config.HUDNode, 20)
	for i := range rows {
		rows[i] = widget("standing")
	}
	s.setTree(true, column(rows...))
	r := gfx.Rect{X: 0, Y: 0, W: 500, H: 134}
	for range 3 {
		s.step(0, 0, false, r, true)
		s.panel.Invalidate()
	}
	if len(s.hints) != 1 || !strings.Contains(s.hints[0], "doesn't fit") || !strings.Contains(s.hints[0], "floating card") {
		t.Fatalf("hints %q", s.hints)
	}
	if got := len(s.keys(r.H)); got == 0 || got >= 20 {
		t.Fatalf("%d of 20 rows in the pane", got)
	}
}

// Docked, the card's sizes follow the screen's height; floating, they are the
// player's.
func TestDockedSizesFollowTheScreen(t *testing.T) {
	s := newShell(t)
	s.sh = 1080
	s.step(0, 0, false, card, true)
	if got := s.panel.builder(s.panel.hud()).body.Size; got != 18 {
		t.Fatalf("docked text at 1080: %v, want 18", got)
	}
	s.set.HUD.Docked = false
	if got := s.panel.hud().Text.BodySize; got != s.set.HUD.Text.BodySize {
		t.Fatalf("floating text %d, want the setting %d", got, s.set.HUD.Text.BodySize)
	}
}
