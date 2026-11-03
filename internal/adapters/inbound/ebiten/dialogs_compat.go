package ebiten

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/worldview"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
)

// dialogs.Dealings owns the trade, comms and caravan dialogs (ARCH-005);
// dialogsHost is the root's side of dialogs.Host and newDialogs builds the
// feature for NewApp. The aliases and one-line wrappers keep the root's
// other call sites unchanged until the sweep (SOLID_GO.md §11).

type dialogsHost struct{ a *App }

var _ dialogs.Host = dialogsHost{}

// dialogsShell is the feature's view of the App: the ports roles plus its Host.
type dialogsShell struct {
	shell
	dialogsHost
}

func newDialogs(a *App) *dialogs.Dealings {
	return dialogs.New(dialogsShell{a.shell, dialogsHost{a}})
}

// DrawPortrait is the wizard's portrait, for the New wanderers card.
func (h dialogsHost) DrawPortrait(dst *ebiten.Image, l look.Look, x, y, w, ht float64) bool {
	return drawLookPortrait(dst, h.a.art, l, x, y, w, ht)
}

// Icon is a UI icon, untinted.
func (h dialogsHost) Icon(key string) *ebiten.Image { return h.a.art.Image(key, color.NRGBA{}) }

// Trade is the open trade session, if the simulation offers sessions.
func (h dialogsHost) Trade() (game.TradeView, bool) {
	if r := h.a.renderer; r != nil {
		if s, ok := r.game.(dialogs.TradeSource); ok {
			return s.TradeView()
		}
	}
	return game.TradeView{}, false
}

// Comms is the open comms session, if the simulation offers sessions.
func (h dialogsHost) Comms() (game.CommsView, bool) {
	if r := h.a.renderer; r != nil {
		if s, ok := r.game.(dialogs.CommsSource); ok {
			return s.CommsView()
		}
	}
	return game.CommsView{}, false
}

// Sites are the settlements a caravan may visit.
func (h dialogsHost) Sites() []game.WorldSiteView {
	if r := h.a.renderer; r != nil {
		if s, ok := r.game.(worldview.WorldSource); ok {
			return s.WorldSites()
		}
	}
	return nil
}

// HoldSpeed sets the speed tier without the speed cue.
func (h dialogsHost) HoldSpeed(tier int) {
	if r := h.a.renderer; r != nil {
		r.speed = speed(tier)
	}
}
