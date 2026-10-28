package wizard

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/faction"
)

// The Factions dialog (NG-1 §2.8): who lives on the planet. At setup it is
// read-only and a row click centres the globe on the faction's capital.
// The table is a reusable widget so the colony window can show it too.

const (
	facModalMaxW = 1120
	facModalH    = 520
	FacRowH      = 34
	FacModalID   = "fac.modal"
	FacTableID   = "modal.fac.table"
	FacHighlight = 180 // frames the settlements pulse (3 s)
)

// factionColumns are the table's columns over rows.
func (w landingPage) factionColumns(rows []FacRow) []uikit.TableCol {
	return []uikit.TableCol{
		{ID: "emblem", Title: "", W: 44, Draw: func(dst *ebiten.Image, i int, cell gfx.Rect) {
			r := rows[i]
			w.DrawEmblem(dst, r.Emblem, r.Accent, cell.X+10, cell.Y+(cell.H-24)/2, 24)
		}},
		{ID: "name", Title: "Name", W: 200, Cell: func(i int) string { return rows[i].Name },
			Less: func(x, y int) bool { return strings.ToLower(rows[x].Name) < strings.ToLower(rows[y].Name) }},
		{ID: "type", Title: "Type", W: 100, Cell: func(i int) string { return rows[i].TypeLabel },
			Less: func(x, y int) bool { return rows[x].TypeLabel < rows[y].TypeLabel }},
		{ID: "leader", Title: "Leader", Cell: func(i int) string { return rows[i].Leader },
			Less: func(x, y int) bool { return rows[x].Leader < rows[y].Leader }},
		{ID: "goodwill", Title: "Goodwill", W: 150, Draw: func(dst *ebiten.Image, i int, cell gfx.Rect) { drawGoodwill(dst, cell, rows[i]) },
			Less: func(x, y int) bool { return rows[x].Goodwill < rows[y].Goodwill },
			Tip:  func(i int) string { return "Goodwill\n" + GoodwillTip(rows[i]) }},
		{ID: "relation", Title: "Relation", W: 90, Draw: func(dst *ebiten.Image, i int, cell gfx.Rect) {
			st := gfx.Body
			st.Color = RelationColor(rows[i].Relation)
			gfx.Text(dst, RelationLabel(rows[i].Relation), float64(cell.X+uikit.TableCellPad), float64(cell.Y)+(float64(cell.H)-gfx.LineH(st))/2, st)
		}, Less: func(x, y int) bool { return rows[x].Relation < rows[y].Relation }},
		{ID: "settle", Title: "Settlements", W: 100, Align: gfx.AlignRight, Cell: func(i int) string {
			if rows[i].Swarm {
				return "-"
			}
			return fmt.Sprint(rows[i].Settlements)
		}, Less: func(x, y int) bool { return rows[x].Settlements < rows[y].Settlements }},
		{ID: "tech", Title: "Tech", W: 100, Cell: func(i int) string { return rows[i].Tech },
			Less: func(x, y int) bool { return rows[x].Tech < rows[y].Tech }},
	}
}

func GoodwillTip(r FacRow) string {
	switch r.Relation {
	case faction.Hostile:
		return "Hostile. They raid you. Goodwill must climb to -10 before they calm down."
	case faction.Friendly:
		return "Friendly. They stay friendly while goodwill stays at +30 or more."
	}
	return "Neutral. At -50 or less they turn hostile; at +50 or more, friendly."
}

// drawGoodwill is the -100..+100 bar with the number.
func drawGoodwill(dst *ebiten.Image, cell gfx.Rect, r FacRow) {
	bar := gfx.Rect{X: cell.X + uikit.TableCellPad, Y: cell.Y + (cell.H-10)/2, W: cell.W - 2*uikit.TableCellPad - 36, H: 10}
	uikit.FillRect(dst, bar, uikit.Field)
	uikit.StrokeRect(dst, bar, uikit.PanelBorder)
	mid := bar.X + bar.W/2
	frac := float32(r.Goodwill) / 100
	c := RelationColor(r.Relation)
	if r.Relation == faction.Neutral {
		c = uikit.Accent
	}
	if frac >= 0 {
		uikit.FillRect(dst, gfx.Rect{X: mid, Y: bar.Y + 1, W: bar.W / 2 * frac, H: bar.H - 2}, c)
	} else {
		uikit.FillRect(dst, gfx.Rect{X: mid + bar.W/2*frac, Y: bar.Y + 1, W: -bar.W / 2 * frac, H: bar.H - 2}, c)
	}
	uikit.FillRect(dst, gfx.Rect{X: mid, Y: bar.Y - 1, W: 1, H: bar.H + 2}, uikit.TextDim)
	st := gfx.Small
	st.Align = gfx.AlignRight
	gfx.Text(dst, GoodwillText(r.Goodwill), float64(cell.Right()-uikit.TableCellPad), float64(cell.Y)+(float64(cell.H)-gfx.LineH(st))/2, st)
}

// factionsTable declares the table in r over rows; a click on a row
// reports it (once) through pick.
func (w landingPage) factionsTable(r gfx.Rect, rows []FacRow, sel *int, pick func(FacRow)) {
	if w.kit().Table(FacTableID, r, w.factionColumns(rows), len(rows), sel, FacRowH) && *sel >= 0 && *sel < len(rows) && pick != nil {
		pick(rows[*sel])
	}
}

// pushFactions opens the setup Factions dialog; a row click closes it and
// shows the faction's settlements on the globe.
func (w landingPage) pushFactions(ng *Draft, g *globe.View) {
	rows := FactionRows(ng.Planet.Factions)
	ng.facSel = -1
	w.kit().Push(uikit.Modal{
		ID: FacModalID, Title: "Factions", W: facModalMaxW, H: facModalH,
		Draw: func(body gfx.Rect) {
			tb, hint := body.SplitV(-24)
			w.kit().Label(hint.X, hint.Y+6, "Click a faction to find its capital on the globe.", uikit.DimText())
			w.factionsTable(gfx.Rect{X: tb.X, Y: tb.Y, W: tb.W, H: tb.H}, rows, &ng.facSel, func(r FacRow) {
				if r.Capital < 0 {
					return // the Swarm has no settlements
				}
				w.kit().CloseModal(FacModalID)
				if g != nil {
					g.Center(r.Capital)
					g.Highlight(r.Tiles, FacHighlight)
				}
			})
		},
		Buttons: []uikit.ModalButton{{ID: "close", Label: "Close", Style: uikit.BtnPrimary, Closes: true}},
	})
}
