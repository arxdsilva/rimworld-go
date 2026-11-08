package windows

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/pawn/look"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/command"
)

// The Quests window's detail pane (QST-125..129, 137): the picked quest's
// title with the dismiss / restore button, its time lines, description,
// one reward panel per option with its Accept button, the jump links and
// the giver's foot, in a scroll view that starts at the top for each quest.

// homeMap is the colony's own map (the first map keeps id 0, MAP-0).
const homeMap = game.MapID(0)

// questBtn is a clickable part of the detail, in content coordinates.
type questBtn struct {
	id, label, tip string
	r              gfx.Rect
	run            func()
	link           bool // a text link (else a button)
	icon           int  // questIconDismiss / questIconRestore (0: text)
}

// The dismiss / restore glyphs.
const (
	questIconNone = iota
	questIconDismiss
	questIconRestore
)

// questText is one line of text in content coordinates.
type questText struct {
	s  string
	x  float32 // the left edge, or the right edge for AlignRight
	y  float32
	st gfx.TextStyle
}

// questChip is one reward chip in content coordinates.
type questChip struct {
	r          gfx.Rect
	label, tip string
	plain      bool               // the "worth N" note: no chip frame
	pawn       *quest.PawnPreview // a pawn reward: its portrait and card (QST-137)
}

// questDetail is the detail laid out from y 0 at x 0, contentW wide.
type questDetail struct {
	texts  []questText
	panels []gfx.Rect
	chips  []questChip
	btns   []questBtn
	h      float32
}

// detail declares the pane for quest q (nil: none to show).
func (w Quests) detail(pane gfx.Rect, q *game.QuestView) {
	k := w.kit()
	k.Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, pane, uikit.Field)
		uikit.StrokeRect(dst, pane, uikit.PanelBorder)
	})
	if q == nil {
		ds := uikit.DimText()
		k.Label(pane.X+questPanePad, pane.Y+questPanePad, uikit.FitText(w.sh.T("Pick a quest on the left to read it."), ds, pane.W-2*questPanePad), ds)
		return
	}
	if w.quests.shown != uint32(q.ID) {
		w.quests.shown = uint32(q.ID)
		if k.Scroll != nil {
			k.Scroll[questsDetailID] = 0
		}
	}
	r := gfx.Rect{X: pane.X + questPanePad, Y: pane.Y + questPanePad, W: max(0, pane.W-questPanePad-5), H: max(0, pane.H-2*questPanePad)}
	cw := min(r.W-uikit.ScrollBarW, hudlayout.QuestRewardW)
	d := w.layoutDetail(*q, cw)
	area := k.ScrollArea(questsDetailID, r, d.h, uikit.ScrollOpts{Step: 48, Focusable: true})
	ox, oy := area.View.X, area.View.Y-area.Off
	for _, b := range d.btns {
		br := gfx.Rect{X: b.r.X + ox, Y: b.r.Y + oy, W: b.r.W, H: b.r.H}
		if br.Intersect(area.View) != br { // only a whole button takes clicks
			continue
		}
		run := b.run
		if k.Add(uikit.Widget{ID: b.id, Label: b.label, R: br, Tip: b.tip, Click: run}) && k.Press(br) {
			k.OK()
			run()
		}
	}
	var card func(dst *ebiten.Image)
	for i, c := range d.chips {
		cr := gfx.Rect{X: c.r.X + ox, Y: c.r.Y + oy, W: c.r.W, H: c.r.H}
		full := cr
		if cr = cr.Intersect(area.View); cr.Empty() {
			continue
		}
		id := fmt.Sprintf("quest.chip.%d.%d", q.ID, i)
		switch {
		case c.pawn != nil: // QST-137: the hover card instead of a tooltip
			if k.Add(uikit.Widget{ID: id, Label: c.label, R: cr}) && k.IsHot(id) {
				mx, _ := w.sh.Cursor()
				sw, sh := w.sh.Size()
				box, lines := layoutPawnCard(pawnCard(w.sh, *c.pawn), full, float32(mx), float32(sw), float32(sh))
				card = func(dst *ebiten.Image) { drawPawnCard(dst, box, lines) }
			}
		case c.tip != "":
			k.Tip(id, cr, c.tip)
		}
	}
	hot := func(id string) bool { return k.IsHot(id) }
	k.Queue(func(dst *ebiten.Image) {
		drawQuestDetail(gfx.ClipTo(dst, area.View), d, ox, oy, hot, w.sh.DrawPortrait)
	})
	if card != nil {
		k.Queue(card)
	}
}

// layoutDetail lays quest q's detail out in a column cw wide.
func (w Quests) layoutDetail(q game.QuestView, cw float32) questDetail {
	var d questDetail
	now := uint64(w.sh.Tick())
	id := int(q.ID)
	ts := gfx.Title
	ts.Color = uikit.Heading
	d.texts = append(d.texts, questText{uikit.FitText(questTitle(w.sh, q), ts, cw-questIconW-10), 0, 0, ts})
	if q.Dismissed {
		d.btns = append(d.btns, questBtn{id: ids.QuestRestore + fmt.Sprint(id), label: w.sh.T("Restore"), icon: questIconRestore,
			tip: w.sh.T("Restore\nPut this quest back in its list."), r: gfx.Rect{X: cw - questIconW, W: questIconW, H: questIconH},
			run: func() { w.sh.Enqueue(command.Command{Kind: command.RestoreQuest, Index: id}) }})
	} else {
		d.btns = append(d.btns, questBtn{id: ids.QuestDismiss + fmt.Sprint(id), label: w.sh.T("Dismiss"), icon: questIconDismiss,
			tip: w.sh.T("Dismiss\nMove this quest to Historical. Its timers keep running."), r: gfx.Rect{X: cw - questIconW, W: questIconW, H: questIconH},
			run: func() { w.sh.Enqueue(command.Command{Kind: command.DismissQuest, Index: id}) }})
	}
	y := float32(gfx.LineH(ts)) + 10
	tl := gfx.Body
	tl.Color, tl.Align = uikit.TextDim, gfx.AlignRight
	for _, s := range questTimeLines(w.sh, q, now) {
		d.texts = append(d.texts, questText{uikit.FitText(s, tl, cw), cw, y, tl})
		y += questTimePitch
	}
	y += questBodyGap
	bs := gfx.Body
	bs.Color = uikit.Text
	lh := float32(gfx.LineH(bs))
	facs := w.sh.Factions()
	for _, l := range q.Detail { // QST-127: the arguments in their roles
		rs := questRuns(w.sh, l, q.Giver, now)
		spans, lines := layoutRuns(rs, bs, cw-4)
		for _, sp := range spans {
			st := bs
			st.Color = runColor(rs[sp.run], w.sh.FactionName, facs)
			d.texts = append(d.texts, questText{sp.s, sp.x, y + float32(sp.line)*lh, st})
		}
		y += float32(lines)*lh + questParaGap
	}
	y += questPanelGap
	y = w.layoutRewards(&d, q, cw, y)
	y = w.layoutLinks(&d, q, cw, y)
	d.h = w.layoutFoot(&d, q, cw, y)
	return d
}

// layoutRewards adds the reward panels from y: every option of an offer
// with its Accept button (one bare Accept for a quest without rewards),
// only the chosen one after accepting. It returns the y under them.
func (w Quests) layoutRewards(d *questDetail, q game.QuestView, cw, y float32) float32 {
	offer := q.State == qd.Offered && !q.Dismissed
	opts := make([]int, 0, len(q.Rewards))
	switch {
	case offer:
		for i := range q.Rewards {
			opts = append(opts, i)
		}
	case q.Chosen >= 0 && q.Chosen < len(q.Rewards):
		opts = append(opts, q.Chosen)
	}
	cs := gfx.Body
	for _, o := range opts {
		chips := rewardChips(w.sh, q.Giver, q.Rewards[o])
		if v := q.Rewards[o].Value; v > 0 {
			chips = append(chips, chipText{label: w.sh.Tf("worth %d", v)})
		}
		ws := make([]float32, len(chips))
		for i, c := range chips {
			ws[i] = gfx.TextW(c.label, cs) + 2*questChipTextX
			if c.pawn != nil {
				ws[i] += pawnChipIcon + pawnChipIconGo
			}
		}
		p := layoutReward(ws, 0, y, cw, offer)
		d.panels = append(d.panels, p.Panel)
		for i, r := range p.Chips {
			c := chips[i]
			d.chips = append(d.chips, questChip{r: r, label: c.label, tip: c.tip, pawn: c.pawn, plain: q.Rewards[o].Value > 0 && i == len(chips)-1})
		}
		if offer {
			w.addAccept(d, q, o, p.Accept)
		}
		y += p.Panel.H + questPanelGap
	}
	if offer && len(opts) == 0 {
		p := layoutReward(nil, 0, y, cw, true)
		d.panels = append(d.panels, p.Panel)
		ds := uikit.DimText()
		d.texts = append(d.texts, questText{w.sh.T("No reward"), p.Accept.Right() + questAcceptGap, y + float32(gfx.CenterTop(0, float64(p.Panel.H), ds)), ds})
		w.addAccept(d, q, -1, p.Accept)
		y += p.Panel.H + questPanelGap
	}
	return y
}

// addAccept adds the Accept button of option o (-1: none) in r.
func (w Quests) addAccept(d *questDetail, q game.QuestView, o int, r gfx.Rect) {
	id := int(q.ID)
	tip := w.sh.T("Accept\nTake on this quest for this reward.")
	if o < 0 {
		tip = w.sh.T("Accept\nTake on this quest.")
	}
	d.btns = append(d.btns, questBtn{id: fmt.Sprintf("%s%d.%d", ids.QuestAccept, id, o), label: w.sh.T("Accept for:"), tip: tip, r: r,
		run: func() { w.sh.Enqueue(command.Command{Kind: command.AcceptQuest, Index: id, Count: o}) }})
}

// layoutLinks adds the jump links from y (the site first, the colony
// last, QuestLinkPitch apart) and returns the y under them.
func (w Quests) layoutLinks(d *questDetail, q game.QuestView, cw, y float32) float32 {
	y += hudlayout.QuestLinkPitch - questPanelGap
	ls := gfx.Body
	add := func(id, label, tip string, run func()) {
		lw := min(cw, gfx.TextW(label, ls)+4)
		d.btns = append(d.btns, questBtn{id: id, label: label, tip: tip, link: true, r: gfx.Rect{Y: y, W: lw, H: hudlayout.QuestLinkPitch - 3}, run: run})
		y += hudlayout.QuestLinkPitch
	}
	if t := q.Tile; t >= 0 {
		add(ids.QuestJump+fmt.Sprint(q.ID), w.sh.T("Jump to the site"), w.sh.T("Jump to the site\nShow the quest's place on the world map."), func() {
			w.CloseKit()
			w.sh.Do(ports.OpenWorld{Select: fmt.Sprintf("tile:%d", t)})
		})
	}
	add(ids.QuestColony+fmt.Sprint(q.ID), w.sh.T("Jump to the colony"), w.sh.T("Jump to the colony\nClose this window and show the colony's map."), func() {
		w.CloseKit()
		w.sh.Do(ports.SwitchMap{Map: homeMap})
	})
	return y
}

// layoutFoot adds the giver's foot (name, goodwill, standing, right
// aligned) from y and returns the content's height.
func (w Quests) layoutFoot(d *questDetail, q game.QuestView, cw, y float32) float32 {
	name := w.sh.FactionName(q.Giver)
	if name == "" {
		return y
	}
	y += questFootGap
	ds := uikit.DimText()
	ds.Align = gfx.AlignRight
	d.texts = append(d.texts, questText{uikit.FitText(w.sh.Tf("Offered by %s", name), ds, cw), cw, y, ds})
	y += questFootLineH
	for _, f := range w.sh.Factions() {
		if f.Name != name {
			continue
		}
		d.texts = append(d.texts, questText{uikit.FitText(w.sh.Tf("Goodwill: %s", goodwillText(f.Goodwill)), ds, cw), cw, y, ds})
		y += questFootLineH
		rs := ds
		rs.Color = f.Color
		d.texts = append(d.texts, questText{uikit.FitText(f.Standing, rs, cw), cw, y, rs})
		y += questFootLineH
		break
	}
	return y + questFootLineH
}

// drawQuestDetail paints d with its origin at (ox, oy); dst is clipped to
// the scroll view.
func drawQuestDetail(dst *ebiten.Image, d questDetail, ox, oy float32, hot func(string) bool, portrait portraitFunc) {
	at := func(r gfx.Rect) gfx.Rect { return gfx.Rect{X: r.X + ox, Y: r.Y + oy, W: r.W, H: r.H} }
	for _, p := range d.panels {
		uikit.FillRect(dst, at(p), uikit.Horizon)
		uikit.StrokeRect(dst, at(p), uikit.PanelBorder)
	}
	cs := gfx.Body
	cs.Color = uikit.Text
	for _, c := range d.chips {
		r := at(c.r)
		st := cs
		if c.plain {
			st.Color = uikit.Accent
		} else {
			uikit.FillRect(dst, r, uikit.Btn)
			uikit.StrokeRect(dst, r, uikit.BtnBorder)
		}
		tx := r.X + questChipTextX
		if c.pawn != nil {
			drawChipPortrait(dst, tx, r.Y+(r.H-pawnChipIcon)/2, *c.pawn, portrait)
			tx += pawnChipIcon + pawnChipIconGo
		}
		gfx.Text(dst, uikit.FitText(c.label, st, r.Right()-questChipTextX-tx), float64(tx), gfx.CenterTop(float64(r.Y), float64(r.H), st), st)
	}
	for _, t := range d.texts {
		gfx.Text(dst, t.s, float64(t.x+ox), float64(t.y+oy), t.st)
	}
	for _, b := range d.btns {
		drawQuestBtn(dst, b, at(b.r), hot(b.id))
	}
}

// drawQuestBtn paints a detail button: a text link, a glyph button or a
// plain button with its label fitted inside.
func drawQuestBtn(dst *ebiten.Image, b questBtn, r gfx.Rect, hover bool) {
	if b.link {
		st := gfx.Body
		st.Color = uikit.LetterInfo
		if hover {
			st.Color = uikit.LerpRGBA(uikit.LetterInfo, uikit.Text, 0.4)
		}
		ty := gfx.CenterTop(float64(r.Y), float64(r.H), st)
		gfx.Text(dst, uikit.FitText(b.label, st, r.W), float64(r.X), ty, st)
		if hover {
			gfx.Line(dst, r.X, float32(ty+gfx.LineH(st))-2, r.X+min(r.W, gfx.TextW(b.label, st)), float32(ty+gfx.LineH(st))-2, 1, st.Color, false)
		}
		return
	}
	fill, border, text := uikit.BtnNormal.Colors(hover, false, false)
	uikit.FillRect(dst, r, fill)
	uikit.StrokeRect(dst, r, border)
	if b.icon != questIconNone {
		drawQuestGlyph(dst, r, b.icon, text)
		return
	}
	st := gfx.Label
	st.Color, st.Align = text, gfx.AlignCenter
	cx, _ := r.Center()
	gfx.Text(dst, uikit.FitText(b.label, st, r.W-12), float64(cx), gfx.CenterTop(float64(r.Y), float64(r.H), st), st)
}

// drawQuestGlyph draws the dismiss bin or the restore arrow in r.
func drawQuestGlyph(dst *ebiten.Image, r gfx.Rect, icon int, c color.RGBA) {
	cx, cy := r.Center()
	if icon == questIconDismiss {
		gfx.Line(dst, cx-6, cy-6, cx+6, cy-6, 1.5, c, true) // the lid
		gfx.Line(dst, cx-2, cy-8, cx+2, cy-8, 1.5, c, true) // its handle
		gfx.FillPoly(dst, [][2]float32{{cx - 5, cy - 4}, {cx + 5, cy - 4}, {cx + 4, cy + 7}, {cx - 4, cy + 7}}, c, true)
		return
	}
	gfx.StrokeCircle(dst, cx, cy+1, 5, 1.5, c, true) // a turn back up
	gfx.FillPoly(dst, [][2]float32{{cx - 8, cy - 3}, {cx - 2, cy - 3}, {cx - 5, cy + 2}}, c, true)
}

// portraitFunc draws a look's portrait in a box (Labels.DrawPortrait).
type portraitFunc func(dst *ebiten.Image, l look.Look, x, y, w, h float64) bool

// drawChipPortrait draws p's 16-px portrait at (x, y), or a head and
// shoulders in the dim text colour while the art is missing.
func drawChipPortrait(dst *ebiten.Image, x, y float32, p quest.PawnPreview, portrait portraitFunc) {
	if portrait != nil && portrait(dst, p.Look, float64(x), float64(y), pawnChipIcon, pawnChipIcon) {
		return
	}
	c := uikit.TextDim
	gfx.FillCircle(dst, x+pawnChipIcon/2, y+5, 4, c, true)
	gfx.FillPoly(dst, [][2]float32{{x + 2, y + pawnChipIcon}, {x + 5, y + 10}, {x + 11, y + 10}, {x + 14, y + pawnChipIcon}}, c, true)
}
