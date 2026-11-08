package windows

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	qd "rimworld-go/internal/domain/quest"
)

// The Quests window (QST-120..139, UIS-066): the Available, Active and
// Historical lists over WorldView.Quests on the left, the picked quest's
// detail on the right. Opened from the HUD's Quests tab (F7) or a quest's
// letter (ports.ShowQuest). Its clicks leave as quest commands.

// QuestsModalID is the window's kit id.
const QuestsModalID = "colony.quests"

// The window's own scroll views.
const (
	questsListID   = "quests.list"
	questsDetailID = "quests.detail"
)

// The list's tabs.
const (
	questsAvail = iota
	questsActive
	questsHistory
)

// questsState is the window's per-session UI state.
type questsState struct {
	tab    int    // questsAvail, questsActive or questsHistory
	sel    uint32 // the picked quest's id (0: the tab's first)
	search string // the search field: rows whose title holds it
	shown  uint32 // the quest the detail showed last (its scroll resets on a change)
}

// Toggle opens the window, or closes it when it is the open one.
func (w Quests) Toggle() { w.toggleKit(QuestsModalID, w.Launch) }

// Launch opens the window on its last tab and pick: a main-tab window in
// the Window rect (window.go).
func (w Quests) Launch() {
	if !w.live() {
		return
	}
	w.OpenKit(uikit.Modal{
		ID: QuestsModalID, Title: w.sh.T("Quests"), W: hudlayout.QuestsW, H: hudlayout.QuestsH, Anchor: w.Anchor, Frameless: true,
		Draw: func(body gfx.Rect) { w.body(body) },
	})
}

// Select opens the window on quest id: the tab its state belongs to, its
// row picked (ports.ShowQuest). An id not in the read model only opens it.
func (w Quests) Select(id uint32) {
	for _, q := range w.sh.World().Quests {
		if uint32(q.ID) == id {
			w.quests.tab, w.quests.sel = questTab(q.State, q.Dismissed), id
		}
	}
	w.Launch()
}

// size is the window's outer size.
func (w Quests) size() (float32, float32) { return hudlayout.QuestsW, hudlayout.QuestsH }

// body declares the tabs, the list, the search field, the preferences
// button and the picked quest's detail.
func (w Quests) body(body gfx.Rect) {
	if !w.live() {
		return
	}
	st, all := w.quests, w.sh.World().Quests
	lay := questsLayout(body)
	w.tabs(lay, all)
	rows := questRows(all, st.tab, func(q game.QuestView) string { return questTitle(w.sh, q) }, st.search)
	pick := questPick(rows, st.sel)
	w.list(lay.List, rows, pick)
	w.kit().TextField(ids.QuestsSearch, lay.Search, &st.search, uikit.TextFieldSpec{MaxLen: 40, Placeholder: w.sh.T("Search quests"),
		Tip: w.sh.T("Search\nShow only the quests whose name holds this text.")})
	if w.kit().Button(ids.QuestsPrefs, lay.Prefs, w.sh.T("Reward preferences"), uikit.BtnOpts{
		Tip: w.sh.T("Reward preferences\nChoose the factions whose goodwill you do not want as a reward.")}) {
		w.openPrefs()
	}
	var q *game.QuestView
	if pick >= 0 {
		q = &rows[pick]
	}
	w.detail(lay.Pane, q)
}

// tabs declares the three tab buttons with their counts.
func (w Quests) tabs(lay questsRects, all []game.QuestView) {
	var n [3]int
	for _, q := range all {
		n[questTab(q.State, q.Dismissed)]++
	}
	labels := [3]string{w.sh.Tf("Available (%d)", n[0]), w.sh.Tf("Active (%d)", n[1]), w.sh.Tf("Historical (%d)", n[2])}
	idsOf := [3]string{ids.QuestsAvail, ids.QuestsActive, ids.QuestsHistory}
	for i, r := range lay.Tabs {
		if w.kit().Button(idsOf[i], r, labels[i], uikit.BtnOpts{Selected: w.quests.tab == i}) {
			w.quests.tab = i
		}
	}
}

// questRows are the quests of tab whose title holds search (any case): in
// id order, Historical newest end first.
func questRows(all []game.QuestView, tab int, title func(game.QuestView) string, search string) []game.QuestView {
	search = strings.ToLower(strings.TrimSpace(search))
	var out []game.QuestView
	for _, q := range all {
		if questTab(q.State, q.Dismissed) != tab {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(title(q)), search) {
			continue
		}
		out = append(out, q)
	}
	if tab == questsHistory {
		slices.SortStableFunc(out, func(a, b game.QuestView) int {
			switch {
			case a.EndedAt > b.EndedAt:
				return -1
			case a.EndedAt < b.EndedAt:
				return 1
			}
			return 0
		})
	}
	return out
}

// questPick is the index of quest sel in rows, else the first row (-1: no
// rows).
func questPick(rows []game.QuestView, sel uint32) int {
	if i := slices.IndexFunc(rows, func(q game.QuestView) bool { return uint32(q.ID) == sel }); i >= 0 {
		return i
	}
	if len(rows) > 0 {
		return 0
	}
	return -1
}

// list declares the rows of the tab in a scroll view: a click picks one.
func (w Quests) list(r gfx.Rect, rows []game.QuestView, pick int) {
	k := w.kit()
	k.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, r, uikit.Field)
		uikit.StrokeRect(dst, r, uikit.PanelBorder)
	})
	if len(rows) == 0 {
		ds := uikit.DimText()
		k.Label(r.X+8, r.Y+8, uikit.FitText(w.sh.T("No quests here."), ds, r.W-16), ds)
		return
	}
	in := r.Inset(2)
	area := k.ScrollArea(questsListID, in, float32(len(rows))*hudlayout.QuestRowH, uikit.ScrollOpts{Step: hudlayout.QuestRowH * 2, Focusable: true})
	first, last := uikit.ListVisible(area.Off, area.View.H, hudlayout.QuestRowH, len(rows))
	now := uint64(w.sh.Tick())
	for i := first; i <= last; i++ {
		q := rows[i]
		row := gfx.Rect{X: area.View.X, Y: area.View.Y + float32(i)*hudlayout.QuestRowH - area.Off, W: area.View.W, H: hudlayout.QuestRowH}
		id := ids.QuestRow + fmt.Sprint(q.ID)
		pickIt := func() { w.quests.sel = uint32(q.ID) }
		wd := uikit.Widget{ID: id, Label: questTitle(w.sh, q), R: row.Intersect(area.View), Click: pickIt}
		if k.Add(wd) && k.Press(wd.R) {
			k.OK()
			pickIt()
		}
		hover, selected := k.IsHot(id), i == pick
		title := questTitle(w.sh, q)
		left, timed := questLeft(q, now)
		urgent := questUrgent(q, now)
		k.Queue(func(dst *ebiten.Image) {
			drawQuestRow(gfx.ClipTo(dst, area.View), row, q, questRowText{title, questShort(w.sh, left), timed, urgent}, hover, selected)
		})
	}
}

// questRowText is a row's words, made before the draw.
type questRowText struct {
	title, time   string
	timed, urgent bool
}

// questTint is a Historical row's tint (QST-130): failed in the danger
// band, completed and lapsed in the warning band, none for a dismissed one
// or any other.
func questTint(q game.QuestView) (color.RGBA, bool) {
	if q.Dismissed {
		return color.RGBA{}, false
	}
	switch q.State {
	case qd.Failed:
		return uikit.AlertDangerBg, true
	case qd.Succeeded, qd.Expired:
		return uikit.AlertWarnBg, true
	}
	return color.RGBA{}, false
}

// drawQuestRow draws one 32 px row: the fill (selected, hovered, tinted),
// the name cut before the stars, the stars right-aligned before the time
// cell, and the time left.
func drawQuestRow(dst *ebiten.Image, row gfx.Rect, q game.QuestView, t questRowText, hover, selected bool) {
	switch {
	case selected:
		uikit.FillRect(dst, row, uikit.BtnSelected)
	case hover:
		uikit.FillRect(dst, row, uikit.BtnHover)
	}
	if c, ok := questTint(q); ok {
		tint := row.InsetXY(2, 2)
		gfx.FillRect(dst, tint.X, tint.Y, tint.W, tint.H, c, false)
	}
	timeR := gfx.Rect{X: row.Right() - questTimeW - questRowPad, Y: row.Y, W: questTimeW, H: row.H}
	stars := int(min(q.Stars, 5))
	starsX := timeR.X - float32(stars)*hudlayout.QuestStar
	for i := range stars {
		drawStar(dst, starsX+float32(i)*hudlayout.QuestStar+hudlayout.QuestStar/2, row.Y+row.H/2, hudlayout.QuestStar/2-1)
	}
	ns := gfx.Body
	ns.Color = uikit.Text
	nameW := starsX - row.X - 2*questRowPad
	gfx.Text(dst, uikit.FitText(t.title, ns, nameW), float64(row.X+questRowPad), gfx.CenterTop(float64(row.Y), float64(row.H), ns), ns)
	ts := gfx.Body
	ts.Color, ts.Align = uikit.TextDim, gfx.AlignRight
	if t.urgent {
		ts.Color = uikit.Danger
	}
	if t.timed && q.State != qd.Succeeded {
		gfx.Text(dst, uikit.FitText(t.time, ts, timeR.W), float64(timeR.Right()), gfx.CenterTop(float64(row.Y), float64(row.H), ts), ts)
	}
}

// drawStar fills a five-pointed star of outer radius r round (cx, cy).
func drawStar(dst *ebiten.Image, cx, cy, r float32) {
	gfx.FillPoly(dst, starPoints(cx, cy, r), uikit.TextDim, true)
}

// questTab is the list a quest in state s shows under: a dismissed quest
// and every ended one under Historical (QST-130).
func questTab(s qd.State, dismissed bool) int {
	switch {
	case dismissed || s != qd.Offered && s != qd.Active:
		return questsHistory
	case s == qd.Active:
		return questsActive
	}
	return questsAvail
}
