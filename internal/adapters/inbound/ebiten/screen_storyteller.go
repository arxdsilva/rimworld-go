package ebiten

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/storyteller"
)

// Choose Storyteller (NG-1 §2.4): three independent choices on one screen:
// who paces the story, how hard it is, and whether the player can reload.

// Layout metrics.
const (
	storyLeftW   = 300
	storyRightW  = 360
	storyCardH   = 150
	storyCardGap = 10
	diffRowH     = 36
	saveRowGap   = 4
	sparkDays    = 60
	sparkSeed    = 0x5eed
	sparkBlockH  = 104
)

// saveModeInfo is a save-mode choice.
type saveModeInfo struct {
	id, Label, Text string
	mode            difficulty.SaveMode
}

var saveModes = []saveModeInfo{
	{"save.reload", "Reload anytime", "Save and load as often as you like. Good for learning and experimenting.", difficulty.ReloadAnytime},
	{"save.commit", "Commitment", "One save per colony that updates itself. You live with every decision. This can't be changed later.", difficulty.Commitment},
}

// modeCaption is a one-line feel of a pacing model.
func modeCaption(m storyteller.Mode) string {
	switch m {
	case storyteller.Cycle:
		return "Hard weeks, then rest"
	case storyteller.Calm:
		return "Slow, gentle, forgiving"
	case storyteller.Dice:
		return "Any day, any size"
	}
	return "The old rhythm"
}

// pacingOf is the example pacing of a storyteller: a fixed showcase seed,
// so it never changes between frames or runs.
func pacingOf(p storyteller.Profile) []int { return storyteller.Spark(p, sparkDays, sparkSeed) }

// storyLayout is the three columns of the screen.
type storyLayout struct{ Left, Centre, Right rect }

func storyRects(content rect) storyLayout {
	left := rect{X: content.X, Y: content.Y, W: storyLeftW, H: content.H}
	right := rect{X: content.Right() - storyRightW, Y: content.Y, W: storyRightW, H: content.H}
	cx := left.Right() + dialogPad
	return storyLayout{left, rect{X: cx, Y: content.Y, W: right.X - dialogPad - cx, H: content.H}, right}
}

func (a *App) drawStoryteller(screen *ebiten.Image, sw, sh int) {
	a.drawNGFrame(screen, sw, sh, "Choose Storyteller")
}

// storytellerButtons: the screen's controls are kit widgets (layoutHits).
func (a *App) storytellerButtons(sw, sh int) []button { return nil }

func (a *App) storytellerUI(sw, sh int) {
	a.declareOnScreenLayer()
	if a.setupTooSmall(sw, sh) {
		return
	}
	ng := a.draft()
	_, _, content, footer := dialogFrame(sw, sh)
	L := storyRects(content)
	profiles := storyteller.Profiles()
	cur, _ := storyteller.ByKey(ng.Storyteller)

	for i, p := range profiles {
		p := p
		r := rect{X: L.Left.X, Y: L.Left.Y + float32(i)*(storyCardH+storyCardGap), W: storyLeftW, H: storyCardH}
		pick := func() { ng.Storyteller = p.Key }
		w := uiWidget{ID: "story.pick." + p.Key, R: r, Label: p.Name, Focusable: true, Click: func() { a.uiOK(); pick() },
			Tip: p.Name + ", " + p.Title + "\n" + modeCaption(p.Mode)}
		u := a.uiSt()
		if u.Add(w) && (a.uiPress(r) || a.uiKeyAct(w.ID)) {
			a.uiOK()
			pick()
		}
		selected := ng.Storyteller == p.Key
		u.Queue(func(dst *ebiten.Image) { a.paintStoryCard(dst, r, p, selected, u.Hot == w.ID) })
	}

	a.storyCentre(L.Centre, cur)
	a.storyRight(ng, L.Right)

	a.ngFooter(footer, "Next", ids.WizardNext, btnOpts{}, func() {
		s := a.settings
		s.Gameplay.LastStoryteller = ng.Storyteller
		s.Gameplay.LastDifficulty = ng.Preset.Key()
		s.Gameplay.LastSaveMode = ng.SaveMode.String()
		a.setSettings(s)
		a.forward()
	})
}

// portraitImage is a storyteller's bust (large or thumbnail) from the art
// catalog, or nil while the art is missing.
func (a *App) portraitImage(key string, thumb bool) *ebiten.Image {
	suffix := ""
	if thumb {
		suffix = "_thumb"
	}
	return a.art.Image("ui_portrait/"+key+suffix, noTint)
}

// drawFit draws img inside box keeping its aspect, centred.
func drawFit(dst, img *ebiten.Image, box rect) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := min(float64(box.W)/iw, float64(box.H)/ih)
	uiImage(dst, img, float64(box.X)+(float64(box.W)-iw*s)/2, float64(box.Y)+(float64(box.H)-ih*s)/2,
		iw*s, ih*s, &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear})
}

// drawSilhouette is the stand-in for a missing portrait: a head and
// shoulders in the panel colours, with the name under it.
func drawSilhouette(dst *ebiten.Image, box rect, name string) {
	fillRect(dst, box, uiField)
	strokeRect(dst, box, uiPanelBorder)
	cx := box.X + box.W/2
	c := uiPanelBorder
	uiFillCircle(dst, cx, box.Y+box.H*0.36, box.W*0.2, c, true)
	uiFillCircle(dst, cx, box.Y+box.H*0.95, box.W*0.42, c, true)
	st := tsSmall
	st.Align, st.Color = alignCenter, uiTextDim
	drawText(dst, fitText(name, st, box.W-8), float64(cx), float64(box.Bottom())-lineH(st)-4, st)
}

// paintStoryCard draws a storyteller's list card.
func (a *App) paintStoryCard(dst *ebiten.Image, r rect, p storyteller.Profile, selected, hot bool) {
	fill := uiPanel
	switch {
	case selected:
		fill = lerpRGBA(uiPanel, uiBtnSelected, 0.8)
	case hot:
		fill = lerpRGBA(uiPanel, uiBtnHover, 0.8)
	}
	fillRect(dst, r, fill)
	if selected {
		uiStrokeRect(dst, r.X, r.Y, r.W, r.H, 2, uiAccent, false)
	} else {
		strokeRect(dst, r, uiPanelBorder)
	}
	pr := rect{X: r.X + 2, Y: r.Y + 2, W: 120, H: r.H - 4}
	if img := a.portraitImage(p.Key, true); img != nil {
		drawFit(dst, img, pr)
	} else {
		drawSilhouette(dst, pr, p.Name)
	}
	tx := float64(r.X + 134)
	nm := tsLarge
	nm.Color = uiHeading
	y := float64(r.Y) + 20
	for _, l := range wrapPx(p.Name, nm, float64(r.W-142)) {
		drawText(dst, l, tx, y, nm)
		y += lineH(nm)
	}
	ts := tsSmall
	ts.Color = uiTextDim
	drawText(dst, p.Title, tx, y+2, ts)
	drawText(dst, modeCaption(p.Mode), tx, y+2+lineH(ts)+8, ts)
}

// storyCentre is the chosen storyteller: portrait, name, bio and the
// example pacing.
func (a *App) storyCentre(r rect, p storyteller.Profile) {
	u := a.uiSt()
	a.uiPanel(r)
	in := r.Inset(12)
	ph := max(float32(180), min(float32(330), in.H*0.46))
	pr := rect{X: in.X, Y: in.Y, W: ph * 0.8, H: ph}
	text := rect{X: pr.Right() + 16, Y: in.Y, W: in.Right() - pr.Right() - 16, H: ph}
	u.Queue(func(dst *ebiten.Image) {
		if img := a.portraitImage(p.Key, false); img != nil {
			drawFit(dst, img, pr)
		} else {
			drawSilhouette(dst, pr, p.Name)
		}
		nm := tsTitle
		nm.Color = uiHeading
		y := float64(text.Y)
		for _, l := range wrapPx(p.Name, nm, float64(text.W)) {
			drawText(dst, l, float64(text.X), y, nm)
			y += lineH(nm)
		}
		ts := tsLarge
		ts.Color = uiTextDim
		drawText(dst, p.Title, float64(text.X), y+2, ts)
		cs := tsBody
		cs.Color = uiAccent
		drawText(dst, "Pacing: "+modeCaption(p.Mode), float64(text.X), y+2+lineH(ts)+10, cs)
	})
	a.uiTip("story.mode", rect{X: text.X, Y: text.Y, W: text.W, H: 90}, p.Name+"\n"+pacingTip(p))

	// The bio, between the portrait and the pacing chart.
	bioY := pr.Bottom() + 10
	bio := rect{X: in.X, Y: bioY, W: in.W, H: max(0, in.Bottom()-sparkBlockH-bioY)}
	paras := strings.Split(p.Bio, "\n\n")
	bw := max(40, bio.W-scrollBarW-6)
	total := float32(0)
	for _, para := range paras {
		total += wrapHeight(para, tsBody, bw) + 8
	}
	area := a.uiScrollArea("story.bio", bio, total, scrollOpts{Step: 34})
	u.Queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		y := area.View.Y - area.Off
		for _, para := range paras {
			for _, l := range wrapPx(para, tsBody, float64(bw)) {
				drawText(sub, l, float64(area.View.X), float64(y), tsBody)
				y += float32(lineH(tsBody))
			}
			y += 8
		}
	})

	chart := rect{X: in.X, Y: in.Bottom() - sparkBlockH + 6, W: in.W, H: sparkBlockH - 6}
	vals := pacingOf(p)
	a.uiTip("story.pacing", chart, "Example pacing\nSixty days of one possible story. Each bar is a day; taller bars are bigger threats and gaps are quiet days.")
	u.Queue(func(dst *ebiten.Image) { drawPacing(dst, chart, vals) })
}

// pacingTip describes how a storyteller paces threats, for the tooltip.
func pacingTip(p storyteller.Profile) string {
	switch p.Mode {
	case storyteller.Cycle:
		return fmt.Sprintf("%d days of pressure, then %d days of rest. First big threat around day %d.", p.CycleOn, p.CycleOff, p.FirstThreatDay)
	case storyteller.Calm:
		return fmt.Sprintf("One big threat every %d to %d days, with more good news between. First around day %d.", p.GapMin, p.GapMax, p.FirstThreatDay)
	case storyteller.Dice:
		return fmt.Sprintf("Each morning rolls the dice for a big threat. First possible on day %d.", p.FirstThreatDay)
	}
	return ""
}

// drawPacing draws daily threat bars with a caption.
func drawPacing(dst *ebiten.Image, r rect, vals []int) {
	cs := tsSmall
	cs.Color = uiTextDim
	drawText(dst, "Example pacing — every colony differs.", float64(r.X), float64(r.Y), cs)
	chart := rect{X: r.X, Y: r.Y + float32(lineH(cs)) + 6, W: r.W, H: r.H - float32(lineH(cs)) - 6 - float32(lineH(cs)) - 2}
	fillRect(dst, chart, uiField)
	strokeRect(dst, chart, uiPanelBorder)
	hi := 1
	for _, v := range vals {
		hi = max(hi, v)
	}
	n := max(len(vals), 1)
	bw := chart.W / float32(n)
	for i, v := range vals {
		if v <= 0 {
			continue
		}
		h := (chart.H - 6) * float32(v) / float32(hi)
		uiFillRect(dst, chart.X+float32(i)*bw+1, chart.Bottom()-3-h, max(1, bw-2), h, uiAccent, false)
	}
	for d := 0; d <= sparkDays; d += 10 {
		x := chart.X + float32(d)*bw
		ls := tsSmall
		ls.Color = uiTextDim
		ls.Align = alignCenter
		switch d {
		case 0:
			ls.Align = alignLeft
		case sparkDays:
			ls.Align = alignRight
		}
		drawText(dst, fmt.Sprintf("day %d", d), float64(x), float64(chart.Bottom())+2, ls)
	}
}

// ---- difficulty and save mode ------------------------------------------------

// presetTip is the key numbers of a preset.
func presetTip(n difficulty.Named) string {
	s := n.Settings
	return fmt.Sprintf("%s\nThreat scale %d%%. Crops %d%%, mining %d%%, research %d%%. Mood %+d. Infection %d%%. Peaceful days %d.",
		n.Label, s.ThreatScale, s.CropYield, s.MineYield, s.ResearchSpeed, s.MoodOffset, s.Infection, s.PeacefulDays)
}

// storyRight is the difficulty and save-mode column.
func (a *App) storyRight(ng *newGameState, r rect) {
	u := a.uiSt()
	a.uiPanel(r)
	in := r.Inset(10)
	presets := difficulty.Presets()
	hs := tsLarge
	hs.Color = uiHeading
	headH := float32(lineH(hs)) + 4
	sd := tsSmall
	noteW := in.W - scrollBarW - 6
	note := "Storyteller and difficulty can be changed later in Options. Save mode can't."
	noteH := wrapHeight(note, tsSmall, noteW)

	type srow struct {
		id, label, desc, tip string
		y, h                 float32
		selected             bool
		click                func()
	}
	var rows []srow
	y := headH
	for _, n := range presets {
		n := n
		rows = append(rows, srow{id: "diff." + n.Key, label: n.Label, desc: n.Desc, tip: presetTip(n), y: y, h: diffRowH,
			selected: ng.Preset == n.Preset, click: func() { a.pickPreset(n.Preset) }})
		y += diffRowH
	}
	customTip := "Custom\nSet every value yourself in the Custom Difficulty dialog."
	rows = append(rows, srow{id: "diff.custom", label: "Custom", desc: "Set every value yourself.", tip: customTip, y: y, h: diffRowH,
		selected: ng.Preset == difficulty.Custom, click: func() { a.pickPreset(difficulty.Custom) }})
	y += diffRowH + 10
	saveHeadY := y
	y += headH
	for _, m := range saveModes {
		m := m
		h := float32(lineH(tsLabel)) + wrapHeight(m.Text, sd, noteW-26) + 8
		rows = append(rows, srow{id: m.id, label: m.Label, desc: m.Text, y: y, h: h, selected: ng.SaveMode == m.mode,
			click: func() { ng.SaveMode = m.mode }})
		y += h + saveRowGap
	}
	noteY := y
	y += noteH + 4

	area := a.uiScrollArea("story.right", in, y, scrollOpts{Step: 48})
	shown := func(sr srow) rect {
		return rect{X: area.View.X, Y: area.View.Y + sr.y - area.Off, W: area.View.W, H: sr.h}
	}

	// The Customize button comes first so it takes its click before the row.
	for _, sr := range rows {
		if sr.id != "diff.custom" || ng.Preset != difficulty.Custom {
			continue
		}
		rr := shown(sr)
		btn := rect{X: rr.Right() - 104, Y: rr.Y + 8, W: 100, H: 24}
		if btn.Y >= area.View.Y && btn.Bottom() <= area.View.Bottom() {
			a.ngButton("diff.customize", btn, "Customize...", btnOpts{Tip: "Customize\nEdit every difficulty value."}, a.openCustomDifficulty)
		}
	}
	for _, sr := range rows {
		sr := sr
		hit := shown(sr).Intersect(area.View)
		if hit.Empty() {
			continue
		}
		w := uiWidget{ID: sr.id, R: hit, Label: sr.label, Tip: sr.tip, Focusable: true, Click: func() { a.uiOK(); sr.click() }}
		if u.Add(w) && (a.uiPress(hit) || a.uiKeyAct(sr.id)) {
			a.uiOK()
			sr.click()
		}
	}
	u.Queue(func(dst *ebiten.Image) {
		sub := clipTo(dst, area.View)
		top := area.View.Y - area.Off
		drawText(sub, "Difficulty", float64(area.View.X+2), float64(top)+2, hs)
		drawText(sub, "Save mode", float64(area.View.X+2), float64(top+saveHeadY)+2, hs)
		ns := tsSmall
		ns.Color = uiTextDim
		for i, l := range wrapPx(note, ns, float64(noteW)) {
			drawText(sub, l, float64(area.View.X+2), float64(top+noteY)+float64(i)*lineH(ns), ns)
		}
		for _, sr := range rows {
			rr := shown(sr)
			if rr.Bottom() < area.View.Y || rr.Y > area.View.Bottom() {
				continue
			}
			hot := u.Hot == sr.id
			switch {
			case sr.selected:
				fillRect(sub, rr.Inset(1), lerpRGBA(uiPanel, uiBtnSelected, 0.8))
				strokeRect(sub, rr.Inset(1), uiAccent)
			case hot:
				fillRect(sub, rr.Inset(1), uiBtnHover)
			}
			cx, cy := rr.X+16, rr.Y+12
			uiFillCircle(sub, cx, cy, radioRadius, uiField, true)
			uiStrokeCircle(sub, cx, cy, radioRadius, 1, uiFieldBorder, true)
			if sr.selected {
				uiFillCircle(sub, cx, cy, radioRadius-3, uiAccent, true)
			}
			drawText(sub, sr.label, float64(rr.X+32), float64(rr.Y)+3, tsLabel)
			ds := tsSmall
			ds.Color = uiTextDim
			lines := wrapPx(sr.desc, ds, float64(rr.W-40))
			if sr.id[:4] == "diff" {
				lines = lines[:min(len(lines), 1)]
			}
			for i, l := range lines {
				drawText(sub, l, float64(rr.X+32), float64(rr.Y)+2+lineH(tsLabel)+float64(i)*lineH(ds), ds)
			}
		}
	})
}

// pickPreset selects a difficulty; Custom opens its dialog the first time.
func (a *App) pickPreset(p difficulty.Preset) {
	ng := a.draft()
	if p == difficulty.Custom && !ng.CustomOpen {
		a.openCustomDifficulty()
		return
	}
	ng.Preset = p
}
