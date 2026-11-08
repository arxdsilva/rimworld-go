package windows

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// The Quests window's pure layout (QST-120..129): every rectangle comes
// from the body rect and the hudlayout tokens, so it is tested without a
// kit or a game.

const (
	questUnderGap  = 6  // list -> search field -> preferences button (QST-124)
	questPanePad   = 17 // the detail pane's inner padding (QST-125)
	questTimeW     = 34 // a row's time-left cell (QST-123)
	questRowPad    = 4  // a row's name inset
	questIconW     = 20 // the dismiss / restore button (QST-126)
	questIconH     = 25
	questTimePitch = 22 // the time lines under the title (QST-126)
	questBodyGap   = 12 // time band -> description
	questParaGap   = 8  // between description lines
	questChipH     = 24 // a reward chip (QST-128)
	questChipGap   = 4
	questChipPad   = 10 // the panel's inner top and left padding
	questChipTextX = 6  // a chip's text inset
	questPanelGap  = 10 // between reward panels
	questAcceptGap = 11 // the Accept button -> the first chip
	questFootLineH = 16 // the giver foot's line pitch (QST-129)
	questFootGap   = 25 // the last link -> the foot
)

// questsRects is the window's regions.
type questsRects struct {
	Tabs   [3]gfx.Rect
	List   gfx.Rect // the scrolling rows, under the tabs
	Search gfx.Rect
	Prefs  gfx.Rect
	Pane   gfx.Rect // the detail pane's frame
}

// questsLayout cuts the body (the frame less WindowPad): the list column
// QuestsListW wide on the left (tabs, rows, search, the preferences
// button), QuestsGap, then the detail pane in the rest, at most
// QuestsDetailW. A short window shrinks the rows first.
func questsLayout(body gfx.Rect) questsRects {
	var r questsRects
	col := gfx.Rect{X: body.X, Y: body.Y, W: min(hudlayout.QuestsListW, body.W), H: body.H}
	tw := col.W / 3
	for i := range r.Tabs {
		r.Tabs[i] = gfx.Rect{X: col.X + float32(i)*tw, Y: col.Y, W: tw, H: hudlayout.QuestsTabH}
	}
	r.Prefs = gfx.Rect{X: col.X, Y: col.Bottom() - hudlayout.QuestPrefsBtnH, W: col.W, H: hudlayout.QuestPrefsBtnH}
	sh := questSearchH()
	r.Search = gfx.Rect{X: col.X, Y: r.Prefs.Y - questUnderGap - sh, W: col.W, H: sh}
	top := col.Y + hudlayout.QuestsTabH
	r.List = gfx.Rect{X: col.X, Y: top, W: col.W, H: max(0, r.Search.Y-questUnderGap-top)}
	px := col.Right() + hudlayout.QuestsGap
	r.Pane = gfx.Rect{X: px, Y: body.Y, W: max(0, min(hudlayout.QuestsDetailW, body.Right()-px)), H: body.H}
	return r
}

// questSearchH is the search field's height: QuestSearchH, grown until
// the field's inner rect holds a line of body text (the kit clips the
// text to it, so a short field cuts the letters).
func questSearchH() float32 {
	lh := float32(gfx.LineH(gfx.Body))
	in := uikit.FieldInner(gfx.Rect{H: hudlayout.QuestSearchH})
	return hudlayout.QuestSearchH + max(0, lh-in.H)
}

// rewardPanelH is a reward panel's height for n chip lines: the measured
// QuestRewardH1..3, then one chip pitch more per line.
func rewardPanelH(n int) float32 {
	switch {
	case n <= 1:
		return hudlayout.QuestRewardH1
	case n == 2:
		return hudlayout.QuestRewardH2
	case n == 3:
		return hudlayout.QuestRewardH3
	}
	return hudlayout.QuestRewardH3 + float32(n-3)*(questChipH+questChipGap)
}

// layoutChips places chips of widths ws left to right from x, wrapping at
// right, questChipH tall and questChipGap apart, the first line at y. A
// chip wider than the line is cut to it. It returns the chip rects and the
// number of lines (at least 1).
func layoutChips(ws []float32, x, y, right float32) ([]gfx.Rect, int) {
	out := make([]gfx.Rect, len(ws))
	cx, line := x, 0
	for i, w := range ws {
		w = min(w, right-x)
		if cx > x && cx+w > right {
			cx, line = x, line+1
		}
		out[i] = gfx.Rect{X: cx, Y: y + float32(line)*(questChipH+questChipGap), W: w, H: questChipH}
		cx += w + questChipGap
	}
	return out, line + 1
}

// rewardPanel is one "Accept for" option laid out: the panel, its Accept
// button (zero when the quest takes no choice now) and its chips.
type rewardPanel struct {
	Panel, Accept gfx.Rect
	Chips         []gfx.Rect
}

// layoutReward lays out an option with chip widths ws in a panel at (x, y)
// QuestRewardW wide (cut to w): with an Accept button (an offer) the chips
// start questAcceptGap right of it, else questChipPad in.
func layoutReward(ws []float32, x, y, w float32, accept bool) rewardPanel {
	pw := min(hudlayout.QuestRewardW, w)
	cx := x + questChipPad
	if accept {
		cx = x + hudlayout.QuestAcceptW + questAcceptGap
	}
	chips, lines := layoutChips(ws, cx, y+questChipPad, x+pw-questChipPad)
	p := rewardPanel{Panel: gfx.Rect{X: x, Y: y, W: pw, H: rewardPanelH(lines)}, Chips: chips}
	if accept {
		p.Accept = gfx.Rect{X: x, Y: y, W: hudlayout.QuestAcceptW, H: p.Panel.H}
	}
	return p
}

// starPoints are a five-pointed star's ten corners round (cx, cy), the
// outer ones at radius r, the first at the top.
func starPoints(cx, cy, r float32) [][2]float32 {
	pts := make([][2]float32, 10)
	for i := range pts {
		rad := float64(r)
		if i%2 == 1 {
			rad *= 0.45
		}
		a := -math.Pi/2 + float64(i)*math.Pi/5
		pts[i] = [2]float32{cx + float32(rad*math.Cos(a)), cy + float32(rad*math.Sin(a))}
	}
	return pts
}
