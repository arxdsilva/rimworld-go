package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/faction"
)

// The Factions dialog (NG-1 §2.8): who lives on the planet. At setup it is
// read-only and a row click centres the globe on the faction's capital.
// The table is a reusable widget so the colony window can show it too.

const (
	facModalMaxW = 1120
	facModalH    = 520
	facRowH      = 34
	facModalID   = "fac.modal"
	facTableID   = "modal.fac.table"
	facHighlight = 180 // frames the settlements pulse (3 s)
)

// factionColumns are the table's columns over rows.
func (a *App) factionColumns(rows []facRow) []tableCol {
	return []tableCol{
		{ID: "emblem", Title: "", W: 44, Draw: func(dst *ebiten.Image, i int, cell rect) {
			r := rows[i]
			a.drawEmblem(dst, r.Emblem, r.Accent, cell.X+10, cell.Y+(cell.H-24)/2, 24)
		}},
		{ID: "name", Title: "Name", W: 200, Cell: func(i int) string { return rows[i].Name },
			Less: func(x, y int) bool { return strings.ToLower(rows[x].Name) < strings.ToLower(rows[y].Name) }},
		{ID: "type", Title: "Type", W: 100, Cell: func(i int) string { return rows[i].TypeLabel },
			Less: func(x, y int) bool { return rows[x].TypeLabel < rows[y].TypeLabel }},
		{ID: "leader", Title: "Leader", Cell: func(i int) string { return rows[i].Leader },
			Less: func(x, y int) bool { return rows[x].Leader < rows[y].Leader }},
		{ID: "goodwill", Title: "Goodwill", W: 150, Draw: func(dst *ebiten.Image, i int, cell rect) { drawGoodwill(dst, cell, rows[i]) },
			Less: func(x, y int) bool { return rows[x].Goodwill < rows[y].Goodwill },
			Tip:  func(i int) string { return "Goodwill\n" + goodwillTip(rows[i]) }},
		{ID: "relation", Title: "Relation", W: 90, Draw: func(dst *ebiten.Image, i int, cell rect) {
			st := tsBody
			st.Color = relationColor(rows[i].Relation)
			drawText(dst, relationLabel(rows[i].Relation), float64(cell.X+tableCellPad), float64(cell.Y)+(float64(cell.H)-lineH(st))/2, st)
		}, Less: func(x, y int) bool { return rows[x].Relation < rows[y].Relation }},
		{ID: "settle", Title: "Settlements", W: 100, Align: alignRight, Cell: func(i int) string {
			if rows[i].Swarm {
				return "-"
			}
			return fmt.Sprint(rows[i].Settlements)
		}, Less: func(x, y int) bool { return rows[x].Settlements < rows[y].Settlements }},
		{ID: "tech", Title: "Tech", W: 100, Cell: func(i int) string { return rows[i].Tech },
			Less: func(x, y int) bool { return rows[x].Tech < rows[y].Tech }},
	}
}

func goodwillTip(r facRow) string {
	switch r.Relation {
	case faction.Hostile:
		return "Hostile. They raid you. Goodwill must climb to -10 before they calm down."
	case faction.Friendly:
		return "Friendly. They stay friendly while goodwill stays at +30 or more."
	}
	return "Neutral. At -50 or less they turn hostile; at +50 or more, friendly."
}

// drawGoodwill is the -100..+100 bar with the number.
func drawGoodwill(dst *ebiten.Image, cell rect, r facRow) {
	bar := rect{X: cell.X + tableCellPad, Y: cell.Y + (cell.H-10)/2, W: cell.W - 2*tableCellPad - 36, H: 10}
	fillRect(dst, bar, uiField)
	strokeRect(dst, bar, uiPanelBorder)
	mid := bar.X + bar.W/2
	frac := float32(r.Goodwill) / 100
	c := relationColor(r.Relation)
	if r.Relation == faction.Neutral {
		c = uiAccent
	}
	if frac >= 0 {
		fillRect(dst, rect{X: mid, Y: bar.Y + 1, W: bar.W / 2 * frac, H: bar.H - 2}, c)
	} else {
		fillRect(dst, rect{X: mid + bar.W/2*frac, Y: bar.Y + 1, W: -bar.W / 2 * frac, H: bar.H - 2}, c)
	}
	fillRect(dst, rect{X: mid, Y: bar.Y - 1, W: 1, H: bar.H + 2}, uiTextDim)
	st := tsSmall
	st.Align = alignRight
	drawText(dst, goodwillText(r.Goodwill), float64(cell.Right()-tableCellPad), float64(cell.Y)+(float64(cell.H)-lineH(st))/2, st)
}

// factionsTable declares the table in r over rows; a click on a row
// reports it (once) through pick.
func (a *App) factionsTable(r rect, rows []facRow, sel *int, pick func(facRow)) {
	if a.uiTable(facTableID, r, a.factionColumns(rows), len(rows), sel, facRowH) && *sel >= 0 && *sel < len(rows) && pick != nil {
		pick(rows[*sel])
	}
}

// pushFactions opens the setup Factions dialog; a row click closes it and
// shows the faction's settlements on the globe.
func (a *App) pushFactions(ng *newGameState, g *globeView) {
	rows := factionRows(ng.planet.Factions)
	ng.facSel = -1
	a.uiPushModal(modal{
		id: facModalID, title: "Factions", w: facModalMaxW, h: facModalH,
		draw: func(a *App, body rect) {
			tb, hint := body.SplitV(-24)
			a.uiLabel(hint.X, hint.Y+6, "Click a faction to find its capital on the globe.", dimText())
			a.factionsTable(rect{X: tb.X, Y: tb.Y, W: tb.W, H: tb.H}, rows, &ng.facSel, func(r facRow) {
				if r.Capital < 0 {
					return // the Swarm has no settlements
				}
				a.uiCloseModal(facModalID)
				if g != nil {
					g.Center(r.Capital)
					g.Highlight(r.Tiles, facHighlight)
				}
			})
		},
		buttons: []modalButton{{id: "close", label: "Close", style: btnPrimary, closes: true}},
	})
}
