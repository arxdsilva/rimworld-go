package worldview

import (
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/adapters/inbound/ebiten/worldinfo"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/platform/input"
)

// The view's own chrome over the globe: the toolbar (Search, Legend,
// Grid, Close) above the tabs beside the inspector, the search box and
// the legend under the colonist bar on the right, the route bar and the
// pod landing menu.

// Toolbar ids.
const (
	SearchID   = "world.search"
	LegendID   = "world.legend"
	GridID     = "world.grid"
	CloseID    = "world.close"
	toolW      = 84
	toolH      = 26
	searchGlow = 150 // frames a picked search result pulses
)

// ToolbarRect is the toolbar's row: above the tabs, right of the
// inspector.
func ToolbarRect(lay hudlayout.Layout) gfx.Rect {
	return gfx.Rect{X: lay.Pane.Right() + hudlayout.HudMargin, Y: lay.Tabs.Y - toolH - hudlayout.HudMargin, W: 4*toolW + 3*6, H: toolH}
}

// declareChrome declares the view's panels and returns the rectangles
// they cover (the globe never takes the pointer there).
func (v chromeBar) declareChrome(sw, sh int, p *planet.Planet) []gfx.Rect {
	lay := v.sh.Layout()
	out := v.declareTools(sw, p)
	ins := inspectorRect(lay)
	if v.pods.on {
		targeting(v).declarePodTargets(ins)
	} else {
		inspector(v).declareInspector(ins)
	}
	out = append(out, ins)
	if v.route.on {
		tb := ToolbarRect(lay)
		r := gfx.Rect{X: tb.X, Y: tb.Y - routeBarH - hudlayout.HudMargin, W: routeBarW, H: routeBarH}
		router(v).declareRouteBar(r)
		out = append(out, r)
	}
	if v.pods.on && !v.pods.menu {
		tb := ToolbarRect(lay)
		r := gfx.Rect{X: tb.X, Y: tb.Y - 40 - hudlayout.HudMargin, W: routeBarW, H: 40}
		v.kit().Panel(r)
		v.kit().Label(r.X+10, r.Y+12, gfx.Ellipsize(targeting(v).podsHead(), gfx.Body, float64(r.W-20)), gfx.Body)
		out = append(out, r)
	}
	if v.pods.on && v.pods.menu {
		r := targeting(v).podMenuRect(sw, sh)
		targeting(v).declarePodMenu(r)
		out = append(out, r)
	}
	return out
}

// podsHead is the targeting line under the globe.
func (v targeting) podsHead() string {
	return "Launch: click a target within the ring · the pods reach " + strconv.Itoa(v.pods.rng) + " tiles, " + strconv.Itoa(v.pods.loadKg) + " kg aboard · Esc cancels"
}

// declareTools declares the toolbar, the search box and the legend, and
// returns the rectangles they cover.
func (v chromeBar) declareTools(sw int, p *planet.Planet) []gfx.Rect {
	k := v.kit()
	lay := v.sh.Layout()
	tb := ToolbarRect(lay)
	x := tb.X
	btn := func(id, label, tip string, on bool, why string, fn func()) {
		if k.Button(id, gfx.Rect{X: x, Y: tb.Y, W: toolW, H: toolH}, label, uikit.BtnOpts{Selected: on, Disabled: why != "", Reason: why, Tip: tip}) {
			fn()
		}
		x += toolW + 6
	}
	why := ""
	if p == nil || v.g == nil {
		why = "The world is still being charted."
	}
	btn(SearchID, "Search", "Search\nFind a region or settlement by name.", v.tools.searchOn, why, func() { v.tools.searchOn = !v.tools.searchOn })
	btn(LegendID, "Legend", "Legend\nWhat the colours, roads and rivers mean.", v.tools.legendOn, why, func() { v.tools.legendOn = !v.tools.legendOn })
	grid := v.g != nil && v.g.Grid()
	btn(GridID, "Grid", "Grid\nOutline every tile (zoomed in).", grid, why, func() { v.g.SetGrid(!v.g.Grid()) })
	btn(CloseID, "Close", "Close\nBack to the map."+v.sh.KeyHint(input.TabWorld), false, "", v.Close)
	out := []gfx.Rect{tb}
	if p == nil || v.g == nil {
		return out
	}
	right := float32(sw) - hudlayout.HudMargin
	if !lay.Right.Empty() {
		right = lay.Right.X - hudlayout.HudMargin
	}
	y := max(lay.Bar.Bottom(), 0) + hudlayout.HudMargin
	if v.tools.searchOn {
		r := v.tools.search.Draw(k, SearchID, right, y, liveFinder{p, v.View}, func(h planet.Hit) { v.pickHit(p, h) })
		out = append(out, r)
		y = r.Bottom() + hudlayout.HudMargin
	}
	if v.tools.legendOn {
		out = append(out, worldinfo.DrawLegend(k, right, y, v.g.Mode(), units.FromUnit(v.sh.Settings().Gameplay.TempUnit)))
	}
	return out
}

// pickHit looks at a search result: its tile selected and centred, a
// named feature's tiles pulsing.
func (v chromeBar) pickHit(p *planet.Planet, h planet.Hit) {
	tiles := []int{h.Tile}
	if fs := p.Features(); h.Feature >= 0 && h.Feature < len(fs) {
		tiles = tiles[:0]
		for _, t := range fs[h.Feature].Tiles {
			tiles = append(tiles, int(t))
		}
	}
	v.selectKey("tile:" + strconv.Itoa(h.Tile))
	v.g.Center(h.Tile)
	v.g.Highlight(tiles, searchGlow)
}
