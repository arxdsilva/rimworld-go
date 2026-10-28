package wizard

import (
	"fmt"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	SparkDays    = 60
	sparkSeed    = 0x5eed
	sparkBlockH  = 104
)

// saveModeInfo is a save-mode choice.
type saveModeInfo struct {
	id, Label, Text string
	mode            difficulty.SaveMode
}

var SaveModes = []saveModeInfo{
	{"save.reload", "Reload anytime", "Save and load as often as you like. Good for learning and experimenting.", difficulty.ReloadAnytime},
	{"save.commit", "Commitment", "One save per colony that updates itself. You live with every decision. This can't be changed later.", difficulty.Commitment},
}

// ModeCaption is a one-line feel of a pacing model.
func ModeCaption(m storyteller.Mode) string {
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

// PacingOf is the example pacing of a storyteller: a fixed showcase seed,
// so it never changes between frames or runs.
func PacingOf(p storyteller.Profile) []int { return storyteller.Spark(p, SparkDays, sparkSeed) }

// storyLayout is the three columns of the screen.
type storyLayout struct{ Left, Centre, Right gfx.Rect }

func StoryRects(content gfx.Rect) storyLayout {
	left := gfx.Rect{X: content.X, Y: content.Y, W: storyLeftW, H: content.H}
	right := gfx.Rect{X: content.Right() - storyRightW, Y: content.Y, W: storyRightW, H: content.H}
	cx := left.Right() + uikit.DialogPad
	return storyLayout{left, gfx.Rect{X: cx, Y: content.Y, W: right.X - uikit.DialogPad - cx, H: content.H}, right}
}

func (w storyPage) drawStoryteller(screen *ebiten.Image, sw, sh int) {
	w.drawNGFrame(screen, sw, sh, "Choose Storyteller")
}

func (w storyPage) storytellerUI(sw, sh int) {
	uikit.OnScreenLayer(w.kit())
	if w.kit().SetupTooSmall(sw, sh) {
		return
	}
	ng := w.draft()
	_, _, content, footer := uikit.DialogFrame(sw, sh)
	L := StoryRects(content)
	profiles := storyteller.Profiles()
	cur, _ := storyteller.ByKey(ng.Storyteller)

	for i, p := range profiles {
		p := p
		r := gfx.Rect{X: L.Left.X, Y: L.Left.Y + float32(i)*(storyCardH+storyCardGap), W: storyLeftW, H: storyCardH}
		pick := func() { ng.Storyteller = p.Key }
		wd := uikit.Widget{ID: "story.pick." + p.Key, R: r, Label: p.Name, Focusable: true, Click: func() { w.kit().OK(); pick() },
			Tip: p.Name + ", " + p.Title + "\n" + ModeCaption(p.Mode)}
		u := w.kit()
		if u.Add(wd) && (w.kit().Press(r) || w.kit().KeyAct(wd.ID)) {
			w.kit().OK()
			pick()
		}
		selected := ng.Storyteller == p.Key
		u.Queue(func(dst *ebiten.Image) { w.paintStoryCard(dst, r, p, selected, u.Hot == wd.ID) })
	}

	w.storyCentre(L.Centre, cur)
	w.storyRight(ng, L.Right)

	w.ngFooter(footer, "Next", ids.WizardNext, uikit.BtnOpts{}, func() {
		s := w.sh.Settings()
		s.Gameplay.LastStoryteller = ng.Storyteller
		s.Gameplay.LastDifficulty = ng.Preset.Key()
		s.Gameplay.LastSaveMode = ng.SaveMode.String()
		w.sh.SaveSettings(s)
		w.forward()
	})
}

// portraitImage is a storyteller's bust (large or thumbnail) from the art
// catalog, or nil while the art is missing.
func (w storyPage) portraitImage(key string, thumb bool) *ebiten.Image {
	suffix := ""
	if thumb {
		suffix = "_thumb"
	}
	return w.art().Image("ui_portrait/"+key+suffix, noTint)
}

// drawFit draws img inside box keeping its aspect, centred.
func drawFit(dst, img *ebiten.Image, box gfx.Rect) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := min(float64(box.W)/iw, float64(box.H)/ih)
	gfx.Image(dst, img, float64(box.X)+(float64(box.W)-iw*s)/2, float64(box.Y)+(float64(box.H)-ih*s)/2,
		iw*s, ih*s, &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear})
}

// DrawSilhouette is the stand-in for a missing portrait: a head and
// shoulders in the panel colours, with the name under it.
func DrawSilhouette(dst *ebiten.Image, box gfx.Rect, name string) {
	uikit.FillRect(dst, box, uikit.Field)
	uikit.StrokeRect(dst, box, uikit.PanelBorder)
	cx := box.X + box.W/2
	c := uikit.PanelBorder
	gfx.FillCircle(dst, cx, box.Y+box.H*0.36, box.W*0.2, c, true)
	gfx.FillCircle(dst, cx, box.Y+box.H*0.95, box.W*0.42, c, true)
	st := gfx.Small
	st.Align, st.Color = gfx.AlignCenter, uikit.TextDim
	gfx.Text(dst, uikit.FitText(name, st, box.W-8), float64(cx), float64(box.Bottom())-gfx.LineH(st)-4, st)
}

// paintStoryCard draws a storyteller's list card.
func (w storyPage) paintStoryCard(dst *ebiten.Image, r gfx.Rect, p storyteller.Profile, selected, hot bool) {
	fill := uikit.Panel
	switch {
	case selected:
		fill = uikit.LerpRGBA(uikit.Panel, uikit.BtnSelected, 0.8)
	case hot:
		fill = uikit.LerpRGBA(uikit.Panel, uikit.BtnHover, 0.8)
	}
	uikit.FillRect(dst, r, fill)
	if selected {
		gfx.StrokeRect(dst, r.X, r.Y, r.W, r.H, 2, uikit.Accent, false)
	} else {
		uikit.StrokeRect(dst, r, uikit.PanelBorder)
	}
	pr := gfx.Rect{X: r.X + 2, Y: r.Y + 2, W: 120, H: r.H - 4}
	if img := w.portraitImage(p.Key, true); img != nil {
		drawFit(dst, img, pr)
	} else {
		DrawSilhouette(dst, pr, p.Name)
	}
	tx := float64(r.X + 134)
	nm := gfx.Large
	nm.Color = uikit.Heading
	y := float64(r.Y) + 20
	for _, l := range gfx.WrapPx(p.Name, nm, float64(r.W-142)) {
		gfx.Text(dst, l, tx, y, nm)
		y += gfx.LineH(nm)
	}
	ts := gfx.Small
	ts.Color = uikit.TextDim
	gfx.Text(dst, p.Title, tx, y+2, ts)
	gfx.Text(dst, ModeCaption(p.Mode), tx, y+2+gfx.LineH(ts)+8, ts)
}

// storyCentre is the chosen storyteller: portrait, name, bio and the
// example pacing.
func (w storyPage) storyCentre(r gfx.Rect, p storyteller.Profile) {
	u := w.kit()
	w.kit().Panel(r)
	in := r.Inset(12)
	ph := max(float32(180), min(float32(330), in.H*0.46))
	pr := gfx.Rect{X: in.X, Y: in.Y, W: ph * 0.8, H: ph}
	text := gfx.Rect{X: pr.Right() + 16, Y: in.Y, W: in.Right() - pr.Right() - 16, H: ph}
	u.Queue(func(dst *ebiten.Image) {
		if img := w.portraitImage(p.Key, false); img != nil {
			drawFit(dst, img, pr)
		} else {
			DrawSilhouette(dst, pr, p.Name)
		}
		nm := gfx.Title
		nm.Color = uikit.Heading
		y := float64(text.Y)
		for _, l := range gfx.WrapPx(p.Name, nm, float64(text.W)) {
			gfx.Text(dst, l, float64(text.X), y, nm)
			y += gfx.LineH(nm)
		}
		ts := gfx.Large
		ts.Color = uikit.TextDim
		gfx.Text(dst, p.Title, float64(text.X), y+2, ts)
		cs := gfx.Body
		cs.Color = uikit.Accent
		gfx.Text(dst, "Pacing: "+ModeCaption(p.Mode), float64(text.X), y+2+gfx.LineH(ts)+10, cs)
	})
	w.kit().Tip("story.mode", gfx.Rect{X: text.X, Y: text.Y, W: text.W, H: 90}, p.Name+"\n"+PacingTip(p))

	// The bio, between the portrait and the pacing chart.
	bioY := pr.Bottom() + 10
	bio := gfx.Rect{X: in.X, Y: bioY, W: in.W, H: max(0, in.Bottom()-sparkBlockH-bioY)}
	paras := strings.Split(p.Bio, "\n\n")
	bw := max(40, bio.W-uikit.ScrollBarW-6)
	total := float32(0)
	for _, para := range paras {
		total += wrapHeight(para, gfx.Body, bw) + 8
	}
	area := w.kit().ScrollArea("story.bio", bio, total, uikit.ScrollOpts{Step: 34})
	u.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		y := area.View.Y - area.Off
		for _, para := range paras {
			for _, l := range gfx.WrapPx(para, gfx.Body, float64(bw)) {
				gfx.Text(sub, l, float64(area.View.X), float64(y), gfx.Body)
				y += float32(gfx.LineH(gfx.Body))
			}
			y += 8
		}
	})

	chart := gfx.Rect{X: in.X, Y: in.Bottom() - sparkBlockH + 6, W: in.W, H: sparkBlockH - 6}
	vals := PacingOf(p)
	w.kit().Tip("story.pacing", chart, "Example pacing\nSixty days of one possible story. Each bar is a day; taller bars are bigger threats and gaps are quiet days.")
	u.Queue(func(dst *ebiten.Image) { drawPacing(dst, chart, vals) })
}

// PacingTip describes how a storyteller paces threats, for the tooltip.
func PacingTip(p storyteller.Profile) string {
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
func drawPacing(dst *ebiten.Image, r gfx.Rect, vals []int) {
	cs := gfx.Small
	cs.Color = uikit.TextDim
	gfx.Text(dst, "Example pacing — every colony differs.", float64(r.X), float64(r.Y), cs)
	chart := gfx.Rect{X: r.X, Y: r.Y + float32(gfx.LineH(cs)) + 6, W: r.W, H: r.H - float32(gfx.LineH(cs)) - 6 - float32(gfx.LineH(cs)) - 2}
	uikit.FillRect(dst, chart, uikit.Field)
	uikit.StrokeRect(dst, chart, uikit.PanelBorder)
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
		gfx.FillRect(dst, chart.X+float32(i)*bw+1, chart.Bottom()-3-h, max(1, bw-2), h, uikit.Accent, false)
	}
	for d := 0; d <= SparkDays; d += 10 {
		x := chart.X + float32(d)*bw
		ls := gfx.Small
		ls.Color = uikit.TextDim
		ls.Align = gfx.AlignCenter
		switch d {
		case 0:
			ls.Align = gfx.AlignLeft
		case SparkDays:
			ls.Align = gfx.AlignRight
		}
		gfx.Text(dst, fmt.Sprintf("day %d", d), float64(x), float64(chart.Bottom())+2, ls)
	}
}

// ---- difficulty and save mode ------------------------------------------------

// PresetTip is the key numbers of a preset.
func PresetTip(n difficulty.Named) string {
	s := n.Settings
	return fmt.Sprintf("%s\nThreat scale %d%%. Crops %d%%, mining %d%%, research %d%%. Mood %+d. Infection %d%%. Peaceful days %d.",
		n.Label, s.ThreatScale, s.CropYield, s.MineYield, s.ResearchSpeed, s.MoodOffset, s.Infection, s.PeacefulDays)
}

// storyRight is the difficulty and save-mode column.
func (w storyPage) storyRight(ng *Draft, r gfx.Rect) {
	u := w.kit()
	w.kit().Panel(r)
	in := r.Inset(10)
	presets := difficulty.Presets()
	hs := gfx.Large
	hs.Color = uikit.Heading
	headH := float32(gfx.LineH(hs)) + 4
	sd := gfx.Small
	noteW := in.W - uikit.ScrollBarW - 6
	note := "Storyteller and difficulty can be changed later in Options. Save mode can't."
	noteH := wrapHeight(note, gfx.Small, noteW)

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
		rows = append(rows, srow{id: "diff." + n.Key, label: n.Label, desc: n.Desc, tip: PresetTip(n), y: y, h: diffRowH,
			selected: ng.Preset == n.Preset, click: func() { w.pickPreset(n.Preset) }})
		y += diffRowH
	}
	customTip := "Custom\nSet every value yourself in the Custom Difficulty dialog."
	rows = append(rows, srow{id: "diff.custom", label: "Custom", desc: "Set every value yourself.", tip: customTip, y: y, h: diffRowH,
		selected: ng.Preset == difficulty.Custom, click: func() { w.pickPreset(difficulty.Custom) }})
	y += diffRowH + 10
	saveHeadY := y
	y += headH
	for _, m := range SaveModes {
		m := m
		h := float32(gfx.LineH(gfx.Label)) + wrapHeight(m.Text, sd, noteW-26) + 8
		rows = append(rows, srow{id: m.id, label: m.Label, desc: m.Text, y: y, h: h, selected: ng.SaveMode == m.mode,
			click: func() { ng.SaveMode = m.mode }})
		y += h + saveRowGap
	}
	noteY := y
	y += noteH + 4

	area := w.kit().ScrollArea("story.right", in, y, uikit.ScrollOpts{Step: 48})
	shown := func(sr srow) gfx.Rect {
		return gfx.Rect{X: area.View.X, Y: area.View.Y + sr.y - area.Off, W: area.View.W, H: sr.h}
	}

	// The Customize button comes first so it takes its click before the row.
	for _, sr := range rows {
		if sr.id != "diff.custom" || ng.Preset != difficulty.Custom {
			continue
		}
		rr := shown(sr)
		btn := gfx.Rect{X: rr.Right() - 104, Y: rr.Y + 8, W: 100, H: 24}
		if btn.Y >= area.View.Y && btn.Bottom() <= area.View.Bottom() {
			uikit.StyledButton(w.kit(), "diff.customize", btn, "Customize...", uikit.BtnOpts{Tip: "Customize\nEdit every difficulty value."}, w.openCustomDifficulty)
		}
	}
	for _, sr := range rows {
		sr := sr
		hit := shown(sr).Intersect(area.View)
		if hit.Empty() {
			continue
		}
		wd := uikit.Widget{ID: sr.id, R: hit, Label: sr.label, Tip: sr.tip, Focusable: true, Click: func() { w.kit().OK(); sr.click() }}
		if u.Add(wd) && (w.kit().Press(hit) || w.kit().KeyAct(sr.id)) {
			w.kit().OK()
			sr.click()
		}
	}
	u.Queue(func(dst *ebiten.Image) {
		sub := gfx.ClipTo(dst, area.View)
		top := area.View.Y - area.Off
		gfx.Text(sub, "Difficulty", float64(area.View.X+2), float64(top)+2, hs)
		gfx.Text(sub, "Save mode", float64(area.View.X+2), float64(top+saveHeadY)+2, hs)
		ns := gfx.Small
		ns.Color = uikit.TextDim
		for i, l := range gfx.WrapPx(note, ns, float64(noteW)) {
			gfx.Text(sub, l, float64(area.View.X+2), float64(top+noteY)+float64(i)*gfx.LineH(ns), ns)
		}
		for _, sr := range rows {
			rr := shown(sr)
			if rr.Bottom() < area.View.Y || rr.Y > area.View.Bottom() {
				continue
			}
			hot := u.Hot == sr.id
			switch {
			case sr.selected:
				uikit.FillRect(sub, rr.Inset(1), uikit.LerpRGBA(uikit.Panel, uikit.BtnSelected, 0.8))
				uikit.StrokeRect(sub, rr.Inset(1), uikit.Accent)
			case hot:
				uikit.FillRect(sub, rr.Inset(1), uikit.BtnHover)
			}
			cx, cy := rr.X+16, rr.Y+12
			gfx.FillCircle(sub, cx, cy, uikit.RadioRadius, uikit.Field, true)
			gfx.StrokeCircle(sub, cx, cy, uikit.RadioRadius, 1, uikit.FieldBorder, true)
			if sr.selected {
				gfx.FillCircle(sub, cx, cy, uikit.RadioRadius-3, uikit.Accent, true)
			}
			gfx.Text(sub, sr.label, float64(rr.X+32), float64(rr.Y)+3, gfx.Label)
			ds := gfx.Small
			ds.Color = uikit.TextDim
			lines := gfx.WrapPx(sr.desc, ds, float64(rr.W-40))
			if sr.id[:4] == "diff" {
				lines = lines[:min(len(lines), 1)]
			}
			for i, l := range lines {
				gfx.Text(sub, l, float64(rr.X+32), float64(rr.Y)+2+gfx.LineH(gfx.Label)+float64(i)*gfx.LineH(ds), ds)
			}
		}
	})
}

// pickPreset selects a difficulty; Custom opens its dialog the first time.
func (w storyPage) pickPreset(p difficulty.Preset) {
	ng := w.draft()
	if p == difficulty.Custom && !ng.CustomOpen {
		w.openCustomDifficulty()
		return
	}
	ng.Preset = p
}
