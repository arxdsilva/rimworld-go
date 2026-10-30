package windows

import (
	"fmt"
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// The History window's Records tab (CBT-5 UI): each colonist's kills and
// damage dealt (PawnView.Kills, DamageDealt), most kills first. Read-only.

const recordsEmpty = "No colonist has fought yet."

// recordRow is one colonist's combat record.
type recordRow struct {
	Name          string
	Kills, Damage int
}

// RecordRows are the colonists that have killed or hurt anything, most
// kills first (then most damage, then name); raiders are not listed.
func RecordRows(pawns []game.PawnView, label func(game.PawnView) string) []recordRow {
	var out []recordRow
	for _, p := range pawns {
		if p.Hostile || p.Kills == 0 && p.DamageDealt == 0 {
			continue
		}
		out = append(out, recordRow{Name: label(p), Kills: p.Kills, Damage: p.DamageDealt})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Kills != b.Kills {
			return a.Kills > b.Kills
		}
		if a.Damage != b.Damage {
			return a.Damage > b.Damage
		}
		return a.Name < b.Name
	})
	return out
}

// recordsList declares the combat records list in body.
func (w History) recordsList(body gfx.Rect) {
	rows := RecordRows(w.sh.Pawns(), w.sh.PawnLabel)
	if len(rows) == 0 {
		w.kit().Label(body.X, body.Y, recordsEmpty, uikit.DimText())
		return
	}
	area := w.kit().ScrollArea("history.records", body, float32(len(rows)+1)*historyRowH, uikit.ScrollOpts{Step: historyRowH * 2, Focusable: true})
	w.kit().Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		dim := gfx.WithColor(gfx.Small, uikit.TextDim)
		draw := func(i int, a, b, c string, st gfx.TextStyle) {
			y := area.View.Y + float32(i)*historyRowH - area.Off
			if y+historyRowH < area.View.Y || y > area.View.Bottom() {
				return
			}
			if i%2 == 1 {
				uikit.FillRect(sub, gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: historyRowH}, rowStripe)
			}
			ty := float64(y) + float64(historyRowH-gfx.LineH(st))/2
			gfx.Text(sub, uikit.FitText(a, st, area.View.W-300), float64(area.View.X+6), ty, st)
			gfx.Text(sub, b, float64(area.View.X+area.View.W-260), ty, st)
			gfx.Text(sub, c, float64(area.View.X+area.View.W-140), ty, st)
		}
		draw(0, "Colonist", "Kills", "Damage dealt", dim)
		for i, r := range rows {
			draw(i+1, r.Name, fmt.Sprint(r.Kills), fmt.Sprint(r.Damage), gfx.Body)
		}
	})
}
