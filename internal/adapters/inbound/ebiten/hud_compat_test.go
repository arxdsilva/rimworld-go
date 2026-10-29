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

func buildAlerts(pawns []game.PawnView, w game.WorldView) []alert {
	return toAlerts(hud.BuildAlerts(pawns, w))
}

func tradeAlertsFrom(src any) []alert { return toAlerts(hud.TradeAlertsFrom(src)) }

func fireAlert(w game.WorldView) (alert, bool) {
	a, ok := hud.FireAlert(w)
	return toAlert(a), ok
}

func waitingFramesAlert(w game.WorldView) (alert, bool) {
	a, ok := hud.WaitingFramesAlert(w)
	return toAlert(a), ok
}

func (a *App) currentAlerts() []alert { return toAlerts(a.chrome.Alerts().Current()) }

func alertRowLabel(r alertRow) string {
	return hud.AlertRowLabel(hud.AlertRow{R: r.r, Al: fromAlert(r.al), More: r.more, Hidden: fromAlerts(r.hidden)})
}

func (a *App) alertRows(l hudlayout.Layout) []alertRow {
	rows := a.chrome.Alerts().Rows(l)
	if rows == nil {
		return nil
	}
	out := make([]alertRow, len(rows))
	for i, r := range rows {
		out[i] = alertRow{r: r.R, al: toAlert(r.Al), more: r.More, hidden: toAlerts(r.Hidden)}
	}
	return out
}

func layoutRightCard(c rightCard, anchor gfx.Rect, sw, sh int) (box gfx.Rect, title, body []string) {
	return hud.LayoutRightCard(hud.RightCard{Title: c.title, Body: c.body, Clr: c.clr}, anchor, sw, sh)
}

func (a *App) rightHover(l hudlayout.Layout) (rightCard, gfx.Rect, bool) {
	c, r, ok := a.chrome.Right().Hover(l)
	return rightCard{title: c.Title, body: c.Body, clr: c.Clr}, r, ok
}

func letterRowLabel(r letterRow) string {
	return hud.LetterRowLabel(hud.LetterRow{R: r.r, Letter: r.letter, Older: r.older})
}

func (a *App) letterRows(l hudlayout.Layout) []letterRow {
	rows := a.chrome.Right().LetterRows(l)
	if rows == nil {
		return nil
	}
	out := make([]letterRow, len(rows))
	for i, r := range rows {
		out[i] = letterRow{r: r.R, letter: r.Letter, older: r.Older}
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

func (a *App) messageRows(l hudlayout.Layout) []messageRow {
	rows := a.chrome.Right().MessageRows(l)
	if rows == nil {
		return nil
	}
	out := make([]messageRow, len(rows))
	for i, r := range rows {
		out[i] = messageRow{r: r.R, msg: r.Msg, idx: r.Idx, alpha: r.Alpha}
	}
	return out
}

func statusChips(w game.WorldView) []statusChip { return toChips(hud.StatusChips(w)) }

func (a *App) chipRects(chips []statusChip, sw int) []gfx.Rect {
	return a.chrome.Global().ChipRects(fromChips(chips), sw)
}

func (a *App) globalPlan() globalPlan {
	p := a.chrome.Global().Plan()
	return globalPlan{reason: p.Reason, weather: p.Weather, chips: toChips(p.Chips), legend: p.Legend}
}

// globalToggleList is hud.GlobalToggles as the root tests read it.
var globalToggleList = func() []struct{ id, icon, short string } {
	out := make([]struct{ id, icon, short string }, len(hud.GlobalToggles))
	for i, t := range hud.GlobalToggles {
		out[i].id, out[i].icon, out[i].short = t.ID, t.Icon, t.Short
	}
	return out
}()

func (a *App) liveToasts() []toast {
	ts := a.chrome.LiveToasts()
	if ts == nil {
		return nil
	}
	out := make([]toast, len(ts))
	for i, t := range ts {
		out[i] = toast{text: t.Text, at: t.At}
	}
	return out
}

func (a *App) resourceRows() []resRow {
	rows := a.chrome.Res().Rows()
	if rows == nil {
		return nil
	}
	out := make([]resRow, len(rows))
	for i, r := range rows {
		out[i] = resRow{header: r.Header, cat: r.Cat, kind: r.Kind, label: r.Label, qty: r.Qty}
	}
	return out
}

func (a *App) barStatusesOf(p game.PawnView) []barStatus {
	return toStatuses(a.chrome.Bar().StatusesOf(p))
}
