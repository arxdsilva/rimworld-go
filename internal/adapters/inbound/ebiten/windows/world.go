package windows

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/planet"
)

// The World tab (MAIN_UI_SCREENS §33 ENG-003 slice, §35 caravan
// inspector): the colony's planet on the globe of §7 (read-only, rebuilt
// from the world params in a few frames), the colony tile and caravans
// pulsing on it, and the caravan list with a Return (or Call off) button
// each. "Form caravan" opens §34 (the dialogs feature). It reads WorldMap and
// the consumer-owned WorldSource, and emits CaravanReturn.

const (
	WorldModalID     = "colony.world"
	worldW, worldH   = 960, 620
	worldListW       = 320
	worldRowH        = 78
	worldBuildUnits  = 6000 // planet builder work per frame
	worldNoMap       = "This colony has no world map."
	worldNoCaravans  = "No caravans. Form one to travel the world."
	worldCaravanIcon = "ui_icon/caravan"
)

// WorldSource is what the World tab needs beyond Simulation.WorldMap: the
// params to rebuild the globe and the settlements a caravan may visit
// (consumer-owned; the game implements it).
type WorldSource interface {
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
	globe   *globe.View
	// picked is the tile last clicked on the globe (-1 none): the Form
	// Caravan dialog offers it as the destination.
	picked int
	sel    uint64 // selected caravan
}

// Toggle opens the World tab, or closes it when it is the open one.
func (w World) Toggle() { w.toggleKit(WorldModalID, w.Launch) }

// Launch opens the World tab: a main-tab window in the Window rect
// (window.go), with Form caravan in its footer.
func (w World) Launch() {
	if !w.live() {
		return
	}
	st := &worldUI{picked: -1}
	if src, ok := w.sh.Env().Game.(WorldSource); ok {
		st.params, st.hasMap = src.WorldParams()
	}
	w.OpenKit(uikit.Modal{
		ID: WorldModalID, Title: "World", W: worldW, H: worldH, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.body(st, body) },
		Buttons: []uikit.ModalButton{
			{ID: "world.form", Label: "Form caravan", Style: uikit.BtnPrimary, Disabled: func() string {
				if !st.hasMap {
					return worldNoMap
				}
				return ""
			}, OnClick: func() { w.sh.FormCaravan(func() int { return st.picked }) }},
		},
	})
}

// planetFor is the finished planet for st (building it a slice per
// frame), nil while it is being charted.
func (w World) planetFor(st *worldUI) *planet.Planet {
	if st.planet != nil || !st.hasMap {
		return st.planet
	}
	key := st.params.Key()
	worldPlanets.Lock()
	p := worldPlanets.m[key]
	worldPlanets.Unlock()
	if f := w.sh.Env().Planet; p == nil && f != nil && f.Params.Key() == key {
		p = f // the planet the colony was founded on
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

// body declares the globe and the caravan list.
func (w World) body(st *worldUI, body gfx.Rect) {
	if !w.live() {
		return
	}
	view := w.sh.Env().Game.WorldMap()
	globeR, list := body.SplitH(body.W - worldListW)
	globeR.W -= 12
	w.globe(st, globeR, view)
	w.list(st, list, view)
}

// globe declares the globe area: the globe once charted, else the
// progress or the no-map note.
func (w World) globe(st *worldUI, r gfx.Rect, view game.WorldMapView) {
	if !st.hasMap {
		w.kit().Label(r.X, r.Y, worldNoMap, uikit.DimText())
		return
	}
	p := w.planetFor(st)
	if p == nil {
		pct := 0
		if st.builder != nil {
			_, f, _ := st.builder.Step(0)
			pct = int(f * 100)
		}
		w.kit().Label(r.X, r.Y, fmt.Sprintf("Charting the world… %d%%", pct), uikit.DimText())
		return
	}
	if st.globe == nil {
		st.globe = globe.New(p, w.sh.Env().Art)
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
	mx, my := w.sh.Cursor()
	blocked := w.kit().ModalOpen() || w.KitID() != WorldModalID || !r.Hit(mx, my) // a dialog over the tab takes the pointer
	sw, sh := w.sh.Size()
	if clicked, _ := g.Update(globeInput(w.kit().Host().Frame(), sw, sh, blocked), w.kit().Host().ReducedMotion()); clicked >= 0 {
		st.picked = clicked
	}
	art := w.sh.Env().Art
	caravans := view.Caravans
	w.kit().Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, r)
		uikit.FillRect(sub, r, uikit.Space)
		g.Sw, g.Sh = sw, sh
		g.EnsureGPU(sw, sh)
		g.DrawBody(sub)
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
	w.kit().Label(r.X+6, r.Bottom()-float32(gfx.LineH(gfx.Small))-4, line, gfx.WithColor(gfx.Small, uikit.TextDim))
}

// globeInput adapts the frame: w x h is the logical screen, the pointer
// the raw (physical) one the globe picks and drags with. blocked is
// whether the pointer is over screen chrome.
func globeInput(in *inputframe.Frame, w, h int, blocked bool) globe.Input {
	return globe.Input{
		W: w, H: h, MX: in.Raw.Mx, MY: in.Raw.My, Wheel: in.Raw.WheelY,
		Down: in.Raw.Down, Press: in.Raw.Press, Release: in.Raw.Release,
		Held: in.Raw.Held, Keys: in.Raw.Pressed, Shift: in.Shift(), Blocked: blocked,
	}
}

// drawCaravanMarks draws each caravan's marker on the globe's facing side.
func drawCaravanMarks(dst *ebiten.Image, g *globe.View, art *art.Catalog, cs []game.CaravanView) {
	size := globe.Clamp(g.TilePx()*2, 14, 40)
	for _, c := range cs {
		if c.Tile < 0 || c.Tile >= g.P.TileCount() {
			continue
		}
		sx, sy, vz := g.WorldPt(&g.P.Center[c.Tile])
		if vz < g.Zh+0.04 {
			continue
		}
		vector.FillCircle(dst, sx, sy, float32(size*0.56), color.RGBA{R: uikit.Space.R, G: uikit.Space.G, B: uikit.Space.B, A: 170}, true)
		var img *ebiten.Image
		if art != nil {
			img = art.Image(worldCaravanIcon, color.NRGBA{})
		}
		if img == nil {
			vector.FillCircle(dst, sx, sy, float32(size*0.3), uikit.Heading, true)
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

// CaravanReturnLabel is the action a caravan's button offers ("" none).
func CaravanReturnLabel(c game.CaravanView) (label, reason string) {
	switch c.State {
	case "Forming":
		return "Call off", ""
	case "Returning home":
		return "Return", "Already on the way home."
	}
	return "Return", ""
}

// list declares the caravan list (the §35 inspector rows).
func (w World) list(st *worldUI, r gfx.Rect, view game.WorldMapView) {
	head, rest := r.SplitV(28)
	w.kit().Label(head.X, head.Y, "Caravans", gfx.WithColor(gfx.Label, uikit.Heading))
	if len(view.Caravans) == 0 {
		w.kit().Label(rest.X, rest.Y+4, worldNoCaravans, uikit.DimText())
		return
	}
	area := w.kit().ScrollArea("world.caravans", rest, float32(len(view.Caravans)*worldRowH), uikit.ScrollOpts{Step: worldRowH})
	y := area.View.Y - area.Off
	for _, c := range view.Caravans {
		row := gfx.Rect{X: area.View.X, Y: y, W: area.View.W - 4, H: worldRowH - 6}
		y += worldRowH
		if row.Y < area.View.Y || row.Bottom() > area.View.Bottom() {
			continue
		}
		id := fmt.Sprintf("world.caravan.%d", c.ID)
		pick := gfx.Rect{X: row.X, Y: row.Y, W: row.W - 116, H: row.H} // the Return button sits beside it
		tip, note := w.sh.Caravan(c)
		// the hover card lists every member's health (the caravan dialog's)
		if w.kit().Button(id, pick, "", uikit.BtnOpts{Selected: st.sel == c.ID, Tip: tip}) {
			st.sel = c.ID
			if st.globe != nil {
				st.globe.Center(c.Tile)
			}
		}
		w.kit().Label(row.X+8, row.Y+4, gfx.Ellipsize(caravanTitle(c), gfx.Body, float64(row.W-16)), gfx.Body)
		where := fmt.Sprintf("%s%s · tile %d → %d", c.State, note, c.Tile, c.Dest) // WS-M: sick and hurt members first
		if c.State == "Travelling" || c.State == "Returning home" {
			where += fmt.Sprintf(" · %d%% to the next tile", c.Progress)
		}
		w.kit().Label(row.X+8, row.Y+26, gfx.Ellipsize(where, gfx.Small, float64(row.W-16)), gfx.WithColor(gfx.Small, uikit.TextDim))
		label, reason := CaravanReturnLabel(c)
		br := gfx.Rect{X: row.Right() - 108, Y: row.Bottom() - 30, W: 100, H: 26}
		if w.kit().Button(id+".return", br, label, uikit.BtnOpts{Disabled: reason != "", Reason: reason,
			Tip: "Return\nThe caravan turns round and comes home the way it went."}) {
			w.sh.Enqueue(command.Command{Kind: command.CaravanReturn, Target: c.ID})
		}
	}
}
