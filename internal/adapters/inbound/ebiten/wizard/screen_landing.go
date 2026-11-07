package wizard

import (
	"fmt"
	"image/color"
	"math"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/adapters/inbound/ebiten/worldinfo"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// Select Landing Site (NG-1 §2.7): the full-screen globe, the tile info
// panel on the right, the bottom bar and the Advanced and Factions
// dialogs.

// Layout metrics.
const (
	landPanelW   = 380
	landPanelPad = 12
	landBarH     = 56
	landTitleH   = 56
	landLabelW   = 158
	landModalW   = 560
)

// LandTitle heads the screen.
const LandTitle = "Choose your landing site" // keep in step with the T call in the landing draw

// landGeo is the screen's rectangles.
type landGeo struct {
	Globe, Panel, bar, title gfx.Rect
	mode, legend             gfx.Rect
	grid, search             gfx.Rect
}

func LandRects(sw, sh int) landGeo {
	w, h := float32(sw), float32(sh)
	g := landGeo{}
	g.Panel = gfx.Rect{X: w - landPanelW - landPanelPad, Y: landPanelPad, W: landPanelW, H: h - landPanelPad - landBarH - 2*landPanelPad}
	g.bar = gfx.Rect{X: (w - 740) / 2, Y: h - landBarH - landPanelPad, W: 790, H: landBarH}
	g.Globe = gfx.Rect{X: 0, Y: landTitleH, W: g.Panel.X, H: g.bar.Y - landTitleH}
	g.mode = gfx.Rect{X: g.Panel.X - 12 - 170, Y: 14, W: 170, H: 30}
	g.legend = gfx.Rect{X: g.mode.X - 8 - 90, Y: 14, W: 90, H: 30}
	g.grid = gfx.Rect{X: g.legend.X - 8 - 70, Y: 14, W: 70, H: 30}
	g.search = gfx.Rect{X: g.grid.X - 8 - 90, Y: 14, W: 90, H: 30}
	g.title = gfx.Rect{X: 0, Y: 12, W: g.search.X, H: 32} // what the buttons leave
	return g
}

// ---- map sizes -------------------------------------------------------------

// PreReleaseSide is the temporary medium map offered until the engine gate
// opens the standard sizes (NG-1 §3.10).
const PreReleaseSide = 120

// MapSizeOptions lists the map sizes: the application's when it provides
// them, else the standard and large ones gated by MaxEnabledSide plus the
// temporary pre-release option.
func MapSizeOptions() []game.MapSizeView {
	if ms := game.MapSizes(); len(ms) > 0 {
		return ms
	}
	var out []game.MapSizeView
	if game.MaxEnabledSide < 200 {
		out = append(out, game.MapSizeView{Side: PreReleaseSide, Enabled: PreReleaseSide <= game.MaxEnabledSide})
	}
	for _, s := range []int{200, 225, 250, 275, 300, 350, 400, 450, 500} {
		v := game.MapSizeView{Side: s, Enabled: s <= game.MaxEnabledSide, Large: s >= 350}
		if !v.Enabled {
			v.Reason = "Not available yet"
		}
		out = append(out, v)
	}
	return out
}

// DefaultMapSideEnabled is the map side of a fresh draft: 250 when it is
// enabled, else the largest enabled side below it.
// TestMapSides are the small maps developer mode can add (OPT-031).
func TestMapSides() []int { return []int{60, 80, 100} }

// MapSizeOptionsFor is MapSizeOptions, led by the test sizes when developer
// mode and Developer.TestMapSizes are both on. A test size is never the
// default.
func MapSizeOptionsFor(s config.Settings) []game.MapSizeView {
	opts := MapSizeOptions()
	if !s.Gameplay.DevMode || !s.Developer.TestMapSizes {
		return opts
	}
	out := make([]game.MapSizeView, 0, 3+len(opts))
	for _, side := range TestMapSides() {
		out = append(out, game.MapSizeView{Side: side, Enabled: true, Reason: ""})
	}
	return append(out, opts...)
}

func DefaultMapSideEnabled() int {
	best := min(DefaultMapSide, game.MaxEnabledSide)
	for _, o := range MapSizeOptions() {
		if o.Enabled && o.Side <= DefaultMapSide && o.Side > best {
			best = o.Side
		}
	}
	if best < 1 {
		best = PreReleaseSide
	}
	return best
}

func MapSizeLabel(o game.MapSizeView) string {
	l := fmt.Sprintf("%d x %d", o.Side, o.Side)
	if o.Side == PreReleaseSide && game.MaxEnabledSide < 200 {
		l += " (pre-release)"
	}
	if slices.Contains(TestMapSides(), o.Side) {
		l += " (test)"
	}
	return l
}

// ---- seasons ----------------------------------------------------------------

var seasonNames = []string{"Spring", "Summer", "Fall", "Winter"}

// SeasonMid is the middle day of a season at a climate: Summer centres on
// the peak day, then Fall, Winter and Spring follow a quadrum apart.
func SeasonMid(c temperature.Climate, s game.Season) int {
	k := map[game.Season]int{game.SeasonSummer: 0, game.SeasonFall: 1, game.SeasonWinter: 2, game.SeasonSpring: 3}[s]
	return (c.PeakDay + k*clock.DaysPerQuadrum + clock.DaysPerYear) % clock.DaysPerYear
}

func DayMean(c temperature.Climate, day int) int {
	day = (day%clock.DaysPerYear + clock.DaysPerYear) % clock.DaysPerYear
	return int(c.DailyMean(clock.Tick(day * clock.TicksPerDay)))
}

// AutoSeason picks the starting season for Auto (NG-1 §2.7): the first
// season, in the order Spring to Winter, whose middle day averages 8 C or
// more and is rising; the warmest season when none does.
func AutoSeason(c temperature.Climate) game.Season {
	best, bestT := game.SeasonSummer, math.MinInt
	for _, s := range []game.Season{game.SeasonSpring, game.SeasonSummer, game.SeasonFall, game.SeasonWinter} {
		mid := SeasonMid(c, s)
		t := DayMean(c, mid)
		if t >= 80 && DayMean(c, mid+1) > DayMean(c, mid-1) {
			return s
		}
		if t > bestT {
			best, bestT = s, t
		}
	}
	return best
}

// ResolvedSeason is the draft's starting season at its tile.
func (ng *Draft) ResolvedSeason() game.Season {
	if ng.Season >= 0 && ng.Season < len(seasonNames) {
		return game.Season(ng.Season)
	}
	if ng.Planet != nil && ng.Tile >= 0 {
		return AutoSeason(ng.Planet.Site(ng.Tile).Climate)
	}
	return game.SeasonSpring
}

// ---- the tile info ------------------------------------------------------------

// The card itself (BuildTileInfo, rows, search, filter, legend) lives in
// the worldinfo leaf; these keep the wizard's own callers short.

// CoordText writes a position as "34.2°S 121.7°W".
func CoordText(lat, lon int16) string { return worldinfo.CoordText(lat, lon) }

// LandingDetail is the biome card's text in Celsius, wrapped to cols.
func LandingDetail(b game.BiomeInfo, cols int) []string {
	return worldinfo.LandingDetail(b, cols, units.Celsius)
}

// ---- the screen ------------------------------------------------------------------

// landingGlobe is the draft's globe, made when the planet changes.
func (w landingPage) landingGlobe(ng *Draft) *globe.View {
	if ng.Planet == nil {
		return nil
	}
	if ng.Globe == nil || ng.globeOf != ng.Planet {
		ng.Globe, ng.globeOf = globe.New(ng.Planet, w.art()), ng.Planet
		ng.Land.Box.Reset()
		ng.Globe.Select(ng.Tile)
		if ng.Tile >= 0 {
			ng.Globe.Center(ng.Tile)
		}
	}
	return ng.Globe
}

// tileReason is why Next is disabled for the draft's tile ("" it isn't).
func (ng *Draft) tileReason(tr lang.Tr) string {
	if ng.Planet == nil {
		return tr.T("Generate a world first.")
	}
	if ng.Tile < 0 {
		if ng.Outside {
			return planet.ReasonOutside
		}
		return tr.T("Choose a landing site first.")
	}
	if ok, reason := ng.Planet.ValidLanding(ng.Tile); !ok {
		return reason
	}
	return ""
}

func (w landingPage) landingUI(sw, sh int) {
	uikit.OnScreenLayer(w.kit())
	if w.kit().SetupTooSmall(sw, sh) {
		return
	}
	ng := w.draft()
	g := w.landingGlobe(ng)
	if g == nil {
		return
	}
	ng.Tile = g.Selected() // a selection made since the last frame (Select random site, tests)
	geo := LandRects(sw, sh)
	g.SetArea(float64(geo.Globe.X), float64(geo.Globe.Y), float64(geo.Globe.W), float64(geo.Globe.H))

	// The top-right controls.
	uikit.StyledButton(w.kit(), "land.mode", geo.mode, w.sh.Tf("View: %s", g.Mode().Label(w.sh)), uikit.BtnOpts{Tip: w.sh.T("View mode\nColour the globe by biome, temperature, rainfall, elevation or hilliness.")},
		func() { g.SetMode((g.Mode() + 1) % globe.NumModes) })
	uikit.StyledButton(w.kit(), "land.legend", geo.legend, w.sh.T("Legend"), uikit.BtnOpts{Selected: ng.Land.Legend,
		Tip: w.sh.T("Legend\nWhat the colours, rivers and roads on the globe mean.")}, func() { ng.Land.Legend = !ng.Land.Legend })
	uikit.StyledButton(w.kit(), "land.grid", geo.grid, w.sh.T("Grid"), uikit.BtnOpts{Selected: g.Grid(),
		Tip: w.sh.T("Grid\nShow the tile edges once you zoom in.")}, func() { g.SetGrid(!g.Grid()) })
	uikit.StyledButton(w.kit(), "land.search", geo.search, w.sh.T("Search"), uikit.BtnOpts{Selected: ng.Land.Search,
		Tip: w.sh.T("Search\nFind a named ocean, sea, lake, mountain range, desert, forest, ice cap or settlement.")}, func() { ng.Land.Search = !ng.Land.Search })

	// Bottom bar.
	w.kit().Panel(geo.bar)
	w.landBar(ng, g, geo.bar)

	// The info panel.
	w.kit().Panel(geo.Panel)
	shown := ng.Tile
	if shown < 0 && g.Hovered() >= 0 {
		shown = g.Hovered()
	}
	w.landInfoPanel(ng, geo.Panel.Inset(landPanelPad), shown, g.Hovered() == globe.ModeUncovered && ng.Tile < 0)

	// The globe takes the pointer anywhere outside the chrome and modals.
	mx, my := w.kit().Host().Cursor()
	over := w.landOverlays(ng, g, geo)
	blocked := w.kit().ModalOpen() || geo.Panel.Hit(mx, my) || geo.bar.Hit(mx, my) || geo.mode.Hit(mx, my) || geo.legend.Hit(mx, my) ||
		geo.grid.Hit(mx, my) || geo.search.Hit(mx, my)
	for _, r := range over {
		blocked = blocked || r.Hit(mx, my)
	}
	clicked, _ := g.Update(globeInputFrom(w.in(), sw, sh, blocked), w.sh.Settings().Accessibility.ReducedMotion)
	switch {
	case clicked >= 0:
		ng.Outside = false
	case clicked == globe.ModeUncovered:
		ng.Outside = true
	}
	ng.Tile = g.Selected()
}

// landOverlays declares the search box, the legend and the random-site
// filter panel in their stack under the top-right row, and returns the
// rectangles they cover.
func (w landingPage) landOverlays(ng *Draft, g *globe.View, geo landGeo) []gfx.Rect {
	var out []gfx.Rect
	u := w.units()
	y := geo.mode.Bottom() + 8
	if ng.Land.Search {
		r := ng.Land.Box.Draw(w.kit(), "land.search", geo.mode.Right(), y, ng.Planet, func(h planet.Hit) { w.pickHit(ng, g, h) })
		out = append(out, r)
		y = r.Bottom() + 8
	}
	if ng.Land.Legend {
		out = append(out, worldinfo.DrawLegend(w.kit(), geo.mode.Right(), y, g.Mode(), u))
	}
	if ng.Land.Filter {
		x := geo.bar.X + 12 + 110 + 10
		h := ng.Land.Criteria.Height()
		out = append(out, ng.Land.Criteria.Draw(w.kit(), "land.filter", x, geo.bar.Y-h-8, u))
	}
	return out
}

// pickHit selects and centres the tile of a search result and pulses the
// feature's tiles (the globe's Hl highlight).
func (w landingPage) pickHit(ng *Draft, g *globe.View, h planet.Hit) {
	tiles := []int{h.Tile}
	if fs := ng.Planet.Features(); h.Feature >= 0 && h.Feature < len(fs) {
		tiles = tiles[:0]
		for _, t := range fs[h.Feature].Tiles {
			tiles = append(tiles, int(t))
		}
	}
	ng.Tile, ng.Outside = h.Tile, false
	g.Select(h.Tile)
	g.Center(h.Tile)
	g.Highlight(tiles, searchPulseFrames)
}

// searchPulseFrames is how long a picked feature pulses.
const searchPulseFrames = 150

// units is the player's temperature formatter.
func (w landingPage) units() units.Temp {
	return units.FromUnit(w.sh.Settings().Gameplay.TempUnit)
}

// landBar declares the bottom bar's buttons.
func (w landingPage) landBar(ng *Draft, g *globe.View, bar gfx.Rect) {
	x := bar.X + 12
	y := bar.Y + (bar.H-uikit.FooterBtnH)/2
	btn := func(id, label string, bw float32, o uikit.BtnOpts, fn func()) {
		uikit.StyledButton(w.kit(), id, gfx.Rect{X: x, Y: y, W: bw, H: uikit.FooterBtnH}, label, o, fn)
		x += bw + 10
	}
	btn(ids.WizardBack, "Back", 110, uikit.BtnOpts{}, w.back)
	btn("land.random", "Select random site", 190, uikit.BtnOpts{Tip: w.sh.T("Select random site\nPick a pleasant place at random, matching your criteria if you set any. Press again for another.")}, func() { w.randomLanding(ng, g) })
	x -= 10 - 2 // the caret sits against the button it belongs to
	btn("land.filter", "v", 30, uikit.BtnOpts{Selected: ng.Land.Filter || ng.Land.Criteria.Active(),
		Tip: w.sh.T("Random site criteria\nChoose biomes, temperature, terrain, rivers, coast, roads, caves and stone for Select random site.")},
		func() { ng.Land.Filter = !ng.Land.Filter })
	btn("land.advanced", "Advanced...", 120, uikit.BtnOpts{Tip: w.sh.T("Advanced\nStarting season and map size.")}, w.pushLandingAdvanced)
	btn("land.factions", "Factions...", 120, uikit.BtnOpts{Tip: w.sh.T("Factions\nWho lives on this planet.")}, func() { w.pushFactions(ng, g) })
	reason := ng.tileReason(w.sh)
	next := func() {
		if ng.tileReason(w.sh) == "" {
			w.forward()
		}
	}
	btn(ids.WizardNext, "Next", 130, uikit.BtnOpts{Style: uikit.BtnPrimary, Disabled: reason != "", Reason: reason}, next)
	if w.in().Fired(input.Accept) && reason == "" && !w.kit().ModalOpen() && !w.kit().Editing() {
		next()
	}
	if reason != "" {
		st := uikit.DimText()
		w.kit().Label(bar.X+bar.W/2-gfx.TextW(reason, st)/2, bar.Y-float32(gfx.LineH(st))-6, reason, st)
	}
}

// randomLanding selects the next random valid site and flies there.
func (w landingPage) randomLanding(ng *Draft, g *globe.View) {
	t := ng.Planet.RandomLandingFiltered(ng.Land.Criteria.Filter(), ng.LandRolls)
	ng.LandRolls++
	if t < 0 {
		if ng.Land.Criteria.Active() {
			w.kit().Deny(w.sh.T("No tile matches."))
		} else {
			w.kit().Deny(w.sh.T("No tile on this world can be landed on."))
		}
		return
	}
	ng.Tile, ng.Outside = t, false
	g.Select(t)
	g.Center(t)
}

// ---- the info panel ---------------------------------------------------------------

// infoBlock is a part of the panel's scrolling content.
type infoBlock struct {
	h    float32
	tip  string
	draw func(dst *ebiten.Image, r gfx.Rect)
}

func (w landingPage) landInfoPanel(ng *Draft, r gfx.Rect, tile int, uncovered bool) {
	hs := gfx.Large
	hs.Color = uikit.Heading
	w.kit().Label(r.X, r.Y, w.sh.T("Tile info"), hs)
	body := gfx.Rect{X: r.X, Y: r.Y + float32(gfx.LineH(hs)) + 8, W: r.W, H: r.H - float32(gfx.LineH(hs)) - 8}
	if uncovered || tile == globe.ModeUncovered {
		w.kit().Label(body.X, body.Y, planet.ReasonOutside, uikit.DimText())
		return
	}
	if tile < 0 {
		uikit.TextLines(w.kit(), body.X, body.Y, body.W, w.sh.T("Click a tile on the globe to see what it is like. Drag to turn the planet, scroll to zoom."), uikit.DimText())
		return
	}
	info := w.tileInfoFor(ng, tile)
	blocks := w.infoBlocks(info, body.W-uikit.ScrollBarW-6)
	total := float32(0)
	for _, b := range blocks {
		total += b.h
	}
	area := w.kit().ScrollArea("land.info", body, total, uikit.ScrollOpts{Step: 40})
	y := area.View.Y - area.Off
	for i, b := range blocks {
		br := gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: b.h}
		y += b.h
		vis := br.Intersect(area.View)
		if vis.Empty() {
			continue
		}
		if b.tip != "" {
			w.kit().Tip(fmt.Sprintf("land.info.tip.%d", i), vis, b.tip)
		}
		b := b
		w.kit().Queue(func(dst *ebiten.Image) { b.draw(gfx.ClipTo(dst, area.View), br) })
	}
}

// tileInfoFor is the cached info of a tile.
func (w landingPage) tileInfoFor(ng *Draft, tile int) worldinfo.TileInfo {
	c := &ng.Info
	note := w.sh.MapSizeNote(ng.MapSize)
	u := w.units()
	day := 0
	if ng.Planet != nil && tile >= 0 && tile < ng.Planet.TileCount() {
		day = SeasonMid(ng.Planet.Site(tile).Climate, ng.ResolvedSeason())
	}
	if c.Planet != ng.Planet || c.Tile != tile || c.side != ng.MapSize || c.note != note || c.units != u || c.day != day {
		*c = infoCache{Planet: ng.Planet, Tile: tile, side: ng.MapSize, note: note, units: u, day: day,
			info: worldinfo.BuildTileInfoWith(ng.Planet, tile, worldinfo.Opts{SizeNote: note, Units: u, Day: day})}
	}
	return c.info
}

// infoCache remembers the last tile info (the nearest-settlement search
// is not free, and the hover changes every frame).
type infoCache struct {
	Planet *planet.Planet
	Tile   int
	side   int
	note   string // the map-size warning the info was made with
	units  units.Temp
	day    int
	info   worldinfo.TileInfo
}

// Card is the cached tile card.
func (c infoCache) Card() worldinfo.TileInfo { return c.info }

// infoBlocks lays the tile info out for a width.
func (w landingPage) infoBlocks(ti worldinfo.TileInfo, width float32) []infoBlock {
	var out []infoBlock
	// Header: icon, name, difficulty tag, blurb.
	nameSt := gfx.Large
	nameSt.Color = uikit.Heading
	blurbLines := gfx.WrapPx(ti.Blurb, gfx.Body, float64(width))
	hh := float32(56)
	out = append(out, infoBlock{h: hh, tip: ti.BiomeTip, draw: func(dst *ebiten.Image, r gfx.Rect) {
		if img := w.art().Image(biome.DefOf(ti.Biome).Icon(), noTint); img != nil && ti.Name != "Ocean" && ti.Name != "Lake" {
			gfx.Icon(dst, img, float64(r.X), float64(r.Y+4), 48)
		}
		gfx.Text(dst, ti.Name, float64(r.X+58), float64(r.Y+8), nameSt)
		if ti.Difficulty != "" {
			t := gfx.Small
			t.Color = uikit.Warning
			gfx.Text(dst, w.sh.Tf("Difficulty: %s", ti.Difficulty), float64(r.X+58), float64(r.Y+8)+gfx.LineH(nameSt), t)
		}
	}})
	out = append(out, infoBlock{h: float32(len(blurbLines))*float32(gfx.LineH(gfx.Body)) + 8, draw: func(dst *ebiten.Image, r gfx.Rect) {
		st := uikit.DimText()
		for i, l := range blurbLines {
			gfx.Text(dst, l, float64(r.X), float64(r.Y)+float64(i)*gfx.LineH(st), st)
		}
	}})
	if ti.Invalid != "" {
		reason := ti.Invalid
		out = append(out, infoBlock{h: wrapHeight(reason, gfx.Label, width) + 8, draw: func(dst *ebiten.Image, r gfx.Rect) {
			st := gfx.Label
			st.Color = uikit.Danger
			for i, l := range gfx.WrapPx(reason, st, float64(r.W)) {
				gfx.Text(dst, l, float64(r.X), float64(r.Y)+float64(i)*gfx.LineH(st), st)
			}
		}})
	}
	for _, wn := range ti.Warnings {
		wn := wn
		out = append(out, infoBlock{h: wrapHeight(wn, gfx.Label, width) + 4, draw: func(dst *ebiten.Image, r gfx.Rect) {
			st := gfx.Label
			st.Color = uikit.Warning
			for i, l := range gfx.WrapPx(wn, st, float64(r.W)) {
				gfx.Text(dst, l, float64(r.X), float64(r.Y)+float64(i)*gfx.LineH(st), st)
			}
		}})
	}
	if len(ti.Quadrums) > 0 {
		out = append(out, quadrumBlock(ti.Quadrums, width))
	}
	vw := width - landLabelW
	for _, row := range ti.Rows {
		row := row
		lines := gfx.WrapPx(row.Value, gfx.Body, float64(vw))
		h := max(float32(len(lines)), 1)*float32(gfx.LineH(gfx.Body)) + 6
		out = append(out, infoBlock{h: h, tip: row.Tip, draw: func(dst *ebiten.Image, r gfx.Rect) {
			ls := uikit.DimText()
			gfx.Text(dst, uikit.FitText(row.Label, ls, landLabelW-8), float64(r.X), float64(r.Y+2), ls)
			vs := gfx.Body
			vs.Color = row.Col
			for i, l := range lines {
				gfx.Text(dst, l, float64(r.X+landLabelW), float64(r.Y+2)+float64(i)*gfx.LineH(vs), vs)
			}
			uikit.FillRect(dst, gfx.Rect{X: r.X, Y: r.Bottom() - 1, W: r.W, H: 1}, uikit.Fade(uikit.PanelBorder, 0x50))
		}})
	}
	return out
}

// quadrumBlock is the card's four-quadrum temperature table.
func quadrumBlock(qs []worldinfo.Quadrum, width float32) infoBlock {
	return infoBlock{h: 2*float32(gfx.LineH(gfx.Small)) + 14, tip: "Temperature by quadrum\nThe tile's mean temperature in each quarter of the year, with the season it falls in here.",
		draw: func(dst *ebiten.Image, r gfx.Rect) {
			cw := r.W / float32(len(qs))
			for i, q := range qs {
				ls := uikit.DimText()
				ls.Align = gfx.AlignCenter
				cx := float64(r.X + cw*float32(i) + cw/2)
				gfx.Text(dst, q.Season, cx, float64(r.Y+2), ls)
				vs := gfx.Body
				vs.Align = gfx.AlignCenter
				gfx.Text(dst, q.Temp, cx, float64(r.Y+2)+gfx.LineH(ls), vs)
			}
			uikit.FillRect(dst, gfx.Rect{X: r.X, Y: r.Bottom() - 1, W: r.W, H: 1}, uikit.Fade(uikit.PanelBorder, 0x50))
		}}
}

// ---- drawing -------------------------------------------------------------------------------

func (w landingPage) drawLanding(screen *ebiten.Image, sw, sh int) {
	ng := w.draft()
	g := w.landingGlobe(ng)
	if g == nil || uikit.TooSmall(sw, sh) {
		w.DrawBackdrop(screen, sw, sh, ng.Planet)
		if uikit.TooSmall(sw, sh) {
			uikit.DrawTooSmall(screen, sw, sh, w.sh)
		}
		return
	}
	g.Draw(screen, sw, sh)
	geo := LandRects(sw, sh)
	st := gfx.Title
	st.Align = gfx.AlignCenter
	cx := float64(geo.title.X + geo.title.W/2)
	st.Color = color.RGBA{0, 0, 0, 200}
	gfx.Text(screen, w.sh.T("Choose your landing site"), cx+1.5, float64(geo.title.Y)+1.5, st)
	st.Color = uikit.Heading
	gfx.Text(screen, w.sh.T("Choose your landing site"), cx, float64(geo.title.Y), st)
}

// ---- Advanced ---------------------------------------------------------------------------------

func (w landingPage) pushLandingAdvanced() {
	ng := w.draft()
	opts := MapSizeOptionsFor(w.sh.Settings())
	h := float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad) + 26 + 5*uikit.RadioRowH + 40 + 26 + float32(len(opts))*uikit.RadioRowH
	w.kit().Push(uikit.Modal{
		ID: "land.advanced", Title: w.sh.T("Advanced"), W: landModalW, H: h,
		Draw: func(body gfx.Rect) { w.landingAdvancedBody(ng, body, opts) },
		Buttons: []uikit.ModalButton{
			{ID: "close", Label: w.sh.T("Close"), Style: uikit.BtnPrimary, Closes: true},
			{ID: "reset", Label: w.sh.T("Reset"), OnClick: func() { ng.Season, ng.MapSize = -1, DefaultMapSideEnabled() }},
		},
	})
}

func (w landingPage) landingAdvancedBody(ng *Draft, body gfx.Rect, opts []game.MapSizeView) {
	hs := gfx.Label
	hs.Color = uikit.Heading
	y := body.Y
	w.kit().Label(body.X, y, w.sh.T("Starting season"), hs)
	w.kit().Tip("land.season.tip", gfx.Rect{X: body.X, Y: y, W: body.W, H: 22}, w.sh.T("Starting season\nSeason the colony starts in, at this site."))
	y += 26
	sel := ng.Season + 1
	if w.kit().Radio("land.season", gfx.Rect{X: body.X, Y: y, W: body.W, H: 5 * uikit.RadioRowH}, append([]string{"Auto"}, seasonNames...), &sel, uikit.RadioOpts{
		Tips: []string{w.sh.Tf("Auto\nThe first season that is already warm (%s or more) and getting warmer; otherwise the warmest one.", w.units().Whole(8))},
	}) {
		ng.Season = sel - 1
	}
	y += 5*uikit.RadioRowH + 6
	note := w.sh.T("Pick a landing site to see what Auto means there.")
	if ng.Planet != nil && ng.Tile >= 0 {
		note = w.sh.Tf("Auto starts this site in %s.", seasonNames[AutoSeason(ng.Planet.Site(ng.Tile).Climate)])
	}
	w.kit().Label(body.X, y, note, uikit.DimText())
	y += 34
	w.kit().Label(body.X, y, w.sh.T("Map size"), hs)
	w.kit().Tip("land.mapsize.tip", gfx.Rect{X: body.X, Y: y, W: body.W, H: 22}, w.sh.T("Map size\nThe side of the square local map. Bigger maps take longer to build and need a faster computer."))
	y += 26
	labels := make([]string, len(opts))
	dis := make([]string, len(opts))
	tips := make([]string, len(opts))
	notes := make([]string, len(opts))
	msel := 0
	for i, o := range opts {
		labels[i], dis[i] = MapSizeLabel(o), o.Reason
		tips[i] = fmt.Sprintf("%d x %d\n%s tiles, about %d tiles across.", o.Side, o.Side, groupDigits(o.Side*o.Side), o.Side)
		if o.Enabled { // a size that is not available yet has nothing to warn about
			if notes[i] = w.sh.MapSizeNote(o.Side); notes[i] != "" {
				tips[i] += "\n" + notes[i]
			}
		}
		if o.Side == ng.MapSize {
			msel = i
		}
	}
	list := gfx.Rect{X: body.X, Y: y, W: body.W, H: float32(len(opts)) * uikit.RadioRowH}
	if w.kit().Radio("land.mapsize", list, labels, &msel, uikit.RadioOpts{Disabled: dis, Tips: tips}) {
		ng.MapSize = opts[msel].Side
	}
	w.mapSizeWarnings(list, labels, notes)
}

// mapSizeWarnings paints each size's warning (Host.MapSizeNote) at the right
// end of its row, in the warning colour, clear of the size label. The size
// stays selectable: this only draws.
func (w landingPage) mapSizeWarnings(list gfx.Rect, labels, notes []string) {
	rows := list.RowsN(len(labels), uikit.RadioRowH, 0)
	w.kit().Queue(func(dst *ebiten.Image) {
		st := gfx.WithColor(gfx.Body, uikit.Warning)
		st.Align = gfx.AlignRight
		for i, note := range notes {
			if note == "" {
				continue
			}
			row := rows[i]
			labelEnd := row.X + 2*uikit.RadioRadius + 1 + uikit.CheckGap + gfx.TextW(labels[i], gfx.Body) + 12
			room := row.Right() - 4 - labelEnd
			if room <= 0 {
				continue
			}
			gfx.Text(dst, uikit.FitText(note, st, room), float64(row.Right()-4), gfx.CenterTop(float64(row.Y), float64(row.H), st), st)
		}
	})
}
