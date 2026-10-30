package windows

import (
	"reflect"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The History window's Combat tab (DEF-4): the recent combat events from
// WorldView.CombatLog, newest first, read-only.

const combatEmpty = "No fighting lately."

// combatRow is one displayed combat event.
type combatRow struct{ When, Text string }

// CombatRows reads WorldView.CombatLog ([]CombatEventView{Tick, Text}) by
// reflection so the window builds before and after the field exists; the
// newest event comes first. A missing field is an empty log.
func CombatRows(world any) []combatRow {
	v := reflect.ValueOf(world)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	log := v.FieldByName("CombatLog")
	if !log.IsValid() || log.Kind() != reflect.Slice {
		return nil
	}
	out := make([]combatRow, 0, log.Len())
	for i := log.Len() - 1; i >= 0; i-- {
		e := log.Index(i)
		t, txt := e.FieldByName("Tick"), e.FieldByName("Text")
		if !t.IsValid() || !txt.IsValid() {
			continue
		}
		out = append(out, combatRow{When: historyDate(t.Convert(reflect.TypeOf(uint64(0))).Uint()), Text: txt.String()})
	}
	return out
}

// combatList declares the combat events' scrolling list in body.
func (w History) combatList(body gfx.Rect) {
	rows := CombatRows(w.sh.World())
	if len(rows) == 0 {
		w.kit().Label(body.X, body.Y, combatEmpty, uikit.DimText())
		return
	}
	area := w.kit().ScrollArea("history.combat", body, float32(len(rows))*historyRowH, uikit.ScrollOpts{Step: historyRowH * 2, Focusable: true})
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
			gfx.Text(sub, rows[i].When, float64(row.X+6), float64(y)+(historyRowH-gfx.LineH(dim))/2, dim)
			gfx.Text(sub, uikit.FitText(rows[i].Text, st, row.W-historyWhenW-10), float64(row.X+historyWhenW), float64(y)+(historyRowH-gfx.LineH(st))/2, st)
		}
	})
}
