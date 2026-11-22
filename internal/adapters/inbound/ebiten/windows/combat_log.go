// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// The History window's Combat and Threats tabs (DEF-4, DEF 3.1): the
// recent combat events from WorldView.CombatLog, newest first, read-only.
// Combat lists every entry; Threats lists the entries flagged Threat.

// combatRow is one displayed combat event.
type combatRow struct{ When, Text string }

// CombatRows lists every combat log entry, newest first.
func CombatRows(log []game.CombatEventView) []combatRow {
	return logRows(log, false)
}

// ThreatRows lists the threat entries only, newest first.
func ThreatRows(log []game.CombatEventView) []combatRow {
	return logRows(log, true)
}

func logRows(log []game.CombatEventView, threatsOnly bool) []combatRow {
	out := make([]combatRow, 0, len(log))
	for i := len(log) - 1; i >= 0; i-- {
		if threatsOnly && !log[i].Threat {
			continue
		}
		out = append(out, combatRow{When: historyDate(uint64(log[i].Tick)), Text: log[i].Text})
	}
	return out
}

// combatList declares the Combat tab's scrolling list in body.
func (w History) combatList(body gfx.Rect) {
	w.eventList(body, "history.combat", CombatRows(w.sh.World().CombatLog), w.sh.T("No fighting lately."))
}

// threatList declares the Threats tab's scrolling list in body.
func (w History) threatList(body gfx.Rect) {
	w.eventList(body, "history.threats", ThreatRows(w.sh.World().CombatLog), w.sh.T("No threats lately."))
}

// eventList is the one virtualised list drawer both tabs share.
func (w History) eventList(body gfx.Rect, id string, rows []combatRow, empty string) {
	if len(rows) == 0 {
		w.kit().Label(body.X, body.Y, empty, uikit.DimText())
		return
	}
	area := w.kit().ScrollArea(id, body, float32(len(rows))*historyRowH, uikit.ScrollOpts{Step: historyRowH * 2, Focusable: true})
	first := max(0, int(area.Off/historyRowH))
	w.kit().Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		for i := first; i < len(rows); i++ {
			y := area.View.Y + float32(i)*historyRowH - area.Off
			if y > area.View.Bottom() {
				break
			}
			row := gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: historyRowH}
			if i%2 == 1 {
				uikit.FillRect(sub, row, rowStripe)
			}
			dim := gfx.Small
			dim.Color = uikit.TextDim
			st := gfx.Body
			gfx.Text(sub, rows[i].When, float64(row.X+6), gfx.CenterTop(float64(y), historyRowH, dim), dim)
			gfx.Text(sub, uikit.FitText(rows[i].Text, st, row.W-historyWhenW-10), float64(row.X+historyWhenW), gfx.CenterTop(float64(y), historyRowH, st), st)
		}
	})
}
