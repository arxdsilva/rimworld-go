package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"strings"

	"rimworld-go/internal/application/game"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The gathering banner (ENG-003 M5, MAIN_UI_SCREENS §5 HUD messages): a
// small top-centre strip while a wedding or party is under way, read from
// the Gatherings read model. It only reads; the letter that announced the
// gathering is the message log's.

// gatheringReader is the read model the banner needs (consumer-owned; the
// game implements it). A simulation without it shows no banner.
type gatheringReader interface {
	Gatherings() []game.GatheringView
}

const (
	gatherBannerH   = 22
	gatherBannerPad = 10
)

// GatheringText is the banner line for v ("Wedding of Ada and Bo:
// ceremony", "Party hosted by Ada: under way").
func GatheringText(v game.GatheringView) string {
	hosts := strings.Join(v.Hosts, " and ")
	var s string
	switch v.Kind {
	case "wedding":
		s = "Wedding of " + hosts
	default:
		s = "Party"
		if hosts != "" {
			s += " hosted by " + hosts
		}
	}
	if v.Phase != "" {
		s += ": " + v.Phase
	}
	return s
}

// Line is the banner's line and rect on a screen sw wide
// (false: nothing under way).
func (c Banner) Line(sw int) (string, gfx.Rect, bool) {
	if !c.live() {
		return "", gfx.Rect{}, false
	}
	return c.LineIn(sw, c.sh.Layout())
}

// LineIn is gatheringBanner for the layout l: centred under the
// colonist bar (and under the messages, when some are up), never wider
// than the gap between the resource list and the right column.
func (c Banner) LineIn(sw int, l hudlayout.Layout) (string, gfx.Rect, bool) {
	rd, ok := c.sh.Env().Game.(gatheringReader)
	if !ok {
		return "", gfx.Rect{}, false
	}
	vs := rd.Gatherings()
	if len(vs) == 0 {
		return "", gfx.Rect{}, false
	}
	s := GatheringText(vs[0])
	room := float32(sw) - 2*(2*hudlayout.HudMargin+hudlayout.RightW)
	w := min(gfx.TextW(s, gfx.Body)+2*gatherBannerPad, max(room, 120))
	y := float32(hudlayout.HudMargin)
	if !l.Bar.Empty() {
		y = l.Bar.Bottom() + hudlayout.HudGap
	}
	if !l.Messages.Empty() {
		y = max(y, l.Messages.Bottom()+hudlayout.HudGap)
	}
	return s, gfx.Rect{X: (float32(sw) - w) / 2, Y: y, W: w, H: gatherBannerH}, true
}

// Declare declares the banner's tooltip (call from the colony UI).
func (c Banner) Declare(sw int) {
	if s, r, ok := c.Line(sw); ok {
		c.kit().Tip("hud.gathering", r, s+"\nFree colonists join in; a threat or a drafted host calls it off.")
	}
}

// Draw paints the banner (call from the HUD draw).
func (c Banner) Draw(screen *ebiten.Image, sw int) {
	if c.live() {
		c.DrawIn(screen, sw, c.sh.Layout())
	}
}

func (c Banner) DrawIn(screen *ebiten.Image, sw int, l hudlayout.Layout) {
	if c.Hidden || !c.live() {
		return
	}
	s, r, ok := c.LineIn(sw, l)
	if !ok {
		return
	}
	fill := uikit.Accent
	fill.A = 0x40
	gfx.FillRect(screen, r.X, r.Y, r.W, r.H, fill, false)
	gfx.StrokeRect(screen, r.X, r.Y, r.W, r.H, 1, uikit.Accent, false)
	gfx.Text(screen, uikit.FitText(s, gfx.Body, r.W-2*gatherBannerPad), float64(r.X)+gatherBannerPad, float64(r.Y)+2, gfx.Body)
}
