package ebiten

import (
	"fmt"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
)

// globalSim is a stub colony with a canned world and a temperature that
// depends on the cell, so a test can tell which cell the HUD asked about.
type globalSim struct {
	*stubSim
	world game.WorldView
	temp  func(x, y int) int
}

func (s *globalSim) World() game.WorldView { return s.world }
func (s *globalSim) TemperatureAt(x, y int) int {
	if s.temp != nil {
		return s.temp(x, y)
	}
	return s.stubSim.TemperatureAt(x, y)
}

// globalWorld is a 16x16 world with one room (id 0) at x 8..11, y 8..11:
// outdoors is 9 C, every cell reads 10 x its column + 100 tenths.
func globalWorld() game.WorldView {
	const n = 16
	grid := make([]int, n*n)
	for i := range grid {
		grid[i] = -1
	}
	for y := 8; y < 12; y++ {
		for x := 8; x < 12; x++ {
			grid[y*n+x] = 0
		}
	}
	return game.WorldView{TerrainW: n, Terrain: make([]uint8, n*n), RoomGrid: grid,
		Rooms: []game.RoomView{{ID: 0, Role: "bedroom", Temp: 215, Cells: 16}}, OutdoorTemp: 90, Season: "Spring", Weather: "clear"}
}

func newGlobalSim() *globalSim {
	return &globalSim{stubSim: &stubSim{w: 16, h: 16}, world: globalWorld(), temp: func(x, y int) int { return 100 + 10*x }}
}

// globalScales are the UI scales the HUD is checked at, each on a window
// that keeps the logical screen at 1280x720.
var globalScales = []struct {
	ui     int
	ww, wh float64
}{{100, 1280, 720}, {150, 1920, 1080}, {200, 2560, 1440}}

// globalColony is a live colony on a ww x wh window at UI scale ui
// (percent), paused, with the cursor parked off the map.
func globalColony(t *testing.T, sim Simulation, ui int, ww, wh float64) (*App, *fakeInput) {
	t.Helper()
	t.Cleanup(func() { curUIScale = unitUIScale })
	a := testApp(sim)
	a.devFn = func() float64 { return 1 } // not the test machine's Retina factor
	st := a.settings
	st.Accessibility.UIScaleAuto, st.Accessibility.UIScale = false, ui
	a.setSettings(st)
	a.LayoutF(ww, wh)
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != screenColony {
		t.Fatalf("screen = %s, want colony", a.nav.screen())
	}
	f := fakeOn(a)
	f.cur.mx, f.cur.my = -50, -50
	f.frame(t, a)
	a.renderer.speed = speedPaused
	return a, f
}

// onRect puts the fake cursor on the centre of r (logical pixels).
func (f *fakeInput) onRect(a *App, r rect) {
	cx, cy := r.center()
	f.cur.mx, f.cur.my = int(float64(cx)*a.scale.s), int(float64(cy)*a.scale.s)
}

var globalToggleIDs = []string{"hud.roof", "hud.home", "hud.homeauto", "hud.plan", "hud.rooms", "hud.learn", "hud.bar", "hud.rescat"}

// All eight overlay toggles are 24 px squares inside Global, in one
// right-aligned row of eight, at every UI scale, and none shares a rect
// (hud.plan and hud.rooms used to).
func TestGlobalTogglesInsideGlobal(t *testing.T) {
	for _, sc := range globalScales {
		t.Run(fmt.Sprintf("ui%d", sc.ui), func(t *testing.T) {
			a, _ := globalColony(t, newGlobalSim(), sc.ui, sc.ww, sc.wh)
			g := a.hud().Global
			if g.empty() {
				t.Fatal("Global has no rect")
			}
			var rs []rect
			for _, id := range globalToggleIDs {
				r, ok := a.globalToggleRect(id)
				if !ok {
					t.Fatalf("toggle %s missing", id)
				}
				if r.W != 24 || r.H != 24 {
					t.Errorf("%s is %vx%v, want 24x24", id, r.W, r.H)
				}
				if r.X < g.X || r.right() > g.right() || r.Y < g.Y || r.bottom() > g.bottom() {
					t.Errorf("%s %+v is outside Global %+v", id, r, g)
				}
				for j, o := range rs {
					if r.intersect(o).W > 0 && r.intersect(o).H > 0 {
						t.Errorf("%s %+v overlaps %s %+v", id, r, globalToggleIDs[j], o)
					}
				}
				rs = append(rs, r)
			}
			for i := 1; i < len(rs); i++ {
				if rs[i].Y != rs[0].Y || rs[i].X <= rs[i-1].X {
					t.Errorf("toggles are not one left-to-right row: %+v", rs)
					break
				}
			}
			if last := rs[len(rs)-1]; g.right()-last.right() > 8 {
				t.Errorf("the row is not right-aligned: last toggle ends %v short of Global's right edge", g.right()-last.right())
			}
			// the five legacy toggles are buttons, the three new ones widgets;
			// both are hit-testable by id
			hits := a.layoutHits(a.sw, a.sh)
			for _, id := range globalToggleIDs {
				if _, ok := hasButton(hits, id); !ok {
					t.Errorf("no hit target for %s", id)
				}
			}
		})
	}
}

// overlayFlags is every flag a toggle can change.
type overlayFlags struct{ roof, home, plan, rooms, learn, bar, rescat bool }

func (a *App) overlayFlags() overlayFlags {
	r, g := a.renderer, a.settings.Gameplay
	return overlayFlags{r.roofOverlay, r.homeOverlay, !r.planHidden, a.rooms, g.ShowLearningHelper, g.ShowColonistBar, g.CategorizedResources}
}

// Each toggle's own rect toggles its own overlay and nothing else; the
// real pointer path is used (press and release at the rect's centre).
func TestGlobalTogglesClickTheirOwnOverlay(t *testing.T) {
	for _, sc := range globalScales {
		t.Run(fmt.Sprintf("ui%d", sc.ui), func(t *testing.T) {
			s := newGlobalSim()
			a, f := globalColony(t, s, sc.ui, sc.ww, sc.wh)
			flip := map[string]func(*overlayFlags){
				"hud.roof":     func(o *overlayFlags) { o.roof = !o.roof },
				"hud.home":     func(o *overlayFlags) { o.home = !o.home },
				"hud.homeauto": func(o *overlayFlags) {},
				"hud.plan":     func(o *overlayFlags) { o.plan = !o.plan },
				"hud.rooms":    func(o *overlayFlags) { o.rooms = !o.rooms },
				"hud.learn":    func(o *overlayFlags) { o.learn = !o.learn },
				"hud.bar":      func(o *overlayFlags) { o.bar = !o.bar },
				"hud.rescat":   func(o *overlayFlags) { o.rescat = !o.rescat },
			}
			for _, id := range globalToggleIDs {
				rc, _ := a.globalToggleRect(id)
				want := a.overlayFlags()
				flip[id](&want)
				f.onRect(a, rc)
				f.click(t, a, mbLeft, 0)
				if got := a.overlayFlags(); got != want {
					t.Errorf("%s: click at %+v left %+v, want only its own flag flipped: %+v", id, rc, got, want)
				}
			}
			if n := len(s.cmds); n != 1 || s.cmds[0].Kind != command.SetHomeAuto || !s.cmds[0].Enabled {
				t.Errorf("cmds = %+v, want one SetHomeAuto (on) from hud.homeauto", s.cmds)
			}
		})
	}
}

// The three settings toggles save through the existing settings path.
func TestGlobalSettingsTogglesSave(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	var saved []config.Settings
	a.cfg.Settings.Save = func(s config.Settings) error { saved = append(saved, s); return nil }
	for _, id := range []string{"hud.learn", "hud.bar", "hud.rescat"} {
		click(t, a, id)
	}
	if len(saved) != 3 {
		t.Fatalf("saved %d times, want 3", len(saved))
	}
	g := saved[2].Gameplay
	if g.ShowLearningHelper || g.ShowColonistBar || !g.CategorizedResources {
		t.Fatalf("saved gameplay = %+v, want helper off, bar off, categorised on", g)
	}
	click(t, a, "hud.learn")
	if !a.settings.Gameplay.ShowLearningHelper {
		t.Fatal("a second click turns the helper back on")
	}
}

// globalColonyP is globalColony for parallel tests at the default scale.
func globalColonyP(t *testing.T, sim Simulation) (*App, *fakeInput) {
	t.Helper()
	a := testApp(sim)
	a.devFn = func() float64 { return 1 }
	a.finishBoot()
	a.startGame()
	if a.nav.screen() != screenColony {
		t.Fatalf("screen = %s, want colony", a.nav.screen())
	}
	f := fakeOn(a)
	f.cur.mx, f.cur.my = -50, -50
	f.frame(t, a)
	a.renderer.speed = speedPaused
	return a, f
}

// Toggle tooltips carry the live key; the three settings toggles have
// kit tooltips.
func TestGlobalToggleTooltipsShowKeys(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	bs := a.colonyButtons(a.sw, a.sh)
	for id, key := range map[string]string{"hud.roof": "[Shift+R]", "hud.rooms": "[G]"} {
		b, ok := hasButton(bs, id)
		if !ok || !strings.Contains(b.tip, key) {
			t.Errorf("%s tip = %q, want it to show %s", id, b.tip, key)
		}
	}
	for _, id := range []string{"hud.home", "hud.homeauto", "hud.plan"} {
		if b, ok := hasButton(bs, id); !ok || b.tip == "" {
			t.Errorf("%s has no tooltip", id)
		}
	}
	for _, id := range []string{"hud.learn", "hud.bar", "hud.rescat"} {
		if tip := tipOf(t, a, id); tip == "" {
			t.Errorf("%s has no tooltip", id)
		}
	}
}

// Four speed buttons: pause, normal, fast, superfast. A click sets the
// speed (through the renderer's own intents), the active one is
// highlighted, and tooltips show the keys.
func TestGlobalSpeedButtons(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	r := a.renderer
	find := func(i int) button {
		b, ok := hasButton(a.colonyButtons(a.sw, a.sh), fmt.Sprintf("speed.%d", i))
		if !ok {
			t.Fatalf("speed.%d missing", i)
		}
		return b
	}
	for i := 4; i < 6; i++ {
		if _, ok := hasButton(a.colonyButtons(a.sw, a.sh), fmt.Sprintf("speed.%d", i)); ok {
			t.Errorf("speed.%d exists: ultrafast is developer-only and not a button", i)
		}
	}
	g := a.hud().Global
	for i, want := range []struct {
		speed speed
		key   string
	}{{speedPaused, "[Space]"}, {speedNormal, "[1]"}, {speedFast, "[2]"}, {speedVeryFast, "[3]"}} {
		b := find(i)
		if b.w != 52 || b.h != 24 || b.x < g.X || b.x+b.w > g.right() || b.y < g.Y || b.y+b.h > g.bottom() {
			t.Errorf("speed.%d = %+v, want 52x24 inside Global %+v", i, b, g)
		}
		if !strings.Contains(b.tip, want.key) {
			t.Errorf("speed.%d tip = %q, want it to show %s", i, b.tip, want.key)
		}
		click(t, a, b.id)
		if i == 0 {
			// pause toggles: it was paused, so the click resumes at normal
			if r.speed != speedNormal {
				t.Errorf("clicking pause while paused: speed %v, want normal", r.speed)
			}
			click(t, a, b.id)
		}
		if r.speed != want.speed {
			t.Errorf("speed.%d: speed %v, want %v", i, r.speed, want.speed)
		}
		for j := 0; j < 4; j++ {
			if got := find(j).selected; got != (j == i) {
				t.Errorf("after speed.%d: speed.%d selected = %v", i, j, got)
			}
		}
	}
	r.speed = speedUltra // developer ultrafast: no button lights up
	for j := 0; j < 4; j++ {
		if find(j).selected {
			t.Errorf("ultrafast lit speed.%d", j)
		}
	}
}

// The background pause and the threat pause keep saying why: a reason
// line above the speed buttons (and the time tooltip carries the label).
func TestGlobalSpeedReasonShows(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	if got := a.speedReason(); got != "" {
		t.Fatalf("reason %q with the game paused by hand", got)
	}
	h0 := a.globalHeight()
	a.bg.paused = true
	if got := a.speedReason(); got != bgPausedLabel {
		t.Fatalf("reason %q, want %q", got, bgPausedLabel)
	}
	if h1 := a.globalHeight(); h1 <= h0 {
		t.Fatalf("a reason line should make Global taller: %v -> %v", h0, h1)
	}
	a.bg.paused = false
	a.renderer.speed, a.renderer.threatPaused = speedPaused, true
	if got := a.speedReason(); !strings.Contains(strings.ToLower(got), "paused (threat)") {
		t.Fatalf("reason %q, want the threat pause", got)
	}
	a.renderer.speed = speedFast // resumed: the stale flag does not show
	if got := a.speedReason(); got != "" {
		t.Fatalf("reason %q after resuming", got)
	}
}

// The temperature line follows the cell under the cursor: indoors inside a
// room, outdoors elsewhere, and the outdoor temperature over the HUD.
func TestGlobalTemperatureFollowsHoveredCell(t *testing.T) {
	t.Parallel()
	a, f := globalColonyP(t, newGlobalSim())
	hover := func(tx, ty int) string {
		f.moveTo(a, tx, ty)
		f.frame(t, a)
		return a.temperatureLine()
	}
	if got, want := hover(3, 3), "Outdoors 13 °C"; got != want { // TemperatureAt(3,3) = 130
		t.Errorf("outdoor cell: %q, want %q", got, want)
	}
	if got, want := hover(9, 9), "Indoors 19 °C"; got != want { // TemperatureAt(9,9) = 190 inside the room
		t.Errorf("room cell: %q, want %q", got, want)
	}
	if got, want := hover(5, 3), "Outdoors 15 °C"; got != want {
		t.Errorf("another outdoor cell: %q, want %q", got, want)
	}
	f.onRect(a, a.hud().Global)
	f.frame(t, a)
	if got, want := a.temperatureLine(), "Outdoors 9 °C"; got != want { // over the HUD: WorldView.OutdoorTemp
		t.Errorf("over the HUD: %q, want %q", got, want)
	}
	f.cur.mx, f.cur.my = -50, -50
	f.frame(t, a)
	if got, want := a.temperatureLine(), "Outdoors 9 °C"; got != want {
		t.Errorf("off the window: %q, want %q", got, want)
	}
}

// Negative temperatures round away from zero like the rest of the HUD.
func TestFmtDegrees(t *testing.T) {
	t.Parallel()
	for in, want := range map[int]string{0: "0 °C", 94: "9 °C", 95: "10 °C", 215: "22 °C", -4: "0 °C", -5: "-1 °C", -94: "-9 °C", -95: "-10 °C"} {
		if got := fmtDegrees(in); got != want {
			t.Errorf("fmtDegrees(%d) = %q, want %q", in, got, want)
		}
	}
}

// Active conditions are one line each (the chips' successors), right
// aligned in Global with the chips' tooltips.
func TestGlobalConditionsAreLines(t *testing.T) {
	t.Parallel()
	s := newGlobalSim()
	a, _ := globalColonyP(t, s)
	h0 := a.globalHeight()
	s.world.Extreme, s.world.Conditions = "heat_wave", []string{"ashfall_sky"}
	a.updateOnce(t)
	chips := statusChips(a.renderer.world())
	if len(chips) != 2 {
		t.Fatalf("chips = %+v", chips)
	}
	row := textRowH()
	if h1 := a.globalHeight(); h1 < h0+2*row-1 {
		t.Fatalf("two conditions: Global %v -> %v, want about two rows (%v) taller", h0, h1, row)
	}
	g := a.hud().Global
	rs := a.chipRects(chips, a.sw)
	if len(rs) != 2 {
		t.Fatalf("rects = %+v", rs)
	}
	for i, r := range rs {
		if r.X < g.X || r.right() > g.right() || r.Y < g.Y || r.bottom() > g.bottom() {
			t.Errorf("condition %d %+v outside Global %+v", i, r, g)
		}
		if r.right() != g.right() {
			t.Errorf("condition %d is not right-aligned: ends at %v, Global at %v", i, r.right(), g.right())
		}
	}
	if rs[1].Y <= rs[0].Y {
		t.Errorf("conditions do not stack downward: %+v", rs)
	}
	if got := tipOf(t, a, "hud.chip.1"); !strings.Contains(got, "Ashfall sky") {
		t.Errorf("condition tip = %q", got)
	}
	if got := tipOf(t, a, "hud.chip.0"); !strings.Contains(got, "Heat wave") {
		t.Errorf("condition tip = %q", got)
	}
}

// updateOnce runs one Update so the HUD's widgets follow the world.
func (a *App) updateOnce(t *testing.T) {
	t.Helper()
	if err := a.Update(); err != nil {
		t.Fatal(err)
	}
}

// Global is exactly its measured stack: it ends 8 px above the tab strip,
// every row sits inside it, and no two rows overlap.
func TestGlobalStackIsMeasured(t *testing.T) {
	for _, sc := range globalScales {
		t.Run(fmt.Sprintf("ui%d", sc.ui), func(t *testing.T) {
			s := newGlobalSim()
			s.world.Extreme, s.world.Conditions = "cold_snap", []string{"ashfall_sky"}
			a, _ := globalColony(t, s, sc.ui, sc.ww, sc.wh)
			a.renderer.roofOverlay = true // the legend joins the stack
			a.updateOnce(t)
			l := a.hud()
			if l.Global.H != a.globalHeight() || l.Global.H <= 0 {
				t.Fatalf("Global %+v vs globalHeight %v", l.Global, a.globalHeight())
			}
			if l.Global.bottom() != l.Tabs.Y-hudMargin || l.Global.X != float32(a.sw)-240 || l.Global.W != 232 {
				t.Fatalf("Global %+v not at the lower-right above Tabs %+v", l.Global, l.Tabs)
			}
			rows := a.globalRows()
			var all []rect
			add := func(name string, r rect) {
				t.Helper()
				if r.empty() {
					return
				}
				if r.X < l.Global.X || r.right() > l.Global.right() || r.Y < l.Global.Y || r.bottom() > l.Global.bottom() {
					t.Errorf("row %s %+v outside Global %+v", name, r, l.Global)
				}
				for _, o := range all {
					if i := r.intersect(o); i.W > 0 && i.H > 0 {
						t.Errorf("row %s %+v overlaps %+v", name, r, o)
					}
				}
				all = append(all, r)
			}
			for i, r := range rows.Toggles {
				add(fmt.Sprintf("toggle%d", i), r)
			}
			for i, r := range rows.Speed {
				add(fmt.Sprintf("speed%d", i), r)
			}
			add("reason", rows.Reason)
			add("hour", rows.Hour)
			add("date", rows.Date)
			add("weather", rows.Weather)
			add("temp", rows.Temp)
			for i, r := range rows.Conds {
				add(fmt.Sprintf("cond%d", i), r)
			}
			add("legend", rows.Legend)
			if rows.Legend.empty() {
				t.Error("the roof legend is not in the stack while the overlay is on")
			}
			if rows.Legend.Y > rows.Conds[0].Y {
				t.Errorf("the legend %+v should be the top line, conditions at %+v", rows.Legend, rows.Conds[0])
			}
			if rows.Toggles[0].bottom() < rows.Speed[0].bottom() {
				t.Errorf("toggles %+v should be below the speed buttons %+v", rows.Toggles[0], rows.Speed[0])
			}
		})
	}
}

// The hour line and the date line show the renderer's date and the
// schedule activity; the date line carries the season.
func TestGlobalDateLines(t *testing.T) {
	t.Parallel()
	s := newGlobalSim()
	a, _ := globalColonyP(t, s)
	s.stubSim.tick = 3*clock.TicksPerDay + 8*clock.TicksPerHour + 500 // a few days in, about 08:12
	hour, date := a.hourLine(), a.dateLine()
	if !strings.Contains(hour, "08:") || !strings.Contains(hour, "work") {
		t.Errorf("hour line %q should show the time and the activity", hour)
	}
	if !strings.Contains(date, "Spring") {
		t.Errorf("date line %q should carry the season", date)
	}
	if w := textW(date, tsBody); w > 224 {
		t.Errorf("date line %q is %v px wide, more than Global's inner width", date, w)
	}
	// a long calendar still fits in a 232 px column
	long := dateSeasonLine(clock.Date{Year: 11, Quadrum: 3, Day: 14, Hour: 23}, "Midwinter", 224)
	if textW(long, tsBody) > 224 {
		t.Errorf("long date %q is %v px", long, textW(long, tsBody))
	}
}

// Global draws (every state) without raw drawing; smoke test the paint.
func TestGlobalDrawsAtScales(t *testing.T) {
	for _, sc := range globalScales {
		t.Run(fmt.Sprintf("ui%d", sc.ui), func(t *testing.T) {
			s := newGlobalSim()
			s.world.Extreme, s.world.Conditions = "heat_wave", []string{"ashfall_sky"}
			s.world.Weather, s.world.WindX, s.world.WindY = "rain", 2, 1
			a, _ := globalColony(t, s, sc.ui, sc.ww, sc.wh)
			a.renderer.roofOverlay, a.rooms = true, true
			a.bg.paused = true
			screen := ebiten.NewImage(a.pw, a.ph)
			a.drawGlobalControls(screen, a.sw, a.sh)
		})
	}
}

// Over the lower-right corner nothing leaks past Global into the map
// click path: its centre consumes a click, the map beside it does not.
func TestGlobalConsumesClicks(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	g := a.hud().Global
	cx, cy := g.center()
	if !a.colonyChromeHit(int(cx), int(cy), a.sw, a.sh) {
		t.Fatal("Global's centre does not consume the click")
	}
	assertChromeIsTheLayout(t, a) // the layout covers Global: no legacy chrome
}

// More than eight toggles wrap into rows of eight, right-aligned, upward
// from the bottom; the measured height grows by a row each time.
func TestLayoutGlobalWrapsToggleRows(t *testing.T) {
	t.Parallel()
	g := rect{1040, 400, 232, 0}
	var prev float32
	for _, n := range []int{8, 9, 16, 17} {
		h := globalStackHeight(globalPlan{}, n)
		g.H = h
		g.Y = 692 - hudMargin - h
		rows := layoutGlobal(g, globalPlan{}, n)
		if len(rows.Toggles) != n {
			t.Fatalf("n=%d: %d toggle rects", n, len(rows.Toggles))
		}
		for i, r := range rows.Toggles {
			if r.X < g.X || r.right() > g.right() || r.Y < g.Y || r.bottom() > g.bottom() {
				t.Errorf("n=%d toggle %d %+v outside %+v", n, i, r, g)
			}
			if i >= globalPerRow && r.Y <= rows.Toggles[i-globalPerRow].Y {
				t.Errorf("n=%d: toggle %d %+v should be below toggle %d %+v (rows run top to bottom)", n, i, r, i-globalPerRow, rows.Toggles[i-globalPerRow])
			}
		}
		if last := rows.Toggles[n-1]; g.right()-globalPad != last.right() {
			t.Errorf("n=%d: the last toggle ends at %v, want the inner right edge %v", n, last.right(), g.right()-globalPad)
		}
		if h < prev {
			t.Errorf("n=%d: height %v shrank from %v", n, h, prev)
		}
		prev = h
	}
	if h8, h9 := globalStackHeight(globalPlan{}, 8), globalStackHeight(globalPlan{}, 9); h9-h8 != globalIconPx+globalIconGap {
		t.Errorf("a second toggle row adds %v, want %v", h9-h8, globalIconPx+globalIconGap)
	}
}

// The weather row's width follows the label: calm is plain text, wind has
// the arrow's room.
func TestWeatherLabelWidth(t *testing.T) {
	t.Parallel()
	if weatherLabelW(game.WorldView{}) != 0 {
		t.Fatal("no weather has no width")
	}
	calm := game.WorldView{Weather: "clear"}
	if got, want := weatherLabelW(calm), textW("Clear, calm", tsBody); got != want {
		t.Errorf("calm width %v, want %v", got, want)
	}
	windy := game.WorldView{Weather: "rain", WindX: 2, WindY: 1}
	lbl := weatherLabel(windy)
	if got := weatherLabelW(windy); got <= textW("Rain", tsBody)+20 || got > textW(lbl, tsBody)+40 {
		t.Errorf("windy width %v for %q", got, lbl)
	}
}

// A click on an unknown speed id changes nothing, and with no colony the
// global controls are inert.
func TestGlobalInertWithoutAColony(t *testing.T) {
	t.Parallel()
	a, _ := globalColonyP(t, newGlobalSim())
	a.renderer.speed = speedFast
	a.applySpeedClick("speed.9")
	if a.renderer.speed != speedFast {
		t.Fatalf("speed.9 changed the speed to %v", a.renderer.speed)
	}
	menu := testApp(&stubSim{w: 4, h: 4})
	if menu.globalHeight() != 0 || menu.globalButtons(100) != nil || menu.minimapOn() {
		t.Fatal("the menu has no global controls")
	}
	menu.applySpeedClick("speed.1")
	menu.globalUI(100, 100)
	if _, ok := menu.globalToggleRect("hud.roof"); ok {
		t.Fatal("a toggle rect without a colony")
	}
}
