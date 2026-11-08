package windows

import (
	"fmt"
	"math"
	"slices"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/command"
)

// The reward-preferences dialog (QST-135): one row per faction that can
// pay goodwill, with a checkbox that sends command.SetRewardPref. Its
// state is WorldView.NoGoodwill, read every Update; the dialog keeps none.

// QuestPrefsModalID is the dialog's kit id; QuestPrefsCloseID its Close.
const (
	QuestPrefsModalID = "quests.prefs.dialog"
	QuestPrefsCloseID = "quests.prefs.close"
)

const (
	questPrefRowH   = 45 // QST-135
	questPrefCheckX = 260
	questPrefCheckW = 210
	questPrefGoodW  = 120
)

// questPrefRow is one faction of the dialog.
type questPrefRow struct {
	ref      int // 1 + the faction's index
	faction  Faction
	accepted bool
}

// prefRows are the factions the dialog lists: every named faction ref with
// a row in the Factions table (hidden factions and the Swarm have none), in
// ref order, each with its setting from noGoodwill.
func prefRows(name func(ref int) string, table []Faction, noGoodwill []int) []questPrefRow {
	var out []questPrefRow
	for ref := 1; ; ref++ {
		n := name(ref)
		if n == "" {
			return out
		}
		i := slices.IndexFunc(table, func(f Faction) bool { return f.Name == n })
		if i < 0 {
			continue
		}
		out = append(out, questPrefRow{ref: ref, faction: table[i], accepted: !slices.Contains(noGoodwill, ref)})
	}
}

// openPrefs pushes the dialog over the window.
func (w Quests) openPrefs() {
	w.kit().Push(uikit.Modal{
		ID: QuestPrefsModalID, Title: w.sh.T("Reward preferences"), W: hudlayout.QuestPrefsW, H: hudlayout.QuestPrefsH,
		Draw:    func(body gfx.Rect) { w.prefsBody(body) },
		Buttons: []uikit.ModalButton{{ID: QuestPrefsCloseID, Label: w.sh.T("Close"), Closes: true}},
	})
}

// prefsBody declares the description and the faction rows.
func (w Quests) prefsBody(body gfx.Rect) {
	k := w.kit()
	ds := uikit.DimText()
	y := body.Y
	for _, l := range gfx.WrapPx(w.sh.T("Untick a faction to stop quests from paying you in its goodwill; they offer other rewards instead."), ds, float64(body.W)) {
		k.Label(body.X, y, l, ds)
		y += float32(gfx.LineH(ds))
	}
	y += 8
	rows := prefRows(w.sh.FactionName, w.sh.Factions(), w.sh.World().NoGoodwill)
	if len(rows) == 0 {
		k.Label(body.X, y, uikit.FitText(w.sh.T("No faction can pay goodwill yet."), ds, body.W), ds)
		return
	}
	list := gfx.Rect{X: body.X, Y: y, W: body.W, H: max(0, body.Bottom()-y)}
	area := k.ScrollArea("quests.prefs.list", list, float32(len(rows))*questPrefRowH, uikit.ScrollOpts{Step: questPrefRowH, Focusable: true})
	first, last := uikit.ListVisible(area.Off, area.View.H, questPrefRowH, len(rows))
	for i := first; i <= last; i++ {
		row := gfx.Rect{X: area.View.X, Y: area.View.Y + float32(i)*questPrefRowH - area.Off, W: area.View.W, H: questPrefRowH}
		w.prefRow(row, i, rows[i], area.View)
	}
}

// prefRow declares one faction row: name, the checkbox, goodwill and
// standing.
func (w Quests) prefRow(row gfx.Rect, i int, r questPrefRow, view gfx.Rect) {
	k := w.kit()
	k.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, view)
		uikit.RowStripe(sub, row, i)
		ns := gfx.Body
		ns.Color = uikit.Text
		gfx.Text(sub, uikit.FitText(r.faction.Name, ns, questPrefCheckX-16), float64(row.X+8), gfx.CenterTop(float64(row.Y), float64(row.H), ns), ns)
		gs := gfx.Body
		gs.Color, gs.Align = r.faction.Color, gfx.AlignRight
		right := float64(row.Right() - 8)
		lh := gfx.LineH(gs)
		top := float64(row.Y) + (float64(row.H)-2*lh)/2
		gfx.Text(sub, uikit.FitText(goodwillText(r.faction.Goodwill), gs, questPrefGoodW), right, math.Round(top), gs)
		gfx.Text(sub, uikit.FitText(r.faction.Standing, gs, questPrefGoodW), right, math.Round(top+lh), gs)
	})
	cr := gfx.Rect{X: row.X + questPrefCheckX, Y: row.Y + (row.H-24)/2, W: questPrefCheckW, H: 24}
	if cr.Intersect(view) != cr {
		return
	}
	on := r.accepted // a fresh copy each Update: a toggle sends the other value
	if k.Check(ids.QuestPref+fmt.Sprint(r.ref), cr, uikit.FitText(w.sh.T("Accept goodwill"), gfx.Body, questPrefCheckW-30), &on,
		uikit.CheckOpts{Tip: w.sh.Tf("Accept goodwill\nQuests may pay in goodwill with %s.", r.faction.Name)}) {
		w.sh.Enqueue(command.Command{Kind: command.SetRewardPref, Index: r.ref, Enabled: !r.accepted})
	}
}
