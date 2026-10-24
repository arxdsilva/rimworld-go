package ebiten

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/planet"
)

// The World tab (MAIN_UI_SCREENS §33 ENG-003 slice, §35 caravan
// inspector): the colony's planet on the globe of §7 (read-only, rebuilt
// from the world params in a few frames), the colony tile and caravans
// pulsing on it, and the caravan list with a Return (or Call off) button
// each. "Form caravan" opens §34 (caravan_ui.go). It reads WorldMap and
// the consumer-owned worldSource, and emits CaravanReturn.

const (
	worldModalID     = "colony.world"
	worldW, worldH   = 960, 620
	worldListW       = 320
	worldRowH        = 78
	worldBuildUnits  = 6000 // planet builder work per frame
	worldNoMap       = "This colony has no world map."
	worldNoCaravans  = "No caravans. Form one to travel the world."
	worldCaravanIcon = "ui_icon/caravan"
)

// worldSource is what the World tab needs beyond Simulation.WorldMap: the
// params to rebuild the globe and the settlements a caravan may visit
// (consumer-owned; the game implements it).
type worldSource interface {
	WorldParams() (planet.Params, bool)
	WorldSites() []game.WorldSiteView
}

// worldPlanets caches finished planets by params key (a pure function of
// the params; read-only once built), so reopening the tab is instant.
var worldPlanets = struct {
	sync.Mutex
	m map[string]*planet.Planet
}{m: map[string]*planet.Planet{}}

// worldUI is the open World tab's state (held by its modal's closures).
type worldUI struct {
	params  planet.Params
	hasMap  bool
	builder *planet.Builder
	planet  *planet.Planet
	globe   *globeView
	// picked is the tile last clicked on the globe (-1 none): the Form
	// Caravan dialog offers it as the destination.
	picked int
	sel    uint64 // selected caravan
}

// worldTabUI makes the HUD's World tab open the window (called from the
// colony's widget hook).
func (a *App) worldTabUI(sw, sh int) {
	for _, b := range a.hudTabButtons(sw, sh) {
		if b.id != "tab.world" || b.disabled {
			continue
		}
		a.uiSt().add(uiWidget{id: b.id, label: b.label, r: rect{b.x, b.y, b.w, b.h}, click: a.toggleWorld})
	}
}

// toggleWorld opens the World tab, or closes it when it is on top.
func (a *App) toggleWorld() {
	if a.uiModalTop() == worldModalID {
		a.uiCloseTop()
		return
	}
	a.openWorld()
}

// openWorld opens the World tab.
func (a *App) openWorld() {
	if a.renderer == nil {
		return
	}
	st := &worldUI{picked: -1}
	if src, ok := a.renderer.game.(worldSource); ok {
		st.params, st.hasMap = src.WorldParams()
	}
	a.uiPushModal(modal{
		id: worldModalID, title: "World", w: worldW, h: worldH,
		draw: func(a *App, body rect) { a.worldBody(st, body) },
		buttons: []modalButton{
			{id: "close", label: "Close", style: btnPrimary, closes: true},
			{id: "world.form", label: "Form caravan", disabled: func() string {
				if !st.hasMap {
					return worldNoMap
				}
				return ""
			}, onClick: func() { a.openFormCaravan(st) }},
		},
	})
}

// worldPlanetFor is the finished planet for st (building it a slice per
// frame), nil while it is being charted.
func (a *App) worldPlanetFor(st *worldUI) *planet.Planet {
	if st.planet != nil || !st.hasMap {
		return st.planet
	}
	key := st.params.Key()
	worldPlanets.Lock()
	p := worldPlanets.m[key]
	worldPlanets.Unlock()
	if p == nil && a.ng != nil && a.ng.planet != nil && a.ng.planet.Params.Key() == key {
		p = a.ng.planet // the planet the colony was founded on
	}
	if p == nil {
		if st.builder == nil {
			st.builder = planet.NewBuilder(st.params)
		}
		if _, _, done := st.builder.Step(worldBuildUnits); !done {
			return nil
		}
		p = st.builder.Planet()
		st.builder = nil
		worldPlanets.Lock()
		worldPlanets.m[key] = p
		worldPlanets.Unlock()
	}
	st.planet = p
	return p
}

// worldBody declares the globe and the caravan list.
func (a *App) worldBody(st *worldUI, body rect) {
	if a.renderer == nil {
		return
	}
	view := a.renderer.game.WorldMap()
	globeR, list := body.splitH(body.W - worldListW)
	globeR.W -= 12
	a.worldGlobe(st, globeR, view)
	a.worldList(st, list, view)
}

// worldGlobe declares the globe area: the globe once charted, else the
// progress or the no-map note.
func (a *App) worldGlobe(st *worldUI, r rect, view game.WorldMapView) {
	if !st.hasMap {
		a.uiLabel(r.X, r.Y, worldNoMap, dimText())
		return
	}
	p := a.worldPlanetFor(st)
	if p == nil {
		pct := 0
		if st.builder != nil {
			_, f, _ := st.builder.Step(0)
			pct = int(f * 100)
		}
		a.uiLabel(r.X, r.Y, fmt.Sprintf("Charting the world… %d%%", pct), dimText())
		return
	}
	if st.globe == nil {
		st.globe = newGlobeView(p, a.art)
		st.globe.Select(view.Tile)
		st.globe.Center(view.Tile)
	}
	g := st.globe
	g.SetArea(float64(r.X), float64(r.Y), float64(r.W), float64(r.H))
	tiles := []int{}
	if view.Tile >= 0 {
		tiles = append(tiles, view.Tile)
	}
	for _, c := range view.Caravans {
		tiles = append(tiles, c.Tile)
	}
	g.Highlight(tiles, 2)
	mx, my := a.currentCursor()
	blocked := a.uiModalTop() != worldModalID || !r.hit(mx, my)
	if clicked, _ := g.Update(globeInputFrom(&a.in, a.sw, a.sh, blocked), a.settings.Accessibility.ReducedMotion); clicked >= 0 {
		st.picked = clicked
	}
	sw, sh := a.sw, a.sh
	art := a.art
	caravans := view.Caravans
	a.uiSt().queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, r)
		fillRect(sub, r, uiSpace)
		g.sw, g.sh = sw, sh
		g.ensureGPU(sw, sh)
		g.drawBody(sub)
		drawCaravanMarks(sub, g, art, caravans)
	})
	line := "Colony tile marked. Click a tile to pick it as a caravan destination."
	if st.picked >= 0 {
		name := p.SettlementName(st.picked)
		if name == "" {
			name = fmt.Sprintf("tile %d", st.picked)
		}
		line = "Picked: " + name
	}
	a.uiLabel(r.X+6, r.bottom()-float32(lineH(tsSmall))-4, line, withColor(tsSmall, uiTextDim))
}

// drawCaravanMarks draws each caravan's marker on the globe's facing side.
func drawCaravanMarks(dst *ebiten.Image, g *globeView, art *artCatalog, cs []game.CaravanView) {
	size := globeClamp(g.tilePx()*2, 14, 40)
	for _, c := range cs {
		if c.Tile < 0 || c.Tile >= g.p.TileCount() {
			continue
		}
		sx, sy, vz := g.worldPt(&g.p.Center[c.Tile])
		if vz < g.zh+0.04 {
			continue
		}
		vector.FillCircle(dst, sx, sy, float32(size*0.56), color.RGBA{10, 14, 20, 170}, true)
		var img *ebiten.Image
		if art != nil {
			img = art.image(worldCaravanIcon, color.NRGBA{})
		}
		if img == nil {
			vector.FillCircle(dst, sx, sy, float32(size*0.3), uiHeading, true)
			continue
		}
		b := img.Bounds()
		s := size / float64(b.Dx())
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(sx)-size/2, float64(sy)-float64(b.Dy())*s/2)
		dst.DrawImage(img, op)
	}
}

// caravanTitle is a caravan's list heading ("Caravan: Ada, Bo").
func caravanTitle(c game.CaravanView) string {
	if len(c.Members) == 0 {
		return "Caravan"
	}
	return "Caravan: " + strings.Join(c.Members, ", ")
}

// caravanReturnLabel is the action a caravan's button offers ("" none).
func caravanReturnLabel(c game.CaravanView) (label, reason string) {
	switch c.State {
	case "Forming":
		return "Call off", ""
	case "Returning home":
		return "Return", "Already on the way home."
	}
	return "Return", ""
}

// worldList declares the caravan list (the §35 inspector rows).
func (a *App) worldList(st *worldUI, r rect, view game.WorldMapView) {
	head, rest := r.splitV(28)
	a.uiLabel(head.X, head.Y, "Caravans", withColor(tsLabel, uiHeading))
	if len(view.Caravans) == 0 {
		a.uiLabel(rest.X, rest.Y+4, worldNoCaravans, dimText())
		return
	}
	area := a.uiScrollArea("world.caravans", rest, float32(len(view.Caravans)*worldRowH), scrollOpts{Step: worldRowH})
	y := area.View.Y - area.Off
	for _, c := range view.Caravans {
		row := rect{area.View.X, y, area.View.W - 4, worldRowH - 6}
		y += worldRowH
		if row.Y < area.View.Y || row.bottom() > area.View.bottom() {
			continue
		}
		id := fmt.Sprintf("world.caravan.%d", c.ID)
		pick := rect{row.X, row.Y, row.W - 116, row.H} // the Return button sits beside it
		if a.uiButton(id, pick, "", btnOpts{Selected: st.sel == c.ID, Tip: caravanTitle(c)}) {
			st.sel = c.ID
			if st.globe != nil {
				st.globe.Center(c.Tile)
			}
		}
		a.uiLabel(row.X+8, row.Y+4, ellipsize(caravanTitle(c), tsBody, float64(row.W-16)), tsBody)
		where := fmt.Sprintf("%s · tile %d → %d", c.State, c.Tile, c.Dest)
		if c.State == "Travelling" || c.State == "Returning home" {
			where += fmt.Sprintf(" · %d%% to the next tile", c.Progress)
		}
		a.uiLabel(row.X+8, row.Y+26, ellipsize(where, tsSmall, float64(row.W-16)), withColor(tsSmall, uiTextDim))
		label, reason := caravanReturnLabel(c)
		br := rect{row.right() - 108, row.bottom() - 30, 100, 26}
		if a.uiButton(id+".return", br, label, btnOpts{Disabled: reason != "", Reason: reason,
			Tip: "Return\nThe caravan turns round and comes home the way it went."}) {
			a.renderer.enqueue(command.Command{Kind: command.CaravanReturn, Target: c.ID})
		}
	}
}
