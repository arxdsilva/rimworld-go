package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
)

// drawPawnPortrait draws the whole colonist, fitted into the logical box
// (x, y, w, h): the inspector's portrait.
func (a *App) drawPawnPortrait(screen *ebiten.Image, p game.PawnView, x, y, w, h float64) {
	if a.renderer.art != nil {
		a.renderer.drawPawnPortraitArt(screen, p, x, y, w, h)
	}
}

func buildAlerts(pawns []game.PawnView, w game.WorldView) []hud.Alert {
	return hud.BuildAlerts(pawns, w)
}

func tradeAlertsFrom(src any) []hud.Alert { return hud.TradeAlertsFrom(src) }

func fireAlert(w game.WorldView) (hud.Alert, bool) {
	a, ok := hud.FireAlert(w)
	return a, ok
}

func waitingFramesAlert(w game.WorldView) (hud.Alert, bool) {
	a, ok := hud.WaitingFramesAlert(w)
	return a, ok
}

func (a *App) currentAlerts() []hud.Alert { return a.chrome.Alerts().Current() }

func alertRowLabel(r hud.AlertRow) string {
	return hud.AlertRowLabel(hud.AlertRow{R: r.R, Al: r.Al, More: r.More, Hidden: r.Hidden})
}

func (a *App) alertRows(l hudlayout.Layout) []hud.AlertRow {
	rows := a.chrome.Alerts().Rows(l)
	if rows == nil {
		return nil
	}
	out := make([]hud.AlertRow, len(rows))
	for i, r := range rows {
		out[i] = hud.AlertRow{R: r.R, Al: r.Al, More: r.More, Hidden: r.Hidden}
	}
	return out
}

func layoutRightCard(c hud.RightCard, anchor gfx.Rect, sw, sh int) (box gfx.Rect, title, body []string) {
	return hud.LayoutRightCard(hud.RightCard{Title: c.Title, Body: c.Body, Clr: c.Clr}, anchor, sw, sh)
}

func (a *App) rightHover(l hudlayout.Layout) (hud.RightCard, gfx.Rect, bool) {
	c, r, ok := a.chrome.Right().Hover(l)
	return hud.RightCard{Title: c.Title, Body: c.Body, Clr: c.Clr}, r, ok
}

func letterRowLabel(r hud.LetterRow) string {
	return hud.LetterRowLabel(hud.LetterRow{R: r.R, Letter: r.Letter, Older: r.Older})
}

func (a *App) letterRows(l hudlayout.Layout) []hud.LetterRow {
	rows := a.chrome.Right().LetterRows(l)
	if rows == nil {
		return nil
	}
	out := make([]hud.LetterRow, len(rows))
	for i, r := range rows {
		out[i] = hud.LetterRow{R: r.R, Letter: r.Letter, Older: r.Older}
	}
	return out
}

// letterKinds is hud.LetterKinds as the root tests read it.
var letterKinds = func() []struct {
	kind, icon string
	clr        color.RGBA
} {
	out := make([]struct {
		kind, icon string
		clr        color.RGBA
	}, len(hud.LetterKinds))
	for i, k := range hud.LetterKinds {
		out[i].kind, out[i].icon, out[i].clr = k.Kind, k.Icon, k.Clr
	}
	return out
}()

func (a *App) messageRows(l hudlayout.Layout) []hud.MessageRow {
	rows := a.chrome.Right().MessageRows(l)
	if rows == nil {
		return nil
	}
	out := make([]hud.MessageRow, len(rows))
	for i, r := range rows {
		out[i] = hud.MessageRow{R: r.R, Msg: r.Msg, Idx: r.Idx, Alpha: r.Alpha}
	}
	return out
}

func statusChips(w game.WorldView) []hud.StatusChip { return hud.StatusChips(w) }

func (a *App) chipRects(chips []hud.StatusChip, sw int) []gfx.Rect {
	return a.chrome.Global().ChipRects(chips, sw)
}

func (a *App) globalPlan() hud.GlobalPlan {
	p := a.chrome.Global().Plan()
	return hud.GlobalPlan{Reason: p.Reason, Weather: p.Weather, Chips: p.Chips, Legend: p.Legend}
}

// globalToggleList is hud.GlobalToggles as the root tests read it.
var globalToggleList = func() []struct{ id, icon, short string } {
	out := make([]struct{ id, icon, short string }, len(hud.GlobalToggles))
	for i, t := range hud.GlobalToggles {
		out[i].id, out[i].icon, out[i].short = t.ID, t.Icon, t.Short
	}
	return out
}()

func (a *App) liveToasts() []hud.Toast {
	ts := a.chrome.LiveToasts()
	if ts == nil {
		return nil
	}
	out := make([]hud.Toast, len(ts))
	for i, t := range ts {
		out[i] = hud.Toast{Text: t.Text, At: t.At}
	}
	return out
}

func (a *App) resourceRows() []hud.ResRow {
	rows := a.chrome.Res().Rows()
	if rows == nil {
		return nil
	}
	out := make([]hud.ResRow, len(rows))
	for i, r := range rows {
		out[i] = hud.ResRow{Header: r.Header, Cat: r.Cat, Kind: r.Kind, Label: r.Label, Qty: r.Qty}
	}
	return out
}

func (a *App) barStatusesOf(p game.PawnView) []hud.BarStatus {
	return a.chrome.Bar().StatusesOf(p)
}
