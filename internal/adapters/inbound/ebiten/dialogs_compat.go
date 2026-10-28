package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
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

// Trade is the open trade session, if the simulation offers sessions.
func (h dialogsHost) Trade() (game.TradeView, bool) {
	if r := h.a.renderer; r != nil {
		if s, ok := r.game.(tradeSource); ok {
			return s.TradeView()
		}
	}
	return game.TradeView{}, false
}

// Comms is the open comms session, if the simulation offers sessions.
func (h dialogsHost) Comms() (game.CommsView, bool) {
	if r := h.a.renderer; r != nil {
		if s, ok := r.game.(commsSource); ok {
			return s.CommsView()
		}
	}
	return game.CommsView{}, false
}

// Sites are the settlements a caravan may visit.
func (h dialogsHost) Sites() []game.WorldSiteView {
	if r := h.a.renderer; r != nil {
		if s, ok := r.game.(worldSource); ok {
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

type (
	tradeSource = dialogs.TradeSource
	commsSource = dialogs.CommsSource
)

const (
	tradeModalID   = dialogs.TradeModalID
	commsModalID   = dialogs.CommsModalID
	commsGiftStep  = dialogs.CommsGiftStep
	commsGiftStart = dialogs.CommsGiftStart
	formCaravanID  = dialogs.FormCaravanID
	formNoMembers  = dialogs.FormNoMembers
	formNoDest     = dialogs.FormNoDest
)

// tradeLine is one row of the trade table (test-only reach).
type tradeLine struct {
	head string
	row  game.TradeRowView
}

func tradeLines(rows []game.TradeRowView) []tradeLine {
	var out []tradeLine
	for _, l := range dialogs.TradeLines(rows) {
		out = append(out, tradeLine{head: l.Head, row: l.Row})
	}
	return out
}

func caravanHealthNote(c game.CaravanView) string { return dialogs.CaravanHealthNote(c) }
func caravanTip(c game.CaravanView) string        { return dialogs.CaravanTip(c) }
func memberList(ids []entity.EntityID) string     { return dialogs.MemberList(ids) }

func (a *App) syncDealings() { a.deal.SyncDealings() }
