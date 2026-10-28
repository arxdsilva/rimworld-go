package wizard

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	Globe, Panel, bar, title gfx.Rect
	mode, legend             gfx.Rect
}

func LandRects(sw, sh int) landGeo {
	w, h := float32(sw), float32(sh)
	g := landGeo{}
	g.Panel = gfx.Rect{X: w - landPanelW - landPanelPad, Y: landPanelPad, W: landPanelW, H: h - landPanelPad - landBarH - 2*landPanelPad}
	g.bar = gfx.Rect{X: (w - 740) / 2, Y: h - landBarH - landPanelPad, W: 740, H: landBarH}
	g.Globe = gfx.Rect{X: 0, Y: landTitleH, W: g.Panel.X, H: g.bar.Y - landTitleH}
	g.title = gfx.Rect{X: 0, Y: 12, W: g.Panel.X, H: 32}
	g.mode = gfx.Rect{X: g.Panel.X - 12 - 170, Y: 14, W: 170, H: 30}
	g.legend = gfx.Rect{X: g.mode.X - 8 - 90, Y: 14, W: 90, H: 30}
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

// infoRow is a label/value line of the tile panel.
type infoRow struct {
	Label, Value, tip string
	col               color.RGBA
}

// TileInfo is everything the panel shows for one tile.
type TileInfo struct {
	tile      int
	biome     biome.ID
	Name      string
	Blurb     string
	difficult string
	BiomeTip  string
	Rows      []infoRow
	Warnings  []string
	Invalid   string // why the tile can't be landed on ("" valid)
	Outside   bool
}

func CoordText(lat, lon int16) string {
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

// MovementDifficulty is the informational travel multiplier of a tile
// (x1.0 to x3.0) from its hills and biome.
func MovementDifficulty(h site.Hills, b biome.ID) float64 {
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

// ForageLevel is the wild food on offer, from the biome's berry bushes.
func ForageLevel(b biome.ID) string {
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

// WildlifeLevel is the animal density label of a biome.
func WildlifeLevel(b biome.ID) string {
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

// BuildTileInfo reads tile i of p for the panel; sizeNote is the memory
// warning for the chosen map size on this computer ("" none,
// Host.MapSizeNote).
func BuildTileInfo(p *planet.Planet, i int, sizeNote string) TileInfo {
	if i < 0 || i >= p.TileCount() {
		return TileInfo{tile: i, Outside: true}
	}
	s := p.Site(i)
	d := biome.DefOf(p.Biome[i])
	ti := TileInfo{tile: i, biome: p.Biome[i], Name: d.Name, Blurb: d.Blurb, difficult: d.Difficulty}
	if p.Water[i] == planet.Ocean {
		ti.Name, ti.Blurb, ti.difficult = "Ocean", "Open sea. Nobody can land here.", ""
	} else if p.Water[i] == planet.Lake {
		ti.Name, ti.Blurb, ti.difficult = "Lake", "Fresh water. Nobody can land here.", ""
	}
	ti.BiomeTip = d.Name + "\n" + strings.Join(LandingDetail(game.BiomeInfoOf(p.Biome[i]), 60)[2:], "\n")
	if ok, reason := p.ValidLanding(i); !ok {
		ti.Invalid = reason
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
	mv := MovementDifficulty(s.Hills, p.Biome[i])
	ti.Rows = []infoRow{
		{"Terrain", s.Hills.Label(), hillsTip(s.Hills), uikit.Text},
		{"Coordinates", CoordText(p.Lat[i], p.Lon[i]), "", uikit.Text},
		{"Elevation", fmt.Sprintf("%d m", p.Elev[i]), "", uikit.Text},
		{"Avg temperature", fmt.Sprintf("%.1f°C", float64(p.Temp[i])/10), "Average temperature\nThe yearly mean at this tile.", uikit.Text},
		{"Summer / winter", celsius(hi) + " / " + celsius(lo), "The warmest afternoon and the coldest night of the year.", uikit.Text},
		{"Rainfall", fmt.Sprintf("%d mm/year", p.Rain[i]), "", uikit.Text},
		{"Growing period", growText, "Growing period\nDays a year warm enough for crops to grow (daily mean above 3°C).", uikit.Text},
		{"Stone types", strings.Join(stones, ", "), "Stone types\nWhat the local rock is made of. It sets the building stone and ore you can mine.", uikit.Text},
		{"Water", waterText, "Water\nRivers, coast and lakes shape the map and give water to build beside.", uikit.Text},
		{"Caves", yesNo(s.Caves), "Caves\nCave systems under the hills hide ore, and sometimes things worse.", uikit.Text},
		{"Roads", yesNo(s.Road != 0), "", uikit.Text},
		{"Nearest settlement", near, "", uikit.Text},
		{"Movement difficulty", fmt.Sprintf("x%.1f", mv), "Movement difficulty\nHow slow it is to cross this tile. Informational until world travel exists.", uikit.Text},
		{"Forage", ForageLevel(p.Biome[i]), "Forage\nWild berries and plants to eat without farming.", uikit.Text},
		{"Wildlife", WildlifeLevel(p.Biome[i]), "Wildlife\nHow many wild animals live here.", uikit.Text},
	}
	if p.Water[i] != planet.Land {
		ti.Rows = ti.Rows[:2]
	}
	switch p.Biome[i] {
	case biome.IceSheet, biome.SeaIce, biome.ExtremeDesert:
		ti.Warnings = append(ti.Warnings, "Extreme climate")
	}
	if p.Water[i] == planet.Land && grow < 15 {
		ti.Warnings = append(ti.Warnings, "Very short growing season")
	}
	if sizeNote != "" { // the size picker's rule (Host.MapSizeNote)
		ti.Warnings = append(ti.Warnings, sizeNote)
	}
	return ti
}

// LandingDetail is the biome card's text: name, blurb, climate, ground,
// plants and wildlife, wrapped to cols characters.
func LandingDetail(b game.BiomeInfo, cols int) []string {
	title := b.Name
	if b.Difficulty != "" {
		title += "  [" + b.Difficulty + "]"
	}
	out := []string{title, ""}
	out = append(out, uikit.WrapCols(b.Blurb, cols)...)
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
	rows := uikit.WrapCols(text, cols-lw)
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
func (w landingPage) landingGlobe(ng *Draft) *globe.View {
	if ng.Planet == nil {
		return nil
	}
	if ng.Globe == nil || ng.globeOf != ng.Planet {
		ng.Globe, ng.globeOf = globe.New(ng.Planet, w.art()), ng.Planet
		ng.Globe.Select(ng.Tile)
		if ng.Tile >= 0 {
			ng.Globe.Center(ng.Tile)
		}
	}
	return ng.Globe
}

// tileReason is why Next is disabled for the draft's tile ("" it isn't).
func (ng *Draft) tileReason() string {
	if ng.Planet == nil {
		return "Generate a world first."
	}
	if ng.Tile < 0 {
		if ng.Outside {
			return planet.ReasonOutside
		}
		return "Choose a landing site first."
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
	uikit.StyledButton(w.kit(), "land.mode", geo.mode, "View: "+g.Mode().String(), uikit.BtnOpts{Tip: "View mode\nColour the globe by biome, temperature, rainfall or elevation."},
		func() { g.SetMode((g.Mode() + 1) % globe.NumModes) })
	uikit.StyledButton(w.kit(), "land.legend", geo.legend, "Legend", uikit.BtnOpts{Selected: ng.Legend}, func() { ng.Legend = !ng.Legend })

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
	blocked := w.kit().ModalOpen() || geo.Panel.Hit(mx, my) || geo.bar.Hit(mx, my) || geo.mode.Hit(mx, my) || geo.legend.Hit(mx, my) ||
		(ng.Legend && w.legendRect(geo).Hit(mx, my))
	clicked, _ := g.Update(globeInputFrom(w.in(), sw, sh, blocked), w.sh.Settings().Accessibility.ReducedMotion)
	switch {
	case clicked >= 0:
		ng.Outside = false
	case clicked == globe.ModeUncovered:
		ng.Outside = true
	}
	ng.Tile = g.Selected()
	if ng.Legend {
		w.landLegend(g, w.legendRect(geo))
	}
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
	btn("land.random", "Select random site", 190, uikit.BtnOpts{Tip: "Select random site\nPick a pleasant place at random. Press again for another."}, func() { w.randomLanding(ng, g) })
	btn("land.advanced", "Advanced...", 120, uikit.BtnOpts{Tip: "Advanced\nStarting season and map size."}, w.pushLandingAdvanced)
	btn("land.factions", "Factions...", 120, uikit.BtnOpts{Tip: "Factions\nWho lives on this planet."}, func() { w.pushFactions(ng, g) })
	reason := ng.tileReason()
	next := func() {
		if ng.tileReason() == "" {
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
	t := ng.Planet.RandomLanding(ng.LandRolls)
	ng.LandRolls++
	if t < 0 {
		w.kit().Deny("No tile on this world can be landed on.")
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
	w.kit().Label(r.X, r.Y, "Tile info", hs)
	body := gfx.Rect{X: r.X, Y: r.Y + float32(gfx.LineH(hs)) + 8, W: r.W, H: r.H - float32(gfx.LineH(hs)) - 8}
	if uncovered || tile == globe.ModeUncovered {
		w.kit().Label(body.X, body.Y, planet.ReasonOutside, uikit.DimText())
		return
	}
	if tile < 0 {
		uikit.TextLines(w.kit(), body.X, body.Y, body.W, "Click a tile on the globe to see what it is like. Drag to turn the planet, scroll to zoom.", uikit.DimText())
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
func (w landingPage) tileInfoFor(ng *Draft, tile int) TileInfo {
	c := &ng.Info
	note := w.sh.MapSizeNote(ng.MapSize)
	if c.Planet != ng.Planet || c.Tile != tile || c.side != ng.MapSize || c.note != note {
		*c = infoCache{Planet: ng.Planet, Tile: tile, side: ng.MapSize, note: note, info: BuildTileInfo(ng.Planet, tile, note)}
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
	info   TileInfo
}

// infoBlocks lays the tile info out for a width.
func (w landingPage) infoBlocks(ti TileInfo, width float32) []infoBlock {
	var out []infoBlock
	// Header: icon, name, difficulty tag, blurb.
	nameSt := gfx.Large
	nameSt.Color = uikit.Heading
	blurbLines := gfx.WrapPx(ti.Blurb, gfx.Body, float64(width))
	hh := float32(56)
	out = append(out, infoBlock{h: hh, tip: ti.BiomeTip, draw: func(dst *ebiten.Image, r gfx.Rect) {
		if img := w.art().Image(biome.DefOf(ti.biome).Icon(), noTint); img != nil && ti.Name != "Ocean" && ti.Name != "Lake" {
			gfx.Icon(dst, img, float64(r.X), float64(r.Y+4), 48)
		}
		gfx.Text(dst, ti.Name, float64(r.X+58), float64(r.Y+8), nameSt)
		if ti.difficult != "" {
			t := gfx.Small
			t.Color = uikit.Warning
			gfx.Text(dst, "Difficulty: "+ti.difficult, float64(r.X+58), float64(r.Y+8)+gfx.LineH(nameSt), t)
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
	vw := width - landLabelW
	for _, row := range ti.Rows {
		row := row
		lines := gfx.WrapPx(row.Value, gfx.Body, float64(vw))
		h := max(float32(len(lines)), 1)*float32(gfx.LineH(gfx.Body)) + 6
		out = append(out, infoBlock{h: h, tip: row.tip, draw: func(dst *ebiten.Image, r gfx.Rect) {
			ls := uikit.DimText()
			gfx.Text(dst, uikit.FitText(row.Label, ls, landLabelW-8), float64(r.X), float64(r.Y+2), ls)
			vs := gfx.Body
			vs.Color = row.col
			for i, l := range lines {
				gfx.Text(dst, l, float64(r.X+landLabelW), float64(r.Y+2)+float64(i)*gfx.LineH(vs), vs)
			}
			uikit.FillRect(dst, gfx.Rect{X: r.X, Y: r.Bottom() - 1, W: r.W, H: 1}, uikit.Fade(uikit.PanelBorder, 0x50))
		}})
	}
	return out
}

// ---- legend -----------------------------------------------------------------------------

func (w landingPage) legendRect(geo landGeo) gfx.Rect {
	return gfx.Rect{X: geo.mode.Right() - 190, Y: geo.mode.Bottom() + 8, W: 190, H: 14 + 12*18}
}

// landLegend paints the biome colours (or the data ramp) in r.
func (w landingPage) landLegend(g *globe.View, r gfx.Rect) {
	w.kit().Queue(func(dst *ebiten.Image) {
		uikit.DrawPanel(dst, r.X, r.Y, r.W, r.H)
		st := gfx.Small
		if g.Mode() == globe.ModeBiome {
			for i, d := range biome.All() {
				y := r.Y + 8 + float32(i)*18
				c := globe.PalRGB(d.Key)
				uikit.FillRect(dst, gfx.Rect{X: r.X + 10, Y: y + 2, W: 14, H: 12}, color.RGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), 255})
				gfx.Text(dst, d.Name, float64(r.X+32), float64(y), st)
			}
			return
		}
		var stops []globe.ColourStop
		var unit string
		switch g.Mode() {
		case globe.ModeTemperature:
			stops, unit = globe.TempRamp, "°C"
		case globe.ModeRainfall:
			stops, unit = globe.RainRamp, " mm"
		default:
			stops, unit = globe.ElevRamp, " m"
		}
		bar := gfx.Rect{X: r.X + 12, Y: r.Y + 14, W: r.W - 24, H: 16}
		lo, hi := stops[0].At, stops[len(stops)-1].At
		for x := 0; x < int(bar.W); x++ {
			c := globe.Ramp(stops, lo+(hi-lo)*float32(x)/bar.W)
			uikit.FillRect(dst, gfx.Rect{X: bar.X + float32(x), Y: bar.Y, W: 1, H: bar.H}, color.RGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), 255})
		}
		uikit.StrokeRect(dst, bar, uikit.PanelBorder)
		ls := st
		ls.Color = uikit.TextDim
		gfx.Text(dst, fmt.Sprintf("%.0f%s", lo, unit), float64(bar.X), float64(bar.Bottom()+4), ls)
		ls.Align = gfx.AlignRight
		gfx.Text(dst, fmt.Sprintf("%.0f%s", hi, unit), float64(bar.Right()), float64(bar.Bottom()+4), ls)
	})
}

// ---- drawing -------------------------------------------------------------------------------

func (w landingPage) drawLanding(screen *ebiten.Image, sw, sh int) {
	ng := w.draft()
	g := w.landingGlobe(ng)
	if g == nil || uikit.TooSmall(sw, sh) {
		w.DrawBackdrop(screen, sw, sh, ng.Planet)
		if uikit.TooSmall(sw, sh) {
			uikit.DrawTooSmall(screen, sw, sh)
		}
		return
	}
	g.Draw(screen, sw, sh)
	geo := LandRects(sw, sh)
	st := gfx.Title
	st.Align = gfx.AlignCenter
	cx := float64(geo.title.X + geo.title.W/2)
	st.Color = color.RGBA{0, 0, 0, 200}
	gfx.Text(screen, "Choose your landing site", cx+1.5, float64(geo.title.Y)+1.5, st)
	st.Color = uikit.Heading
	gfx.Text(screen, "Choose your landing site", cx, float64(geo.title.Y), st)
}

// ---- Advanced ---------------------------------------------------------------------------------

func (w landingPage) pushLandingAdvanced() {
	ng := w.draft()
	opts := MapSizeOptions()
	h := float32(uikit.ModalTitleH+uikit.ModalFooterH+2*uikit.ModalPad) + 26 + 5*uikit.RadioRowH + 40 + 26 + float32(len(opts))*uikit.RadioRowH
	w.kit().Push(uikit.Modal{
		ID: "land.advanced", Title: "Advanced", W: landModalW, H: h,
		Draw: func(body gfx.Rect) { w.landingAdvancedBody(ng, body, opts) },
		Buttons: []uikit.ModalButton{
			{ID: "close", Label: "Close", Style: uikit.BtnPrimary, Closes: true},
			{ID: "reset", Label: "Reset", OnClick: func() { ng.Season, ng.MapSize = -1, DefaultMapSideEnabled() }},
		},
	})
}

func (w landingPage) landingAdvancedBody(ng *Draft, body gfx.Rect, opts []game.MapSizeView) {
	hs := gfx.Label
	hs.Color = uikit.Heading
	y := body.Y
	w.kit().Label(body.X, y, "Starting season", hs)
	w.kit().Tip("land.season.tip", gfx.Rect{X: body.X, Y: y, W: body.W, H: 22}, "Starting season\nSeason the colony starts in, at this site.")
	y += 26
	sel := ng.Season + 1
	if w.kit().Radio("land.season", gfx.Rect{X: body.X, Y: y, W: body.W, H: 5 * uikit.RadioRowH}, append([]string{"Auto"}, seasonNames...), &sel, uikit.RadioOpts{
		Tips: []string{"Auto\nThe first season that is already warm (8°C or more) and getting warmer; otherwise the warmest one."},
	}) {
		ng.Season = sel - 1
	}
	y += 5*uikit.RadioRowH + 6
	note := "Pick a landing site to see what Auto means there."
	if ng.Planet != nil && ng.Tile >= 0 {
		note = "Auto starts this site in " + seasonNames[AutoSeason(ng.Planet.Site(ng.Tile).Climate)] + "."
	}
	w.kit().Label(body.X, y, note, uikit.DimText())
	y += 34
	w.kit().Label(body.X, y, "Map size", hs)
	w.kit().Tip("land.mapsize.tip", gfx.Rect{X: body.X, Y: y, W: body.W, H: 22}, "Map size\nThe side of the square local map. Bigger maps take longer to build and need a faster computer.")
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
			gfx.Text(dst, uikit.FitText(note, st, room), float64(row.Right()-4), float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
		}
	})
}
