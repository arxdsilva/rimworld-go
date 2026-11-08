package worldview

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The view's drawing over the globe, all through the canvas (logical
// pixels; markers are placed by the projector): the markers and the
// route line. Settlement emblems are the globe's own.

// Marker art (ui_icon; a missing key draws a tinted disc).
const (
	iconColony    = "ui_icon/colony_home"
	iconSite      = "ui_icon/site_camp"
	iconEncounter = "ui_icon/encounter"
	iconDestroyed = "ui_icon/site_ruined"
	iconThreat    = "ui_icon/map_threat"
	iconCaravan   = "ui_icon/caravan"
	iconPod       = "ui_icon/pod_flight"
	iconMining    = "ui_icon/mining_site"
	iconQuest     = "ui_icon/quest_site"
)

// markLook is how object o's marker draws: the icon key ("" none: the
// globe draws a settlement's emblem) and the disc tint if the icon is
// missing.
func markLook(o *Object) (key string, tint color.RGBA) {
	switch o.Kind {
	case KindCaravan:
		return iconCaravan, uikit.Heading
	case KindPod:
		return iconPod, uikit.Heading
	case KindQuest:
		return iconQuest, uikit.Accent
	case KindSite:
		return iconMining, uikit.Accent
	case KindSettlement:
		return "", uikit.TextDim
	}
	switch {
	case o.Destroyed():
		return iconDestroyed, uikit.TextDim
	case o.Map.Kind == "colony":
		return iconColony, uikit.Accent
	case o.Map.Kind == "encounter":
		return iconEncounter, uikit.Danger
	}
	return iconSite, uikit.Heading
}

// drawMarks draws every visible marker: a dark disc, the icon (or a
// tinted disc), the threat icon on a map under attack, and the selection
// ring.
func (v *View) drawMarks(dst *ebiten.Image) {
	cat := v.sh.Env().Art
	back := color.RGBA{R: uikit.Space.R, G: uikit.Space.G, B: uikit.Space.B, A: 170}
	icon := func(key string, x, y, s float64) bool {
		if cat == nil {
			return false
		}
		img := cat.Image(key, color.NRGBA{})
		if img == nil {
			return false
		}
		gfx.Image(dst, img, x-s/2, y-s/2, s, s, nil)
		return true
	}
	for _, m := range v.marks {
		o := &v.objs[m.obj]
		key, tint := markLook(o)
		x, y, s := float64(m.x), float64(m.y), float64(m.r)*1.6
		if key != "" {
			gfx.FillCircle(dst, m.x, m.y, m.r, back, true)
			if !icon(key, x, y, s) {
				gfx.FillCircle(dst, m.x, m.y, m.r*0.55, tint, true)
			}
		}
		if o.Kind == KindMap && o.Map.Threat && !icon(iconThreat, x+s/2, y-s/2, s*0.6) {
			gfx.StrokeCircle(dst, m.x, m.y, m.r+1, 2, uikit.Danger, true)
		}
		if o.Key == v.sel.key {
			gfx.StrokeCircle(dst, m.x, m.y, m.r+3, 2, uikit.Accent, true)
		}
	}
}

// drawRoute draws the planned route as a line over the relief (the
// facing segments).
func (v router) drawRoute(dst *ebiten.Image) {
	path := v.route.path
	if !v.route.on || len(path) < 2 {
		return
	}
	pj := newProjector(v.g)
	shadow := color.RGBA{R: 10, G: 14, B: 20, A: 200}
	x0, y0, ok0 := pj.at(v.g.Surface(path[0]))
	for _, t := range path[1:] {
		x1, y1, ok1 := pj.at(v.g.Surface(t))
		if ok0 && ok1 {
			gfx.Line(dst, x0, y0, x1, y1, 4, shadow, true)
			gfx.Line(dst, x0, y0, x1, y1, 2, uikit.Accent, true)
		}
		x0, y0, ok0 = x1, y1, ok1
	}
}
