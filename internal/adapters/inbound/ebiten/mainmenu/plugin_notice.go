package mainmenu

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Plugin problems notice (PLG-041, MAIN_UI §48): a card at the foot of the
// left column when plugins failed to load this run. View opens the
// problems view; warnings alone raise nothing.
const (
	PluginProblemsID = "menu.plugins.problems"
	pluginViewW      = 96
)

// PluginNotice is what the menu needs to say plugins failed to load.
type PluginNotice interface {
	// PluginErrors counts this run's plugin problems that are not
	// warnings.
	PluginErrors() int
}

// PluginCard is the notice's card: the update card's column, at the foot
// of the screen.
func PluginCard(sw, sh int) gfx.Rect {
	w := min(float32(updateCardW), RailX(sw)-updateCardX-updateRailGap)
	h := float32(2*ramCardPad + ramBtnH)
	return gfx.Rect{X: updateCardX, Y: float32(sh) - ramCardGap - h, W: w, H: h}
}

// declarePluginNotice declares the card while the run has plugin errors.
func (s *Screen) declarePluginNotice(sw, sh int) {
	n := s.sh.PluginErrors()
	if n == 0 {
		return
	}
	text := s.sh.Tf("%d plugin problems", n)
	if n == 1 {
		text = s.sh.T("1 plugin problem")
	}
	card := PluginCard(sw, sh)
	k := s.sh.Kit()
	k.Panel(card)
	st := gfx.WithColor(gfx.Label, uikit.Danger)
	btn := gfx.Rect{X: card.Right() - ramCardPad - pluginViewW, Y: card.Y + ramCardPad, W: pluginViewW, H: ramBtnH}
	k.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, gfx.Rect{X: card.X, Y: card.Y, W: ramCardStripe, H: card.H}, uikit.Danger)
		tx := float64(card.X + ramCardPad + ramCardStripe)
		w := float64(btn.X-card.X) - 2*ramCardPad - ramCardStripe
		gfx.Text(dst, gfx.Ellipsize(text, st, w), tx, gfx.CenterTop(float64(card.Y), float64(card.H), st), st)
	})
	s.noticeButton(PluginProblemsID, btn, s.sh.T("View"),
		uikit.BtnOpts{Tip: s.sh.T("View\nSee what went wrong while loading plugins, and turn a plugin off.")},
		func() { s.sh.Do(ports.Push{Screen: ports.ScreenPluginProblems}) })
}
