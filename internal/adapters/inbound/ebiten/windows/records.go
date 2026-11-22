// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"fmt"
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// The History window's Records tab (CBT-5 UI): each colonist's kills,
// damage dealt and damage taken (PawnView.Kills, DamageDealt, DamageTaken), most kills first. Read-only.

// recordRow is one colonist's combat record.
type recordRow struct {
	Name                 string
	Kills, Damage, Taken int
}

// RecordRows are the colonists that have killed, hurt or been hurt, most
// kills first (then most damage dealt, most taken, then name); raiders are not listed.
func RecordRows(pawns []game.PawnView, label func(game.PawnView) string) []recordRow {
	var out []recordRow
	for _, p := range pawns {
		if p.Hostile || p.Kills == 0 && p.DamageDealt == 0 && p.DamageTaken == 0 {
			continue
		}
		out = append(out, recordRow{Name: label(p), Kills: p.Kills, Damage: p.DamageDealt, Taken: p.DamageTaken})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kills != b.Kills {
			return a.Kills > b.Kills
		}
		if a.Damage != b.Damage {
			return a.Damage > b.Damage
		}
		if a.Taken != b.Taken {
			return a.Taken > b.Taken
		}
		return a.Name < b.Name
	})
	return out
}

// recordCell is one drawn cell of a Records row: its text, left x and, for
// the name column, the width the text is fitted to (0 = not fitted).
type recordCell struct {
	Text string
	X    float32
	Fit  float32
}

// Records columns, measured from the right edge of the list.
const (
	recordsKillsFromRight = 380
	recordsDealtFromRight = 260
	recordsTakenFromRight = 130
	recordsNameReserve    = 400
)

// recordCells lays one row's four cells (Colonist, Kills, Damage dealt,
// Damage taken) out in a list starting at x with width w.
func recordCells(x, w float32, text [4]string) [4]recordCell {
	return [4]recordCell{
		{Text: text[0], X: x + 6, Fit: w - recordsNameReserve},
		{Text: text[1], X: x + w - recordsKillsFromRight},
		{Text: text[2], X: x + w - recordsDealtFromRight},
		{Text: text[3], X: x + w - recordsTakenFromRight},
	}
}

// recordsList declares the combat records list in body.
func (w History) recordsList(body gfx.Rect) {
	rows := RecordRows(w.sh.Pawns(), w.sh.PawnLabel)
	if len(rows) == 0 {
		w.kit().Label(body.X, body.Y, w.sh.T("No colonist has fought yet."), uikit.DimText())
		return
	}
	area := w.kit().ScrollArea("history.records", body, float32(len(rows)+1)*historyRowH, uikit.ScrollOpts{Step: historyRowH * 2, Focusable: true})
	w.kit().Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		dim := gfx.WithColor(gfx.Small, uikit.TextDim)
		draw := func(i int, a, b, c, d string, st gfx.TextStyle) {
			y := area.View.Y + float32(i)*historyRowH - area.Off
			if y+historyRowH < area.View.Y || y > area.View.Bottom() {
				return
			}
			if i%2 == 1 {
				uikit.FillRect(sub, gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: historyRowH}, rowStripe)
			}
			ty := gfx.CenterTop(float64(y), float64(historyRowH), st)
			for _, c := range recordCells(area.View.X, area.View.W, [4]string{a, b, c, d}) {
				txt := c.Text
				if c.Fit > 0 {
					txt = uikit.FitText(txt, st, c.Fit)
				}
				gfx.Text(sub, txt, float64(c.X), ty, st)
			}
		}
		draw(0, w.sh.T("Colonist"), w.sh.T("Kills"), w.sh.T("Damage dealt"), w.sh.T("Damage taken"), dim)
		for i, r := range rows {
			draw(i+1, r.Name, fmt.Sprint(r.Kills), fmt.Sprint(r.Damage), fmt.Sprint(r.Taken), gfx.Body)
		}
	})
}
