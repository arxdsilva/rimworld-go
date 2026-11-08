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

// QuestPrefsModalID is the dialog's kit id; QuestPrefsCloseID its X.
const (
	QuestPrefsModalID = "quests.prefs.dialog"
	QuestPrefsCloseID = "quests.prefs.close"
)

// The dialog's measured geometry (QST-135), from the frame's top-left;
// row columns from the table's left edge.
const (
	questPrefXW       = 22 // the X: 22 x 25 at the top-right corner
	questPrefXH       = 25
	questPrefXInset   = 1  // the X's gap to the frame's right and top edges
	questPrefDescY    = 44 // the description under the heading (body top +44)
	questPrefTableGap = 9  // the description -> the table
	questPrefRowH     = 45
	questPrefIcon     = 30 // the faction emblem, 2 px in
	questPrefIconX    = 2
	questPrefNameX    = 45
	questPrefLabelX   = 295 // "Accept goodwill"
	questPrefCheckX   = 499
	questPrefCheckW   = 49
	questPrefGoodX    = 548 // goodwill over the relation word
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

// openPrefs pushes the dialog over the window: frameless, the heading in
// its body and an X at the top-right; Esc closes it too.
func (w Quests) openPrefs() {
	w.kit().Push(uikit.Modal{
		ID: QuestPrefsModalID, Title: w.sh.T("Reward preferences"), W: hudlayout.QuestPrefsW, H: hudlayout.QuestPrefsH, Frameless: true,
		Draw: func(body gfx.Rect) { w.prefsBody(body) },
	})
}

// prefsBody declares the heading, the X, the description and the faction
// rows; body is the frame less WindowPad.
func (w Quests) prefsBody(body gfx.Rect) {
	k := w.kit()
	frame := body.Inset(-hudlayout.WindowPad)
	x := gfx.Rect{X: frame.Right() - questPrefXInset - questPrefXW, Y: frame.Y + questPrefXInset, W: questPrefXW, H: questPrefXH}
	if k.Button(QuestPrefsCloseID, x, w.sh.T("X"), uikit.BtnOpts{Tip: w.sh.T("Close")}) {
		k.CloseModal(QuestPrefsModalID)
		return
	}
	hs := gfx.Title
	hs.Color = uikit.Heading
	k.Label(body.X, body.Y, uikit.FitText(w.sh.T("Reward preferences"), hs, x.X-body.X-8), hs)
	ds := uikit.DimText()
	y := body.Y + questPrefDescY
	for _, l := range gfx.WrapPx(w.sh.T("Untick a faction to stop quests from paying you in its goodwill; they offer other rewards instead."), ds, float64(body.W)) {
		k.Label(body.X, y, l, ds)
		y += float32(gfx.LineH(ds))
	}
	y += questPrefTableGap
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

// prefRow declares one faction row: the emblem, the name, "Accept
// goodwill" with its checkbox, then goodwill over the relation word in
// the relation's colour.
func (w Quests) prefRow(row gfx.Rect, i int, r questPrefRow, view gfx.Rect) {
	k := w.kit()
	label := w.sh.T("Accept goodwill")
	k.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, view)
		uikit.RowStripe(sub, row, i)
		cy := row.Y + row.H/2
		drawFacEmblem(sub, row.X+questPrefIconX+questPrefIcon/2, cy, questPrefIcon, r.faction)
		ns := gfx.Body
		ns.Color = uikit.Text
		ty := gfx.CenterTop(float64(row.Y), float64(row.H), ns)
		gfx.Text(sub, uikit.FitText(r.faction.Name, ns, questPrefLabelX-questPrefNameX-8), float64(row.X+questPrefNameX), ty, ns)
		gfx.Text(sub, uikit.FitText(label, ns, questPrefCheckX-questPrefLabelX-8), float64(row.X+questPrefLabelX), ty, ns)
		gs := gfx.Body
		gs.Color = r.faction.Color
		gw := max(0, row.W-questPrefGoodX-4)
		lh := gfx.LineH(gs)
		top := math.Round(float64(row.Y) + (float64(row.H)-2*lh)/2)
		gfx.Text(sub, uikit.FitText(goodwillText(r.faction.Goodwill), gs, gw), float64(row.X+questPrefGoodX), top, gs)
		gfx.Text(sub, uikit.FitText(r.faction.Standing, gs, gw), float64(row.X+questPrefGoodX), top+lh, gs)
	})
	cr := gfx.Rect{X: row.X + questPrefCheckX, Y: row.Y + (row.H-24)/2, W: questPrefCheckW, H: 24}
	if cr.Intersect(view) != cr {
		return
	}
	on := r.accepted // a fresh copy each Update: a toggle sends the other value
	if k.Check(ids.QuestPref+fmt.Sprint(r.ref), cr, "", &on,
		uikit.CheckOpts{Tip: w.sh.Tf("Accept goodwill\nQuests may pay in goodwill with %s.", r.faction.Name)}) {
		w.sh.Enqueue(command.Command{Kind: command.SetRewardPref, Index: r.ref, Enabled: !r.accepted})
	}
}

// drawFacEmblem is the Factions window's emblem (a disc in the relation's
// colour with the name's initial) at size px round (cx, cy).
func drawFacEmblem(dst *ebiten.Image, cx, cy, size float32, f Faction) {
	gfx.FillCircle(dst, cx, cy, size/2, f.Color, true)
	is := gfx.Label
	is.Color, is.Align = uikit.Field, gfx.AlignCenter
	gfx.Text(dst, facInitial(f.Name), float64(cx), gfx.CenterTop(float64(cy-size/2), float64(size), is), is)
}
