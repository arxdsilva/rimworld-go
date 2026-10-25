package ebiten

import (
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
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

// gatheringText is the banner line for v ("Wedding of Ada and Bo:
// ceremony", "Party hosted by Ada: under way").
func gatheringText(v game.GatheringView) string {
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

// gatheringBanner is the banner's line and rect on a screen sw wide
// (false: nothing under way).
func (a *App) gatheringBanner(sw int) (string, rect, bool) {
	if a.renderer == nil {
		return "", rect{}, false
	}
	rd, ok := a.renderer.game.(gatheringReader)
	if !ok {
		return "", rect{}, false
	}
	vs := rd.Gatherings()
	if len(vs) == 0 {
		return "", rect{}, false
	}
	s := gatheringText(vs[0])
	w := min(textW(s, tsBody)+2*gatherBannerPad, float32(sw)-2*hudPad)
	return s, rect{(float32(sw) - w) / 2, hudTopH + 4, w, gatherBannerH}, true
}

// gatheringUI declares the banner's tooltip (call from the colony UI).
func (a *App) gatheringUI(sw int) {
	if s, r, ok := a.gatheringBanner(sw); ok {
		a.uiTip("hud.gathering", r, s+"\nFree colonists join in; a threat or a drafted host calls it off.")
	}
}

// drawGatheringBanner paints the banner (call from the HUD draw).
func (a *App) drawGatheringBanner(screen *ebiten.Image, sw int) {
	s, r, ok := a.gatheringBanner(sw)
	if !ok || a.hideUI {
		return
	}
	fill := uiAccent
	fill.A = 0x40
	uiFillRect(screen, r.X, r.Y, r.W, r.H, fill, false)
	uiStrokeRect(screen, r.X, r.Y, r.W, r.H, 1, uiAccent, false)
	drawText(screen, fitText(s, tsBody, r.W-2*gatherBannerPad), float64(r.X)+gatherBannerPad, float64(r.Y)+2, tsBody)
}
