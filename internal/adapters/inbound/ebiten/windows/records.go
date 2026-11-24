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

// Records columns, measured from the right edge of the list.
const (
	recordsKillsFromRight = 380
	recordsDealtFromRight = 260
	recordsTakenFromRight = 130
	recordsNameReserve    = 400
)

// recordColumns declares the four Records columns; the header row reads its
// titles from here and the column test its ids.
func recordColumns(sh interface{ T(string) string }) [4]uikit.TableCol {
	return [4]uikit.TableCol{
		{ID: "colonist", Title: sh.T("Colonist")},
		{ID: "kills", Title: sh.T("Kills")},
		{ID: "dealt", Title: sh.T("Damage dealt")},
		{ID: "taken", Title: sh.T("Damage taken")},
	}
}

// recordTexts is one row's cell texts in column order (colonist, kills,
// damage dealt, damage taken).
func recordTexts(r recordRow) [4]string {
	return [4]string{r.Name, fmt.Sprint(r.Kills), fmt.Sprint(r.Damage), fmt.Sprint(r.Taken)}
}

// recordCells appends the four cells of a Records row in the list rect: the
// Colonist cell is the width its text is fitted to, the others start at their
// distance from the list's right edge and run to the next cell (Damage taken
// to the list's right edge).
func recordCells(dst []uikit.TableCell, list gfx.Rect) []uikit.TableCell {
	x, w := list.X, list.W
	kills, dealt, taken := x+w-recordsKillsFromRight, x+w-recordsDealtFromRight, x+w-recordsTakenFromRight
	row := func(col string, from, to float32) uikit.TableCell {
		return uikit.TableCell{Col: col, R: gfx.Rect{X: from, Y: list.Y, W: to - from, H: historyRowH}}
	}
	return append(dst,
		row("colonist", x+6, x+6+w-recordsNameReserve),
		row("kills", kills, dealt),
		row("dealt", dealt, taken),
		row("taken", taken, list.Right()),
	)
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
		draw := func(i int, texts [4]string, st gfx.TextStyle) {
			y := area.View.Y + float32(i)*historyRowH - area.Off
			if y+historyRowH < area.View.Y || y > area.View.Bottom() {
				return
			}
			if i%2 == 1 {
				uikit.FillRect(sub, gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: historyRowH}, rowStripe)
			}
			ty := gfx.CenterTop(float64(y), float64(historyRowH), st)
			var buf [4]uikit.TableCell
			for k, c := range recordCells(buf[:0], area.View) {
				txt := texts[k]
				if k == 0 {
					txt = uikit.FitText(txt, st, c.R.W)
				}
				gfx.Text(sub, txt, float64(c.R.X), ty, st)
			}
		}
		var head [4]string
		for k, c := range recordColumns(w.sh) {
			head[k] = c.Title
		}
		draw(0, head, dim)
		for i, r := range rows {
			draw(i+1, recordTexts(r), gfx.Body)
		}
	})
}
