package options

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/storyteller"
)

// Options › Story (NG-1 §2.4, §2.12): change the storyteller and the
// difficulty during play with the same widgets as Choose Storyteller, and see
// the save mode (read-only: it is fixed when the colony starts). Changes go
// out as the typed commands SetStoryteller and SetDifficulty. The tab exists
// only while a colony is running.

const (
	stColLeftW = 330
	stColMidW  = 400
	stColGap   = 16
	stCardH    = 118
	stCardGap  = 10
	stHeadH    = 30
	stDiffRowH = 44
	// StCustomID is the Custom difficulty row; StCustomMod the dialog's id
	// and StFormID its scroll area's.
	StCustomID  = "opt.diff.custom"
	StCustomMod = "opt.story.custom"
	StFormID    = "modal.opt.story.scroll"
)

// UI declares the Story tab's widgets (the other tabs are legacy buttons).
func (o *Screen) UI(sw, sh int) {
	if !o.sh.Env().Running || o.CurrentTab() != "story" {
		return
	}
	uikit.OnScreenLayer(o.sh.Kit())
	o.storyTab(sw, sh)
}

// setStoryteller and setDifficulty are the tab's typed commands, applied
// at once: the colony is not stepping while Options is open, and the tab
// shows the new values at once.
func (o *Screen) setStoryteller(key string) {
	o.sh.Send(command.Command{Kind: command.SetStoryteller, Name: key})
}

func (o *Screen) setDifficulty(name string) {
	o.sh.Send(command.Command{Kind: command.SetDifficulty, Name: name})
}

// customName is the SetDifficulty name of custom settings.
func customName(s difficulty.Settings) string {
	return "custom:" + difficulty.Encode(difficulty.Clamp(s))
}

// storyCols splits the content area into the three columns.
func storyCols(c gfx.Rect) (left, mid, right gfx.Rect) {
	left = gfx.Rect{X: c.X, Y: c.Y, W: stColLeftW, H: c.H}
	mid = gfx.Rect{X: left.Right() + stColGap, Y: c.Y, W: stColMidW, H: c.H}
	right = gfx.Rect{X: mid.Right() + stColGap, Y: c.Y, W: max(0, c.Right()-mid.Right()-stColGap), H: c.H}
	return
}

func (o *Screen) storyTab(sw, sh int) {
	v := o.sh.StoryView()
	panel := gfx.Rect{X: 40, Y: 96, W: float32(sw - 80), H: float32(sh - 180)}
	top := float32(optTabY + OptBtnH + 16)
	c := gfx.Rect{X: panel.X + 16, Y: top, W: panel.W - 32, H: panel.Bottom() - top - 16}
	if v.Preset == "" {
		uikit.TextLines(o.sh.Kit(), c.X, c.Y, c.W, "This colony started without a storyteller (the classic start), so there is nothing to change here.", uikit.DimText())
		return
	}
	left, mid, right := storyCols(c)
	o.storytellerColumn(left, v)
	o.difficultyColumn(mid, v)
	o.saveModeColumn(right, v)
}

func headingStyle() gfx.TextStyle {
	hs := gfx.Large
	hs.Color = uikit.Heading
	return hs
}

// storytellerColumn lists the storytellers as compact cards.
func (o *Screen) storytellerColumn(r gfx.Rect, v game.StoryView) {
	u := o.sh.Kit()
	u.Label(r.X, r.Y, "Storyteller", headingStyle())
	for i, p := range storyteller.Profiles() {
		cr := gfx.Rect{X: r.X, Y: r.Y + stHeadH + float32(i)*(stCardH+stCardGap), W: r.W, H: stCardH}
		if cr.Bottom() > r.Bottom() {
			break
		}
		id := "opt.story.pick." + p.Key
		pick := func() { o.setStoryteller(p.Key) }
		w := uikit.Widget{ID: id, R: cr, Label: p.Name, Focusable: true, Click: func() { u.OK(); pick() },
			Tip: p.Name + ", " + p.Title + "\n" + o.sh.CardTip(p)}
		if u.Add(w) && (u.Press(cr) || u.KeyAct(id)) {
			u.OK()
			pick()
		}
		selected := v.Storyteller == p.Name
		u.Queue(func(dst *ebiten.Image) { o.sh.PaintCard(dst, cr, p, selected, u.Hot == id) })
	}
}

// difficultyColumn is the difficulty radio and the Customize button.
func (o *Screen) difficultyColumn(r gfx.Rect, v game.StoryView) {
	u := o.sh.Kit()
	u.Label(r.X, r.Y, "Difficulty", headingStyle())
	cur, known := difficulty.Parse(v.Preset)
	if !known {
		cur = difficulty.NumPresets // none selected
	}
	type row struct {
		id, label, desc, tip string
		preset               difficulty.Preset
		r                    gfx.Rect
	}
	var rows []row
	y := r.Y + stHeadH
	for _, n := range difficulty.Presets() {
		rows = append(rows, row{"opt.diff." + n.Key, n.Label, n.Desc, o.sh.PresetTip(n), n.Preset, gfx.Rect{X: r.X, Y: y, W: r.W, H: stDiffRowH}})
		y += stDiffRowH
	}
	rows = append(rows, row{StCustomID, "Custom", "Every value set by hand.", "Custom\nSet every value yourself in the Custom Difficulty dialog.", difficulty.Custom, gfx.Rect{X: r.X, Y: y, W: r.W, H: stDiffRowH}})

	// The Customize button first, so it takes its click before the row.
	if cur == difficulty.Custom {
		cr := rows[len(rows)-1].r
		uikit.StyledButton(u, "opt.diff.customize", gfx.Rect{X: cr.Right() - 112, Y: cr.Y + 10, W: 104, H: 24}, "Customize...",
			uikit.BtnOpts{Tip: "Customize\nEdit every difficulty value."}, func() { o.OpenStoryCustom(v) })
	}
	for _, rw := range rows {
		click := func() {
			if rw.preset == difficulty.Custom {
				o.OpenStoryCustom(v)
				return
			}
			o.setDifficulty(rw.preset.Key())
		}
		w := uikit.Widget{ID: rw.id, R: rw.r, Label: rw.label, Tip: rw.tip, Focusable: true, Click: func() { u.OK(); click() }}
		if u.Add(w) && (u.Press(rw.r) || u.KeyAct(rw.id)) {
			u.OK()
			click()
		}
		selected := rw.preset == cur
		u.Queue(func(dst *ebiten.Image) { paintDiffRow(dst, rw.r, rw.label, rw.desc, selected, u.Hot == rw.id) })
	}
}

// paintDiffRow draws one difficulty radio row.
func paintDiffRow(dst *ebiten.Image, r gfx.Rect, label, desc string, selected, hot bool) {
	switch {
	case selected:
		uikit.FillRect(dst, r.Inset(1), uikit.LerpRGBA(uikit.Panel, uikit.BtnSelected, 0.8))
		uikit.StrokeRect(dst, r.Inset(1), uikit.Accent)
	case hot:
		uikit.FillRect(dst, r.Inset(1), uikit.BtnHover)
	}
	cx, cy := r.X+16, r.Y+r.H/2
	gfx.FillCircle(dst, cx, cy, uikit.RadioRadius, uikit.Field, true)
	gfx.StrokeCircle(dst, cx, cy, uikit.RadioRadius, 1, uikit.FieldBorder, true)
	if selected {
		gfx.FillCircle(dst, cx, cy, uikit.RadioRadius-3, uikit.Accent, true)
	}
	gfx.Text(dst, label, float64(r.X+32), float64(r.Y)+5, gfx.Label)
	ds := gfx.Small
	ds.Color = uikit.TextDim
	gfx.Text(dst, uikit.FitText(desc, ds, r.W-40), float64(r.X+32), float64(r.Y)+5+gfx.LineH(gfx.Label), ds)
}

// saveModeColumn shows the colony's save mode, read-only, and a summary of
// the story so far.
func (o *Screen) saveModeColumn(r gfx.Rect, v game.StoryView) {
	k := o.sh.Kit()
	k.Label(r.X, r.Y, "Save mode", headingStyle())
	label, text := o.sh.SaveMode(v.Commitment)
	y := r.Y + stHeadH
	k.Label(r.X, y, label, gfx.Label)
	y += float32(gfx.LineH(gfx.Label)) + 2
	y += uikit.TextLines(k, r.X, y, r.W, text, uikit.DimText()) + 8
	ns := gfx.Small
	ns.Color = uikit.Warning
	y += uikit.TextLines(k, r.X, y, r.W, "Fixed when the colony started. It can't be changed.", ns) + 24

	k.Label(r.X, y, "Now", headingStyle())
	y += stHeadH
	for _, l := range StorySummary(v) {
		y += uikit.TextLines(k, r.X, y, r.W, l, gfx.Body) + 4
	}
}

// StorySummary is the "Now" lines: who tells the story, how hard, and what
// the adaptation currently does to threat sizes.
func StorySummary(v game.StoryView) []string {
	out := []string{fmt.Sprintf("%s, %s.", v.Storyteller, v.Title), fmt.Sprintf("Difficulty: %s.", v.Preset)}
	if v.Difficulty.Adaptation {
		out = append(out, fmt.Sprintf("Adaptation: threats are %+d%% in size from how the colony has fared.", v.Adaptation))
	}
	return out
}

// OpenStoryCustom opens the Custom difficulty dialog on the colony's
// current values; accepting it sends SetDifficulty.
func (o *Screen) OpenStoryCustom(v game.StoryView) {
	o.sh.OpenCustom(v, func(s difficulty.Settings) { o.setDifficulty(customName(s)) })
}
