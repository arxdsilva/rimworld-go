package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/internal/domain/world/temperature"
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

// landGeo is the screen's rectangles.
type landGeo struct {
	globe, panel, bar, title rect
	mode, legend             rect
}

func landRects(sw, sh int) landGeo {
	w, h := float32(sw), float32(sh)
	g := landGeo{}
	g.panel = rect{X: w - landPanelW - landPanelPad, Y: landPanelPad, W: landPanelW, H: h - landPanelPad - landBarH - 2*landPanelPad}
	g.bar = rect{X: (w - 740) / 2, Y: h - landBarH - landPanelPad, W: 740, H: landBarH}
	g.globe = rect{X: 0, Y: landTitleH, W: g.panel.X, H: g.bar.Y - landTitleH}
	g.title = rect{X: 0, Y: 12, W: g.panel.X, H: 32}
	g.mode = rect{X: g.panel.X - 12 - 170, Y: 14, W: 170, H: 30}
	g.legend = rect{X: g.mode.X - 8 - 90, Y: 14, W: 90, H: 30}
	return g
}

// ---- map sizes -------------------------------------------------------------

// preReleaseSide is the temporary medium map offered until the engine gate
// opens the standard sizes (NG-1 §3.10).
const preReleaseSide = 120

// mapSizeOptions lists the map sizes: the application's when it provides
// them, else the standard and large ones gated by MaxEnabledSide plus the
// temporary pre-release option.
func mapSizeOptions() []game.MapSizeView {
	if ms := game.MapSizes(); len(ms) > 0 {
		return ms
	}
	var out []game.MapSizeView
	if game.MaxEnabledSide < 200 {
		out = append(out, game.MapSizeView{Side: preReleaseSide, Enabled: preReleaseSide <= game.MaxEnabledSide})
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

// defaultMapSideEnabled is the map side of a fresh draft: 250 when it is
// enabled, else the largest enabled side below it.
func defaultMapSideEnabled() int {
	best := min(defaultMapSide, game.MaxEnabledSide)
	for _, o := range mapSizeOptions() {
		if o.Enabled && o.Side <= defaultMapSide && o.Side > best {
			best = o.Side
		}
	}
	if best < 1 {
		best = preReleaseSide
	}
	return best
}

func mapSizeLabel(o game.MapSizeView) string {
	l := fmt.Sprintf("%d x %d", o.Side, o.Side)
	if o.Side == preReleaseSide && game.MaxEnabledSide < 200 {
		l += " (pre-release)"
	}
	return l
}

// ---- seasons ----------------------------------------------------------------

var seasonNames = []string{"Spring", "Summer", "Fall", "Winter"}

// seasonMid is the middle day of a season at a climate: Summer centres on
// the peak day, then Fall, Winter and Spring follow a quadrum apart.
func seasonMid(c temperature.Climate, s game.Season) int {
	k := map[game.Season]int{game.SeasonSummer: 0, game.SeasonFall: 1, game.SeasonWinter: 2, game.SeasonSpring: 3}[s]
	return (c.PeakDay + k*clock.DaysPerQuadrum + clock.DaysPerYear) % clock.DaysPerYear
}

func dayMean(c temperature.Climate, day int) int {
	day = (day%clock.DaysPerYear + clock.DaysPerYear) % clock.DaysPerYear
	return int(c.DailyMean(clock.Tick(day * clock.TicksPerDay)))
}

// autoSeason picks the starting season for Auto (NG-1 §2.7): the first
// season, in the order Spring to Winter, whose middle day averages 8 C or
// more and is rising; the warmest season when none does.
func autoSeason(c temperature.Climate) game.Season {
	best, bestT := game.SeasonSummer, math.MinInt
	for _, s := range []game.Season{game.SeasonSpring, game.SeasonSummer, game.SeasonFall, game.SeasonWinter} {
		mid := seasonMid(c, s)
		t := dayMean(c, mid)
		if t >= 80 && dayMean(c, mid+1) > dayMean(c, mid-1) {
			return s
		}
		if t > bestT {
			best, bestT = s, t
		}
	}
	return best
}

// resolvedSeason is the draft's starting season at its tile.
func (ng *newGameState) resolvedSeason() game.Season {
	if ng.season >= 0 && ng.season < len(seasonNames) {
		return game.Season(ng.season)
	}
	if ng.planet != nil && ng.tile >= 0 {
		return autoSeason(ng.planet.Site(ng.tile).Climate)
	}
	return game.SeasonSpring
}

// ---- the tile info ------------------------------------------------------------

// infoRow is a label/value line of the tile panel.
type infoRow struct {
	label, value, tip string
	col               color.RGBA
}

// tileInfo is everything the panel shows for one tile.
type tileInfo struct {
	tile      int
	biome     biome.ID
	name      string
	blurb     string
	difficult string
	biomeTip  string
	rows      []infoRow
	warnings  []string
	invalid   string // why the tile can't be landed on ("" valid)
	outside   bool
}

func coordText(lat, lon int16) string {
	ns, ew := "N", "E"
	if lat < 0 {
		ns, lat = "S", -lat
	}
	if lon < 0 {
		ew, lon = "W", -lon
	}
	return fmt.Sprintf("%d.%d°%s %d.%d°%s", lat/10, lat%10, ns, lon/10, lon%10, ew)
}

func celsius(t temperature.Temp) string { return fmt.Sprintf("%.0f°C", float64(t)/10) }

// hillsTip explains hilliness for the local map.
func hillsTip(h site.Hills) string {
	switch h {
	case site.SmallHills:
		return "Small hills\nA few rocky rises. Some mining, easy building."
	case site.LargeHills:
		return "Large hills\nMuch of the map is rock. Good for mining and defence."
	case site.Mountainous:
		return "Mountainous\nMostly mountain. Building room is scarce but ore is plentiful."
	}
	return "Flat\nOpen ground. Easy to build, little rock to mine."
}

// movementDifficulty is the informational travel multiplier of a tile
// (x1.0 to x3.0) from its hills and biome.
func movementDifficulty(h site.Hills, b biome.ID) float64 {
	v := 1.0
	switch h {
	case site.SmallHills:
		v += 0.3
	case site.LargeHills:
		v += 0.7
	case site.Mountainous:
		v += 1.2
	}
	switch b {
	case biome.ColdBog, biome.TemperateSwamp, biome.TropicalSwamp:
		v += 0.5
	case biome.IceSheet, biome.SeaIce:
		v += 0.6
	case biome.TropicalRainforest:
		v += 0.3
	case biome.ExtremeDesert, biome.Desert:
		v += 0.2
	case biome.Tundra:
		v += 0.1
	}
	return math.Min(3, math.Round(v*10)/10)
}

// forageLevel is the wild food on offer, from the biome's berry bushes.
func forageLevel(b biome.ID) string {
	n := 0
	for _, p := range biome.DefOf(b).Plants {
		if p.Species == plant.BerryBush {
			n += p.Permil
		}
	}
	switch {
	case n >= 25:
		return "High"
	case n >= 10:
		return "Medium"
	}
	return "Low"
}

// wildlifeLevel is the animal density label of a biome.
func wildlifeLevel(b biome.ID) string {
	w := biome.DefOf(b).Wildlife
	switch {
	case w <= 15:
		return "Sparse"
	case w <= 45:
		return "Low"
	case w <= 85:
		return "Moderate"
	case w < 100:
		return "Plentiful"
	}
	return "Abundant"
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// buildTileInfo reads tile i of p for the panel; mapSide is the chosen
// map size and total the computer's memory in bytes (0 unknown), for the
// map-size warning.
func buildTileInfo(p *planet.Planet, i, mapSide int, total uint64) tileInfo {
	if i < 0 || i >= p.TileCount() {
		return tileInfo{tile: i, outside: true}
	}
	s := p.Site(i)
	d := biome.DefOf(p.Biome[i])
	ti := tileInfo{tile: i, biome: p.Biome[i], name: d.Name, blurb: d.Blurb, difficult: d.Difficulty}
	if p.Water[i] == planet.Ocean {
		ti.name, ti.blurb, ti.difficult = "Ocean", "Open sea. Nobody can land here.", ""
	} else if p.Water[i] == planet.Lake {
		ti.name, ti.blurb, ti.difficult = "Lake", "Fresh water. Nobody can land here.", ""
	}
	ti.biomeTip = d.Name + "\n" + strings.Join(landingDetail(game.BiomeInfoOf(p.Biome[i]), 60)[2:], "\n")
	if ok, reason := p.ValidLanding(i); !ok {
		ti.invalid = reason
	}
	lo, hi := s.Climate.Range()
	grow := p.GrowingDays(i)
	growText := fmt.Sprintf("%d of %d days", grow, clock.DaysPerYear)
	if grow >= clock.DaysPerYear {
		growText = "Year-round"
	}
	var stones []string
	for _, st := range s.Stones {
		stones = append(stones, st.Label())
	}
	water := []string{}
	if l := s.River.Size.Label(); l != "" {
		water = append(water, l)
	}
	if s.Coast != 0 {
		water = append(water, "Coast")
	}
	if s.Lakes > 0 {
		water = append(water, "Lake")
	}
	waterText := "None"
	if len(water) > 0 {
		waterText = strings.Join(water, ", ")
	}
	near := "None nearby"
	if t, dist := p.Nearest(i, 64, func(t int) bool { return p.Settlement[t] >= 0 }); t >= 0 {
		fname := ""
		if f, _ := p.SettlementFaction(t); f >= 0 && f < len(p.Factions) {
			fname = p.Factions[f].Name
		}
		near = fmt.Sprintf("%s (%s), %d tiles", p.SettlementName(t), fname, dist)
	}
	mv := movementDifficulty(s.Hills, p.Biome[i])
	ti.rows = []infoRow{
		{"Terrain", s.Hills.Label(), hillsTip(s.Hills), uiText},
		{"Coordinates", coordText(p.Lat[i], p.Lon[i]), "", uiText},
		{"Elevation", fmt.Sprintf("%d m", p.Elev[i]), "", uiText},
		{"Avg temperature", fmt.Sprintf("%.1f°C", float64(p.Temp[i])/10), "Average temperature\nThe yearly mean at this tile.", uiText},
		{"Summer / winter", celsius(hi) + " / " + celsius(lo), "The warmest afternoon and the coldest night of the year.", uiText},
		{"Rainfall", fmt.Sprintf("%d mm/year", p.Rain[i]), "", uiText},
		{"Growing period", growText, "Growing period\nDays a year warm enough for crops to grow (daily mean above 3°C).", uiText},
		{"Stone types", strings.Join(stones, ", "), "Stone types\nWhat the local rock is made of. It sets the building stone and ore you can mine.", uiText},
		{"Water", waterText, "Water\nRivers, coast and lakes shape the map and give water to build beside.", uiText},
		{"Caves", yesNo(s.Caves), "Caves\nCave systems under the hills hide ore, and sometimes things worse.", uiText},
		{"Roads", yesNo(s.Road != 0), "", uiText},
		{"Nearest settlement", near, "", uiText},
		{"Movement difficulty", fmt.Sprintf("x%.1f", mv), "Movement difficulty\nHow slow it is to cross this tile. Informational until world travel exists.", uiText},
		{"Forage", forageLevel(p.Biome[i]), "Forage\nWild berries and plants to eat without farming.", uiText},
		{"Wildlife", wildlifeLevel(p.Biome[i]), "Wildlife\nHow many wild animals live here.", uiText},
	}
	if p.Water[i] != planet.Land {
		ti.rows = ti.rows[:2]
	}
	switch p.Biome[i] {
	case biome.IceSheet, biome.SeaIce, biome.ExtremeDesert:
		ti.warnings = append(ti.warnings, "Extreme climate")
	}
	if p.Water[i] == planet.Land && grow < 15 {
		ti.warnings = append(ti.warnings, "Very short growing season")
	}
	if n := mapSizeNote(mapSide, total); n != "" { // the size picker's rule (ram_estimate.go)
		ti.warnings = append(ti.warnings, n)
	}
	return ti
}

// landingDetail is the biome card's text: name, blurb, climate, ground,
// plants and wildlife, wrapped to cols characters.
func landingDetail(b game.BiomeInfo, cols int) []string {
	title := b.Name
	if b.Difficulty != "" {
		title += "  [" + b.Difficulty + "]"
	}
	out := []string{title, ""}
	out = append(out, wrapText(b.Blurb, cols)...)
	out = append(out, "",
		fmt.Sprintf("Temperature  %dC winter low, %dC summer high", b.WinterLow, b.SummerHigh),
		fmt.Sprintf("Growing      about %d days a year", b.GrowingDays),
	)
	out = append(out, wrapLabeled("Ground", b.Ground, cols)...)
	out = append(out, wrapLabeled("Plants", b.Plants, cols)...)
	out = append(out, wrapLabeled("Wildlife", b.Wildlife, cols)...)
	return out
}

// wrapLabeled is "Label      a, b, c" wrapped under its label column.
func wrapLabeled(label string, items []string, cols int) []string {
	const lw = 13
	text := "none"
	if len(items) > 0 {
		text = strings.Join(items, ", ")
	}
	rows := wrapText(text, cols-lw)
	for i := range rows {
		if i == 0 {
			rows[i] = fmt.Sprintf("%-*s%s", lw, label, rows[i])
		} else {
			rows[i] = strings.Repeat(" ", lw) + rows[i]
		}
	}
	return rows
}

// ---- the screen ------------------------------------------------------------------

// landingGlobe is the draft's globe, made when the planet changes.
func (a *App) landingGlobe(ng *newGameState) *globeView {
	if ng.planet == nil {
		return nil
	}
	if ng.globe == nil || ng.globeOf != ng.planet {
		ng.globe, ng.globeOf = newGlobeView(ng.planet, a.art), ng.planet
		ng.globe.Select(ng.tile)
		if ng.tile >= 0 {
			ng.globe.Center(ng.tile)
		}
	}
	return ng.globe
}

// tileReason is why Next is disabled for the draft's tile ("" it isn't).
func (ng *newGameState) tileReason() string {
	if ng.planet == nil {
		return "Generate a world first."
	}
	if ng.tile < 0 {
		if ng.outside {
			return planet.ReasonOutside
		}
		return "Choose a landing site first."
	}
	if ok, reason := ng.planet.ValidLanding(ng.tile); !ok {
		return reason
	}
	return ""
}

func (a *App) landingUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.setupTooSmall(sw, sh) {
		return
	}
	ng := a.draft()
	g := a.landingGlobe(ng)
	if g == nil {
		return
	}
	ng.tile = g.Selected() // a selection made since the last frame (Select random site, tests)
	geo := landRects(sw, sh)
	g.SetArea(float64(geo.globe.X), float64(geo.globe.Y), float64(geo.globe.W), float64(geo.globe.H))

	// The top-right controls.
	a.ngButton("land.mode", geo.mode, "View: "+g.Mode().String(), btnOpts{Tip: "View mode\nColour the globe by biome, temperature, rainfall or elevation."},
		func() { g.SetMode((g.Mode() + 1) % numGlobeModes) })
	a.ngButton("land.legend", geo.legend, "Legend", btnOpts{Selected: ng.legend}, func() { ng.legend = !ng.legend })

	// Bottom bar.
	a.uiPanel(geo.bar)
	a.landBar(ng, g, geo.bar)

	// The info panel.
	a.uiPanel(geo.panel)
	shown := ng.tile
	if shown < 0 && g.Hovered() >= 0 {
		shown = g.Hovered()
	}
	a.landInfoPanel(ng, geo.panel.Inset(landPanelPad), shown, g.Hovered() == globeUncovered && ng.tile < 0)

	// The globe takes the pointer anywhere outside the chrome and modals.
	mx, my := a.currentCursor()
	blocked := a.uiModalOpen() || geo.panel.Hit(mx, my) || geo.bar.Hit(mx, my) || geo.mode.Hit(mx, my) || geo.legend.Hit(mx, my) ||
		(ng.legend && a.legendRect(geo).Hit(mx, my))
	clicked, _ := g.Update(globeInputFrom(&a.in, sw, sh, blocked), a.settings.Accessibility.ReducedMotion)
	switch {
	case clicked >= 0:
		ng.outside = false
	case clicked == globeUncovered:
		ng.outside = true
	}
	ng.tile = g.Selected()
	if ng.legend {
		a.landLegend(g, a.legendRect(geo))
	}
}

// landBar declares the bottom bar's buttons.
func (a *App) landBar(ng *newGameState, g *globeView, bar rect) {
	x := bar.X + 12
	y := bar.Y + (bar.H-footerBtnH)/2
	btn := func(id, label string, w float32, o btnOpts, fn func()) {
		a.ngButton(id, rect{X: x, Y: y, W: w, H: footerBtnH}, label, o, fn)
		x += w + 10
	}
	btn(ids.WizardBack, "Back", 110, btnOpts{}, a.goBack)
	btn("land.random", "Select random site", 190, btnOpts{Tip: "Select random site\nPick a pleasant place at random. Press again for another."}, func() { a.randomLanding(ng, g) })
	btn("land.advanced", "Advanced...", 120, btnOpts{Tip: "Advanced\nStarting season and map size."}, a.pushLandingAdvanced)
	btn("land.factions", "Factions...", 120, btnOpts{Tip: "Factions\nWho lives on this planet."}, func() { a.pushFactions(ng, g) })
	reason := ng.tileReason()
	next := func() {
		if ng.tileReason() == "" {
			a.forward()
		}
	}
	btn(ids.WizardNext, "Next", 130, btnOpts{Style: btnPrimary, Disabled: reason != "", Reason: reason}, next)
	if a.in.Fired(input.Accept) && reason == "" && !a.uiModalOpen() && !a.uiTextEditing() {
		next()
	}
	if reason != "" {
		st := dimText()
		a.uiLabel(bar.X+bar.W/2-textW(reason, st)/2, bar.Y-float32(lineH(st))-6, reason, st)
	}
}

// randomLanding selects the next random valid site and flies there.
func (a *App) randomLanding(ng *newGameState, g *globeView) {
	t := ng.planet.RandomLanding(ng.landRolls)
	ng.landRolls++
	if t < 0 {
		a.uiDeny("No tile on this world can be landed on.")
		return
	}
	ng.tile, ng.outside = t, false
	g.Select(t)
	g.Center(t)
}

// ---- the info panel ---------------------------------------------------------------

// infoBlock is a part of the panel's scrolling content.
type infoBlock struct {
	h    float32
	tip  string
	draw func(dst *ebiten.Image, r rect)
}

func (a *App) landInfoPanel(ng *newGameState, r rect, tile int, uncovered bool) {
	hs := tsLarge
	hs.Color = uiHeading
	a.uiLabel(r.X, r.Y, "Tile info", hs)
	body := rect{X: r.X, Y: r.Y + float32(lineH(hs)) + 8, W: r.W, H: r.H - float32(lineH(hs)) - 8}
	if uncovered || tile == globeUncovered {
		a.uiLabel(body.X, body.Y, planet.ReasonOutside, dimText())
		return
	}
	if tile < 0 {
		a.textLines(body.X, body.Y, body.W, "Click a tile on the globe to see what it is like. Drag to turn the planet, scroll to zoom.", dimText())
		return
	}
	info := a.tileInfoFor(ng, tile)
	blocks := a.infoBlocks(info, body.W-scrollBarW-6)
	total := float32(0)
	for _, b := range blocks {
		total += b.h
	}
	area := a.uiScrollArea("land.info", body, total, scrollOpts{Step: 40})
	y := area.View.Y - area.Off
	for i, b := range blocks {
		br := rect{X: area.View.X, Y: y, W: area.View.W, H: b.h}
		y += b.h
		vis := br.Intersect(area.View)
		if vis.Empty() {
			continue
		}
		if b.tip != "" {
			a.uiTip(fmt.Sprintf("land.info.tip.%d", i), vis, b.tip)
		}
		b := b
		a.uiSt().queue(func(dst *ebiten.Image) { b.draw(clipTo(dst, area.View), br) })
	}
}

// tileInfoFor is the cached info of a tile.
func (a *App) tileInfoFor(ng *newGameState, tile int) tileInfo {
	c := &ng.info
	total := a.hardwareTotal()
	if c.planet != ng.planet || c.tile != tile || c.side != ng.mapSize || c.total != total {
		*c = infoCache{planet: ng.planet, tile: tile, side: ng.mapSize, total: total, info: buildTileInfo(ng.planet, tile, ng.mapSize, total)}
	}
	return c.info
}

// infoCache remembers the last tile info (the nearest-settlement search
// is not free, and the hover changes every frame).
type infoCache struct {
	planet *planet.Planet
	tile   int
	side   int
	total  uint64 // the computer's memory the warning was made for
	info   tileInfo
}

// infoBlocks lays the tile info out for a width.
func (a *App) infoBlocks(ti tileInfo, w float32) []infoBlock {
	var out []infoBlock
	// Header: icon, name, difficulty tag, blurb.
	nameSt := tsLarge
	nameSt.Color = uiHeading
	blurbLines := wrapPx(ti.blurb, tsBody, float64(w))
	hh := float32(56)
	out = append(out, infoBlock{h: hh, tip: ti.biomeTip, draw: func(dst *ebiten.Image, r rect) {
		if img := a.art.Image(biome.DefOf(ti.biome).Icon(), noTint); img != nil && ti.name != "Ocean" && ti.name != "Lake" {
			drawIcon(dst, img, float64(r.X), float64(r.Y+4), 48)
		}
		drawText(dst, ti.name, float64(r.X+58), float64(r.Y+8), nameSt)
		if ti.difficult != "" {
			t := tsSmall
			t.Color = uiWarning
			drawText(dst, "Difficulty: "+ti.difficult, float64(r.X+58), float64(r.Y+8)+lineH(nameSt), t)
		}
	}})
	out = append(out, infoBlock{h: float32(len(blurbLines))*float32(lineH(tsBody)) + 8, draw: func(dst *ebiten.Image, r rect) {
		st := dimText()
		for i, l := range blurbLines {
			drawText(dst, l, float64(r.X), float64(r.Y)+float64(i)*lineH(st), st)
		}
	}})
	if ti.invalid != "" {
		reason := ti.invalid
		out = append(out, infoBlock{h: wrapHeight(reason, tsLabel, w) + 8, draw: func(dst *ebiten.Image, r rect) {
			st := tsLabel
			st.Color = uiDanger
			for i, l := range wrapPx(reason, st, float64(r.W)) {
				drawText(dst, l, float64(r.X), float64(r.Y)+float64(i)*lineH(st), st)
			}
		}})
	}
	for _, wn := range ti.warnings {
		wn := wn
		out = append(out, infoBlock{h: wrapHeight(wn, tsLabel, w) + 4, draw: func(dst *ebiten.Image, r rect) {
			st := tsLabel
			st.Color = uiWarning
			for i, l := range wrapPx(wn, st, float64(r.W)) {
				drawText(dst, l, float64(r.X), float64(r.Y)+float64(i)*lineH(st), st)
			}
		}})
	}
	vw := w - landLabelW
	for _, row := range ti.rows {
		row := row
		lines := wrapPx(row.value, tsBody, float64(vw))
		h := max(float32(len(lines)), 1)*float32(lineH(tsBody)) + 6
		out = append(out, infoBlock{h: h, tip: row.tip, draw: func(dst *ebiten.Image, r rect) {
			ls := dimText()
			drawText(dst, fitText(row.label, ls, landLabelW-8), float64(r.X), float64(r.Y+2), ls)
			vs := tsBody
			vs.Color = row.col
			for i, l := range lines {
				drawText(dst, l, float64(r.X+landLabelW), float64(r.Y+2)+float64(i)*lineH(vs), vs)
			}
			fillRect(dst, rect{X: r.X, Y: r.Bottom() - 1, W: r.W, H: 1}, uiFade(uiPanelBorder, 0x50))
		}})
	}
	return out
}

// ---- legend -----------------------------------------------------------------------------

func (a *App) legendRect(geo landGeo) rect {
	return rect{X: geo.mode.Right() - 190, Y: geo.mode.Bottom() + 8, W: 190, H: 14 + 12*18}
}

// landLegend paints the biome colours (or the data ramp) in r.
func (a *App) landLegend(g *globeView, r rect) {
	a.uiSt().queue(func(dst *ebiten.Image) {
		drawPanel(dst, r.X, r.Y, r.W, r.H)
		st := tsSmall
		if g.Mode() == globeBiome {
			for i, d := range biome.All() {
				y := r.Y + 8 + float32(i)*18
				c := palRGB(d.Key)
				fillRect(dst, rect{X: r.X + 10, Y: y + 2, W: 14, H: 12}, color.RGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), 255})
				drawText(dst, d.Name, float64(r.X+32), float64(y), st)
			}
			return
		}
		var stops []colourStop
		var unit string
		switch g.Mode() {
		case globeTemperature:
			stops, unit = tempRamp, "°C"
		case globeRainfall:
			stops, unit = rainRamp, " mm"
		default:
			stops, unit = elevRamp, " m"
		}
		bar := rect{X: r.X + 12, Y: r.Y + 14, W: r.W - 24, H: 16}
		lo, hi := stops[0].At, stops[len(stops)-1].At
		for x := 0; x < int(bar.W); x++ {
			c := ramp(stops, lo+(hi-lo)*float32(x)/bar.W)
			fillRect(dst, rect{X: bar.X + float32(x), Y: bar.Y, W: 1, H: bar.H}, color.RGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), 255})
		}
		strokeRect(dst, bar, uiPanelBorder)
		ls := st
		ls.Color = uiTextDim
		drawText(dst, fmt.Sprintf("%.0f%s", lo, unit), float64(bar.X), float64(bar.Bottom()+4), ls)
		ls.Align = alignRight
		drawText(dst, fmt.Sprintf("%.0f%s", hi, unit), float64(bar.Right()), float64(bar.Bottom()+4), ls)
	})
}

// ---- drawing -------------------------------------------------------------------------------

func (a *App) drawLanding(screen *ebiten.Image, sw, sh int) {
	ng := a.draft()
	g := a.landingGlobe(ng)
	if g == nil || tooSmall(sw, sh) {
		a.drawSetupBackdrop(screen, sw, sh, ng.planet)
		if tooSmall(sw, sh) {
			drawTooSmall(screen, sw, sh)
		}
		return
	}
	g.Draw(screen, sw, sh)
	geo := landRects(sw, sh)
	st := tsTitle
	st.Align = alignCenter
	cx := float64(geo.title.X + geo.title.W/2)
	st.Color = color.RGBA{0, 0, 0, 200}
	drawText(screen, "Choose your landing site", cx+1.5, float64(geo.title.Y)+1.5, st)
	st.Color = uiHeading
	drawText(screen, "Choose your landing site", cx, float64(geo.title.Y), st)
}

// ---- Advanced ---------------------------------------------------------------------------------

func (a *App) pushLandingAdvanced() {
	ng := a.draft()
	opts := mapSizeOptions()
	h := float32(modalTitleH+modalFooterH+2*modalPad) + 26 + 5*radioRowH + 40 + 26 + float32(len(opts))*radioRowH
	a.uiPushModal(modal{
		id: "land.advanced", title: "Advanced", w: landModalW, h: h,
		draw: func(a *App, body rect) { a.landingAdvancedBody(ng, body, opts) },
		buttons: []modalButton{
			{id: "close", label: "Close", style: btnPrimary, closes: true},
			{id: "reset", label: "Reset", onClick: func() { ng.season, ng.mapSize = -1, defaultMapSideEnabled() }},
		},
	})
}

func (a *App) landingAdvancedBody(ng *newGameState, body rect, opts []game.MapSizeView) {
	hs := tsLabel
	hs.Color = uiHeading
	y := body.Y
	a.uiLabel(body.X, y, "Starting season", hs)
	a.uiTip("land.season.tip", rect{X: body.X, Y: y, W: body.W, H: 22}, "Starting season\nSeason the colony starts in, at this site.")
	y += 26
	sel := ng.season + 1
	if a.uiRadio("land.season", rect{X: body.X, Y: y, W: body.W, H: 5 * radioRowH}, append([]string{"Auto"}, seasonNames...), &sel, radioOpts{
		Tips: []string{"Auto\nThe first season that is already warm (8°C or more) and getting warmer; otherwise the warmest one."},
	}) {
		ng.season = sel - 1
	}
	y += 5*radioRowH + 6
	note := "Pick a landing site to see what Auto means there."
	if ng.planet != nil && ng.tile >= 0 {
		note = "Auto starts this site in " + seasonNames[autoSeason(ng.planet.Site(ng.tile).Climate)] + "."
	}
	a.uiLabel(body.X, y, note, dimText())
	y += 34
	a.uiLabel(body.X, y, "Map size", hs)
	a.uiTip("land.mapsize.tip", rect{X: body.X, Y: y, W: body.W, H: 22}, "Map size\nThe side of the square local map. Bigger maps take longer to build and need a faster computer.")
	y += 26
	labels := make([]string, len(opts))
	dis := make([]string, len(opts))
	tips := make([]string, len(opts))
	notes := make([]string, len(opts))
	total := a.hardwareTotal()
	msel := 0
	for i, o := range opts {
		labels[i], dis[i] = mapSizeLabel(o), o.Reason
		tips[i] = fmt.Sprintf("%d x %d\n%s tiles, about %d tiles across.", o.Side, o.Side, groupDigits(o.Side*o.Side), o.Side)
		if o.Enabled { // a size that is not available yet has nothing to warn about
			if notes[i] = mapSizeNote(o.Side, total); notes[i] != "" {
				tips[i] += "\n" + notes[i]
			}
		}
		if o.Side == ng.mapSize {
			msel = i
		}
	}
	list := rect{X: body.X, Y: y, W: body.W, H: float32(len(opts)) * radioRowH}
	if a.uiRadio("land.mapsize", list, labels, &msel, radioOpts{Disabled: dis, Tips: tips}) {
		ng.mapSize = opts[msel].Side
	}
	a.mapSizeWarnings(list, labels, notes)
}

// mapSizeWarnings paints each size's warning (ram_estimate.go) at the right
// end of its row, in the warning colour, clear of the size label. The size
// stays selectable: this only draws.
func (a *App) mapSizeWarnings(list rect, labels, notes []string) {
	rows := list.RowsN(len(labels), radioRowH, 0)
	a.uiSt().queue(func(dst *ebiten.Image) {
		st := withColor(tsBody, uiWarning)
		st.Align = alignRight
		for i, note := range notes {
			if note == "" {
				continue
			}
			row := rows[i]
			labelEnd := row.X + 2*radioRadius + 1 + checkGap + textW(labels[i], tsBody) + 12
			room := row.Right() - 4 - labelEnd
			if room <= 0 {
				continue
			}
			drawText(dst, fitText(note, st, room), float64(row.Right()-4), float64(row.Y)+(float64(row.H)-lineH(st))/2, st)
		}
	})
}
