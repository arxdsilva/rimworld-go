package hud

import (
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The lower-right global controls (PLAY-018: GLOBAL owns them). One panel,
// the layout's Global rect, holds a measured stack. Bottom-up from the
// tab strip:
//
//	overlay toggles   24 px icons in rows of 8, right-aligned
//	speed buttons     pause, normal, fast, superfast (52x24)
//	speed reason      only while the game paused itself (threat, background)
//	hour line         clock and the schedule's current activity
//	date line         the calendar date and the season
//	weather           the weather and the wind arrow
//	temperature       the hovered cell's air: "Indoors 18 °C"
//	conditions        one line per active condition (chips.go)
//	legends           the roof and power keys, while those overlays are on
//
// Height is that stack's measured height; the layout reserves it (the
// root's memo), so nothing here may ask for the layout while measuring.

const (
	GlobalPad     = 4  // inside the panel
	GlobalIconPx  = 24 // an overlay toggle
	GlobalIconGap = 4  // between toggles and between speed buttons
	GlobalPerRow  = 8  // toggles per row
	speedBtnW     = 52
	speedBtnH     = 24
	globalGap     = 4 // between the button rows and the text above
	GlobalInnerW  = hudlayout.RightW - 2*GlobalPad

	learnButtonID  = "hud.learn"
	barButtonID    = "hud.bar"
	rescatButtonID = "hud.rescat"

	// RoofButtonID, HomeButtonID, PlanButtonID and RoomsButtonID are the
	// legacy toggles the shell's click routes answer.
	RoofButtonID  = "hud.roof"
	HomeButtonID  = "hud.home"
	PlanButtonID  = "hud.plan"
	RoomsButtonID = "hud.rooms"
	ZonesButtonID = "hud.zones"
	// HomeAutoButtonID and PowerButtonID are the Architect's toggles
	// (architect.HomeAutoButtonID, architect.PowerButtonID; the root's
	// tests hold them equal).
	HomeAutoButtonID = "hud.homeauto"
	PowerButtonID    = "hud.power"

	roofLegendTxt = "Roof: blue built · slate natural · dashed planned · hatched removal"
)

// GlobalToggle is one overlay toggle. The legacy five route through the
// shell's click routes as plain buttons; the settings toggles and the
// power grid toggle (run != nil) are widget-kit widgets, which the kit
// answers itself.
type GlobalToggle struct {
	ID, Icon, Short string
	on              func(c Global) bool
	tip             func(c Global) string
	run             func(c Global)
}

// toggleGameplay flips one gameplay setting and saves it the way the
// Options screen does.
func (c Global) toggleGameplay(flip func(g *config.Gameplay)) {
	s := c.sh.Settings()
	flip(&s.Gameplay)
	c.sh.SetSettings(s)
}

// GlobalToggles is the toggles, left to right.
var GlobalToggles = []GlobalToggle{
	{ID: RoofButtonID, Icon: "overlay_roof", Short: "Roof",
		on:  func(c Global) bool { return c.sh.RoofShown() },
		tip: func(c Global) string { return "Roof overlay" + c.sh.KeyHint(input.RoofOverlay) }},
	{ID: HomeButtonID, Icon: "home", Short: "Home",
		on:  func(c Global) bool { return c.sh.HomeShown() },
		tip: func(c Global) string { return "Home area overlay" }},
	{ID: HomeAutoButtonID, Icon: "home_auto", Short: "Auto",
		on: func(c Global) bool { return c.sh.World().HomeAuto },
		tip: func(c Global) string {
			if c.sh.World().HomeAuto {
				return "Home auto-expand: on (click to turn off)"
			}
			return "Home auto-expand: off (click to turn on)"
		}},
	{ID: PlanButtonID, Icon: "plan", Short: "Plans",
		on:  func(c Global) bool { return c.sh.PlanShown() },
		tip: func(c Global) string { return "Plans overlay" }},
	{ID: RoomsButtonID, Icon: "overlay_rooms", Short: "Rooms",
		on:  func(c Global) bool { return c.Rooms },
		tip: func(c Global) string { return "Room stats overlay" + c.sh.KeyHint(input.RoomStats) }},
	{ID: learnButtonID, Icon: "learn", Short: "Help",
		on: func(c Global) bool { return c.sh.Settings().Gameplay.ShowLearningHelper },
		tip: func(c Global) string {
			return "Learning helper" + stateSuffix(c.sh.Settings().Gameplay.ShowLearningHelper) + "\nShort lessons in the top-right corner. Click to show or hide them."
		},
		run: func(c Global) {
			c.toggleGameplay(func(g *config.Gameplay) { g.ShowLearningHelper = !g.ShowLearningHelper })
		}},
	{ID: barButtonID, Icon: "colonist", Short: "Bar",
		on: func(c Global) bool { return c.sh.Settings().Gameplay.ShowColonistBar },
		tip: func(c Global) string {
			return "Colonist bar" + stateSuffix(c.sh.Settings().Gameplay.ShowColonistBar) + "\nThe portraits along the top. Click to show or hide them."
		},
		run: func(c Global) { c.toggleGameplay(func(g *config.Gameplay) { g.ShowColonistBar = !g.ShowColonistBar }) }},
	{ID: rescatButtonID, Icon: "zone_stockpile", Short: "Res", // the list counts stockpiled goods
		on: func(c Global) bool { return c.sh.Settings().Gameplay.CategorizedResources },
		tip: func(c Global) string {
			return "Categorised resources" + stateSuffix(c.sh.Settings().Gameplay.CategorizedResources) + "\nGroup the resource list by category. Click to switch."
		},
		run: func(c Global) {
			c.toggleGameplay(func(g *config.Gameplay) { g.CategorizedResources = !g.CategorizedResources })
		}},
	// PLAY-021: the ninth, so it opens a second row (power_overlay.go).
	{ID: PowerButtonID, Icon: "overlay_power", Short: "Power",
		on:  func(c Global) bool { return c.sh.PowerShown() },
		tip: func(c Global) string { _, tip := c.sh.Power(); return tip },
		run: func(c Global) { c.sh.TogglePower() }},
	{ID: ZonesButtonID, Icon: "zone_stockpile", Short: "Zones",
		on: func(c Global) bool { return !c.ZonesOff },
		tip: func(c Global) string {
			return "Zones overlay" + stateSuffix(!c.ZonesOff) + c.sh.KeyHint(input.ZonesOverlay) + "\nShow or hide every zone on the map; each storage zone has its own colour."
		}},
}

func stateSuffix(on bool) string {
	if on {
		return " (on)"
	}
	return " (off)"
}

// speedButtons are the four speed buttons, left to right; tier is the
// clock's speed tier (0 paused … 3 superfast).
var speedButtons = []struct {
	id, icon, short string
	tier            int
	act             input.Action
	name            string
}{
	{"speed.0", "speed_pause", "||", 0, input.Pause, "Pause / resume"},
	{"speed.1", "speed_normal", ">", 1, input.Speed1, "Normal speed"},
	{"speed.2", "speed_fast", ">>", 2, input.Speed2, "Fast speed"},
	{"speed.3", "speed_superfast", ">>>", 3, input.Speed3, "Superfast speed"},
}

// GlobalPlan is the stack's content, found without the layout.
type GlobalPlan struct {
	Reason  string       // why the game is paused ("" none)
	Weather bool         // a weather row
	Chips   []StatusChip // active conditions, one line each
	Legend  []string     // the roof and power legends, wrapped (nil: none)
}

// TextRowH is a text row of the stack: the body line, whose height already
// holds its leading.
func TextRowH() float32 { return gfx.LineStep(gfx.Body) }

// Plan reads the stack's content from the world and the renderer.
func (c Global) Plan() GlobalPlan {
	if !c.live() {
		return GlobalPlan{}
	}
	w := c.sh.World()
	p := GlobalPlan{Reason: c.sh.PauseReason(), Weather: w.Weather != "", Chips: StatusChips(w)}
	if line := c.RoofLegend(); line != "" {
		p.Legend = gfx.WrapPx(line, gfx.Body, GlobalInnerW)
	}
	if line, _ := c.sh.Power(); line != "" { // power_overlay.go
		p.Legend = append(p.Legend, gfx.WrapPx(line, gfx.Body, GlobalInnerW)...)
	}
	return p
}

// GlobalRowRects are the stack's rows. Text rows are the Global rect's
// inner width; a condition's rect is as wide as its label, flush right.
type GlobalRowRects struct {
	Toggles                           []gfx.Rect
	Speed                             [4]gfx.Rect
	Reason, Hour, Date, Weather, Temp gfx.Rect
	Conds                             []gfx.Rect
	Legend                            gfx.Rect
	top                               float32 // where the stack ends, pad included
}

// LayoutGlobal places the stack bottom-up from the bottom of g.
func LayoutGlobal(g gfx.Rect, p GlobalPlan, nToggles int) GlobalRowRects {
	var o GlobalRowRects
	text := TextRowH()
	right := g.Right() - GlobalPad
	y := g.Bottom() - GlobalPad
	rows := (nToggles + GlobalPerRow - 1) / GlobalPerRow
	o.Toggles = make([]gfx.Rect, nToggles)
	for row := rows - 1; row >= 0; row-- {
		y -= GlobalIconPx
		first, last := row*GlobalPerRow, min(nToggles, (row+1)*GlobalPerRow)
		x := right - float32(last-first)*GlobalIconPx - float32(last-first-1)*GlobalIconGap
		for i := first; i < last; i++ {
			o.Toggles[i] = gfx.Rect{X: x, Y: y, W: GlobalIconPx, H: GlobalIconPx}
			x += GlobalIconPx + GlobalIconGap
		}
		if row > 0 {
			y -= GlobalIconGap
		}
	}
	y -= globalGap + speedBtnH
	x := right - 4*speedBtnW - 3*GlobalIconGap
	for i := range o.Speed {
		o.Speed[i] = gfx.Rect{X: x, Y: y, W: speedBtnW, H: speedBtnH}
		x += speedBtnW + GlobalIconGap
	}
	y -= globalGap
	next := func() gfx.Rect {
		y -= text
		return gfx.Rect{X: g.X + GlobalPad, Y: y, W: g.W - 2*GlobalPad, H: text}
	}
	if p.Reason != "" {
		o.Reason = next()
	}
	o.Hour = next()
	o.Date = next()
	if p.Weather {
		o.Weather = next()
	}
	o.Temp = next()
	o.Conds = make([]gfx.Rect, len(p.Chips))
	for i := len(p.Chips) - 1; i >= 0; i-- {
		r := next()
		w := min(gfx.TextW(p.Chips[i].Label, gfx.Body)+2*GlobalPad, g.W)
		o.Conds[i] = gfx.Rect{X: g.Right() - w, Y: r.Y, W: w, H: r.H}
	}
	if n := len(p.Legend); n > 0 {
		y -= float32(n) * text
		o.Legend = gfx.Rect{X: g.X + GlobalPad, Y: y, W: g.W - 2*GlobalPad, H: float32(n) * text}
	}
	o.top = y - GlobalPad
	return o
}

// GlobalStackHeight is the stack's measured height.
func GlobalStackHeight(p GlobalPlan, nToggles int) float32 {
	return -LayoutGlobal(gfx.Rect{X: 0, Y: 0, W: hudlayout.RightW, H: 0}, p, nToggles).top
}

// Height is the lower-right controls' height the layout reserves; it
// never asks for the layout. (The root tells the minimap, which sits
// above Global.)
func (c Global) Height() float32 {
	if !c.live() {
		return 0
	}
	return GlobalStackHeight(c.Plan(), len(GlobalToggles))
}

// Rows is this frame's stack, placed in the layout's Global rect.
func (c Global) Rows() GlobalRowRects {
	return LayoutGlobal(c.sh.Layout().Global, c.Plan(), len(GlobalToggles))
}

// ToggleRect is the rect of toggle id.
func (c Global) ToggleRect(id string) (gfx.Rect, bool) {
	if !c.live() {
		return gfx.Rect{}, false
	}
	rows := c.Rows()
	for i, tg := range GlobalToggles {
		if tg.ID == id {
			return rows.Toggles[i], true
		}
	}
	return gfx.Rect{}, false
}

// toggleButton is toggle i as a button at r.
func (c Global) toggleButton(i int, r gfx.Rect) uikit.Button {
	tg := GlobalToggles[i]
	return uikit.Button{ID: tg.ID, Label: tg.Short, X: r.X, Y: r.Y, W: r.W, H: r.H, Selected: tg.on(c), Tip: tg.tip(c)}
}

// Buttons are the global controls' buttons: the legacy overlay toggles
// and the four speed buttons.
func (c Global) Buttons(sw int) []uikit.Button {
	if !c.live() {
		return nil
	}
	rows := c.Rows()
	var out []uikit.Button
	for i, tg := range GlobalToggles {
		if tg.run == nil {
			out = append(out, c.toggleButton(i, rows.Toggles[i]))
		}
	}
	for i, sb := range speedButtons {
		r := rows.Speed[i]
		out = append(out, uikit.Button{ID: sb.id, Label: sb.short, X: r.X, Y: r.Y, W: r.W, H: r.H,
			Selected: c.sh.SpeedTier() == sb.tier, Tip: sb.name + c.sh.KeyHint(sb.act)})
	}
	return out
}

// SpeedClick handles a speed button ("speed.<n>"): the same intents the
// time keys use.
func (c Global) SpeedClick(id string) {
	if !c.live() {
		return
	}
	for _, sb := range speedButtons {
		if sb.id != id {
			continue
		}
		if sb.tier == 0 {
			c.sh.TogglePause()
		} else {
			c.sh.SetSpeed(sb.tier)
		}
		return
	}
}

// Declare declares the global controls' widget-kit parts: the settings
// toggles (clickable) and the hover cards of the text rows.
func (c Global) Declare(sw, sh int) {
	if !c.live() {
		return
	}
	rows := c.Rows()
	u := c.kit()
	for i, tg := range GlobalToggles {
		if tg.run == nil {
			continue
		}
		u.Add(uikit.Widget{ID: tg.ID, Label: tg.Short, R: rows.Toggles[i], Tip: tg.tip(c), Click: func() { tg.run(c) }})
	}
	w := c.sh.World()
	u.Tip("hud.tip.hour", rows.Hour, "Time\n"+c.TimeText())
	u.Tip("hud.tip.date", rows.Date, c.dateTip(w))
	if !rows.Weather.Empty() {
		u.Tip("hud.tip.weather", rows.Weather, "Weather\nThe sky and the wind (the arrow shows where it blows).")
	}
	u.Tip("hud.tip.temp", rows.Temp, "Temperature\nThe air under the cursor: indoors inside a room, outdoors elsewhere. Over the HUD it shows the outdoor temperature.")
	if !rows.Reason.Empty() {
		u.Tip("hud.tip.reason", rows.Reason, "Paused\n"+c.sh.PauseReason())
	}
}

// ---------------------------------------------------------------- Text rows

// activity is the schedule's current activity (none off the colony).
func (c Global) activity() string {
	if g := c.sh.Env().Game; g != nil {
		return g.Activity().String()
	}
	return ""
}

// TimeText is the clock text with the speed: the date, the speed label
// (which says why the game paused itself) and the schedule's activity.
func (c Global) TimeText() string {
	return fmt.Sprintf("%s  %s  %s", c.sh.Date(), c.sh.SpeedLabel(), c.activity())
}

// HourLine is the clock and the schedule's current activity.
func (c Global) HourLine() string {
	d := c.sh.Date()
	return fmt.Sprintf("%02d:%02d  %s · %s", d.Hour, d.Minute, d.Period(), c.activity())
}

// DateLine is the calendar date and the season.
func (c Global) DateLine() string {
	return DateSeasonLine(c.sh.Date(), c.sh.World().Season, GlobalInnerW)
}

// DateSeasonLine is the date and the season in at most maxW px: the long
// form, else a compact one.
func DateSeasonLine(d clock.Date, season string, maxW float64) string {
	forms := []string{
		fmt.Sprintf("Day %d, Quadrum %d, Year %d", d.Day+1, d.Quadrum+1, d.Year+1),
		fmt.Sprintf("D%d Q%d Y%d", d.Day+1, d.Quadrum+1, d.Year+1),
	}
	var s string
	for _, f := range forms {
		s = f
		if season != "" {
			s += " · " + season
		}
		if w, _ := gfx.Measure(s, gfx.Body); w <= maxW {
			return s
		}
	}
	return gfx.Ellipsize(s, gfx.Body, maxW)
}

// dateTip is the date row's hover card: the biome and its climate, else
// the full date.
func (c Global) dateTip(w game.WorldView) string {
	lines := BiomeTooltip(w)
	if len(lines) == 0 {
		return "Date\n" + c.sh.Date().String()
	}
	return SeasonText(w) + "\n" + strings.Join(lines[1:], "\n")
}

// FmtDegrees is a temperature in tenths of a degree C as "18 °C",
// rounding half away from zero like FmtTemp.
func FmtDegrees(tenths int) string {
	if tenths < 0 {
		return fmt.Sprintf("%d °C", (tenths-5)/10)
	}
	return fmt.Sprintf("%d °C", (tenths+5)/10)
}

// TemperatureLine is "Indoors 18 °C" or "Outdoors 9 °C" for the cell under
// the cursor; over the HUD (or off the map) it is the outdoor temperature.
func (c Global) TemperatureLine() string {
	w := c.sh.World()
	t, indoors := w.OutdoorTemp, false
	if tx, ty, ok := c.Cell().hovered(); ok { // cellinfo.go
		t = c.sh.Env().Game.TemperatureAt(tx, ty)
		_, indoors = tileTemp(w, tx, ty)
	}
	where := "Outdoors"
	if indoors {
		where = "Indoors"
	}
	return where + " " + FmtDegrees(t)
}

// ---------------------------------------------------------------- Drawing

// rightText draws s right-aligned at x in the text row r.
func rightText(screen *ebiten.Image, s string, x float32, r gfx.Rect, st gfx.TextStyle) {
	st.Align = gfx.AlignRight
	gfx.Text(screen, s, float64(x), float64(r.Y), st)
}

// Draw draws the Global panel and its stack.
func (c Global) Draw(screen *ebiten.Image, sw, sh int) {
	if !c.live() {
		return
	}
	g := c.sh.Layout().Global
	if g.Empty() {
		return
	}
	plan := c.Plan()
	rows := LayoutGlobal(g, plan, len(GlobalToggles))
	uikit.DrawPanel(screen, g.X, g.Y, g.W, g.H)
	right := g.Right() - GlobalPad
	w := c.sh.World()
	mx, my := c.sh.Cursor()

	for i, l := range plan.Legend {
		r := gfx.Rect{X: rows.Legend.X, Y: rows.Legend.Y + float32(i)*TextRowH(), W: rows.Legend.W, H: TextRowH()}
		rightText(screen, l, right, r, gfx.WithColor(gfx.Body, uikit.TextDim))
	}
	drawConditionLines(screen, plan.Chips, rows.Conds) // chips.go
	rightText(screen, c.TemperatureLine(), right, rows.Temp, gfx.Body)
	if !rows.Weather.Empty() {
		c.sh.DrawWeather(screen, int(right), int(rows.Weather.Y)+2, w) // the weather label lifts its text 2 px
	}
	rightText(screen, c.DateLine(), right, rows.Date, gfx.Body)
	rightText(screen, c.HourLine(), right, rows.Hour, gfx.Body)
	if !rows.Reason.Empty() {
		rightText(screen, plan.Reason, right, rows.Reason, gfx.WithColor(gfx.Body, uikit.Warning))
	}
	tier := c.sh.SpeedTier()
	for i, sb := range speedButtons {
		c.drawButton(screen, rows.Speed[i], sb.id, sb.icon, sb.short, tier == sb.tier, mx, my)
	}
	for i, tg := range GlobalToggles {
		c.drawButton(screen, rows.Toggles[i], tg.ID, tg.Icon, tg.Short, tg.on(c), mx, my)
	}
}

// drawButton draws a square or speed button: its icon, or its short
// label while the icon is not drawn yet.
func (c Global) drawButton(screen *ebiten.Image, r gfx.Rect, id, icon, short string, on bool, mx, my int) {
	b := uikit.Button{ID: id, X: r.X, Y: r.Y, W: r.W, H: r.H, Selected: on}
	uikit.DrawButton(screen, b, b.Contains(mx, my))
	if img := c.icon(icon); img != nil && icon != "" {
		size := min(r.W, r.H) - 4
		gfx.Icon(screen, img, float64(r.X+(r.W-size)/2), float64(r.Y+(r.H-size)/2), float64(size))
		return
	}
	cx, cy := r.Center()
	uikit.DrawCentered(screen, short, float64(cx), float64(cy), gfx.Small)
}

// RoofLegend is the one-line key shown while the Roof overlay is on
// (UI-010).
func (c Global) RoofLegend() string {
	if !c.live() || !c.sh.RoofShown() {
		return ""
	}
	return roofLegendTxt
}

// HomeLabelAt is "Home area" when the tile is in the Home area (SIM-021).
func HomeLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return ""
	}
	if i := y*w.TerrainW + x; i < len(w.Home) && w.Home[i] != 0 {
		return "Home area"
	}
	return ""
}

// PlanLabelAt is "Planned" when the tile carries a plan mark (PLAY-012).
func PlanLabelAt(w game.WorldView, x, y int) string {
	if w.TerrainW <= 0 || x < 0 || y < 0 || x >= w.TerrainW {
		return ""
	}
	if i := y*w.TerrainW + x; i < len(w.Plan) && w.Plan[i] != 0 {
		return "Planned"
	}
	return ""
}
