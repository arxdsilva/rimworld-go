package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
	stColLeftW  = 330
	stColMidW   = 400
	stColGap    = 16
	stCardH     = 118
	stCardGap   = 10
	stHeadH     = 30
	stDiffRowH  = 44
	stCustomID  = "opt.diff.custom"
	stCustomMod = "opt.story.custom"
	stFormID    = "modal.opt.story.scroll"
)

// optionsUI declares the Story tab's widgets (the other tabs are legacy
// buttons).
func (a *App) optionsUI(sw, sh int) {
	if a.renderer == nil || a.currentOptionsTab() != "story" {
		return
	}
	a.declareOnScreenLayer()
	a.storyTab(sw, sh)
}

// sendNow sends a command and applies it at once: the colony is not
// stepping while Options is open, and the tab shows the new values at once.
func (a *App) sendNow(c command.Command) {
	if a.renderer == nil {
		return
	}
	a.shell.Enqueue(c) // the renderer's queue: applies at once while paused, and the lessons see it
	if p, ok := a.renderer.game.(pendingApplier); ok {
		p.ApplyPending()
	}
}

// setStoryteller and setDifficulty are the tab's typed commands.
func (a *App) setStoryteller(key string) {
	a.sendNow(command.Command{Kind: command.SetStoryteller, Name: key})
}

func (a *App) setDifficulty(name string) {
	a.sendNow(command.Command{Kind: command.SetDifficulty, Name: name})
}

// customName is the SetDifficulty name of custom settings.
func customName(s difficulty.Settings) string {
	return "custom:" + difficulty.Encode(difficulty.Clamp(s))
}

// storyCols splits the content area into the three columns.
func storyCols(c rect) (left, mid, right rect) {
	left = rect{X: c.X, Y: c.Y, W: stColLeftW, H: c.H}
	mid = rect{X: left.Right() + stColGap, Y: c.Y, W: stColMidW, H: c.H}
	right = rect{X: mid.Right() + stColGap, Y: c.Y, W: max(0, c.Right()-mid.Right()-stColGap), H: c.H}
	return
}

func (a *App) storyTab(sw, sh int) {
	v := a.renderer.game.Story()
	panel := rect{X: 40, Y: 96, W: float32(sw - 80), H: float32(sh - 180)}
	top := float32(optTabY + optBtnH + 16)
	c := rect{X: panel.X + 16, Y: top, W: panel.W - 32, H: panel.Bottom() - top - 16}
	if v.Preset == "" {
		a.textLines(c.X, c.Y, c.W, "This colony started without a storyteller (the classic start), so there is nothing to change here.", dimText())
		return
	}
	left, mid, right := storyCols(c)
	a.storytellerColumn(left, v)
	a.difficultyColumn(mid, v)
	a.saveModeColumn(right, v)
}

func headingStyle() textStyle {
	hs := tsLarge
	hs.Color = uiHeading
	return hs
}

// storytellerColumn lists the storytellers as compact cards.
func (a *App) storytellerColumn(r rect, v game.StoryView) {
	u := a.uiSt()
	a.uiLabel(r.X, r.Y, "Storyteller", headingStyle())
	for i, p := range storyteller.Profiles() {
		p := p
		cr := rect{X: r.X, Y: r.Y + stHeadH + float32(i)*(stCardH+stCardGap), W: r.W, H: stCardH}
		if cr.Bottom() > r.Bottom() {
			break
		}
		id := "opt.story.pick." + p.Key
		pick := func() { a.setStoryteller(p.Key) }
		w := uiWidget{id: id, r: cr, label: p.Name, focusable: true, click: func() { a.uiOK(); pick() },
			tip: p.Name + ", " + p.Title + "\n" + modeCaption(p.Mode) + ". " + pacingTip(p)}
		if u.add(w) && (a.uiPress(cr) || a.uiKeyAct(id)) {
			a.uiOK()
			pick()
		}
		selected := v.Storyteller == p.Name
		u.queue(func(dst *ebiten.Image) { a.paintStoryCard(dst, cr, p, selected, u.hot == id) })
	}
}

// difficultyColumn is the difficulty radio and the Customize button.
func (a *App) difficultyColumn(r rect, v game.StoryView) {
	u := a.uiSt()
	a.uiLabel(r.X, r.Y, "Difficulty", headingStyle())
	cur, known := difficulty.Parse(v.Preset)
	if !known {
		cur = difficulty.NumPresets // none selected
	}
	type row struct {
		id, label, desc, tip string
		preset               difficulty.Preset
		r                    rect
	}
	var rows []row
	y := r.Y + stHeadH
	for _, n := range difficulty.Presets() {
		rows = append(rows, row{"opt.diff." + n.Key, n.Label, n.Desc, presetTip(n), n.Preset, rect{X: r.X, Y: y, W: r.W, H: stDiffRowH}})
		y += stDiffRowH
	}
	rows = append(rows, row{stCustomID, "Custom", "Every value set by hand.", "Custom\nSet every value yourself in the Custom Difficulty dialog.", difficulty.Custom, rect{X: r.X, Y: y, W: r.W, H: stDiffRowH}})

	// The Customize button first, so it takes its click before the row.
	if cur == difficulty.Custom {
		cr := rows[len(rows)-1].r
		a.ngButton("opt.diff.customize", rect{X: cr.Right() - 112, Y: cr.Y + 10, W: 104, H: 24}, "Customize...",
			btnOpts{Tip: "Customize\nEdit every difficulty value."}, func() { a.openStoryCustom(v) })
	}
	for _, rw := range rows {
		rw := rw
		click := func() {
			if rw.preset == difficulty.Custom {
				a.openStoryCustom(v)
				return
			}
			a.setDifficulty(rw.preset.Key())
		}
		w := uiWidget{id: rw.id, r: rw.r, label: rw.label, tip: rw.tip, focusable: true, click: func() { a.uiOK(); click() }}
		if u.add(w) && (a.uiPress(rw.r) || a.uiKeyAct(rw.id)) {
			a.uiOK()
			click()
		}
		selected := rw.preset == cur
		u.queue(func(dst *ebiten.Image) { paintDiffRow(dst, rw.r, rw.label, rw.desc, selected, u.hot == rw.id) })
	}
}

// paintDiffRow draws one difficulty radio row.
func paintDiffRow(dst *ebiten.Image, r rect, label, desc string, selected, hot bool) {
	switch {
	case selected:
		fillRect(dst, r.Inset(1), lerpRGBA(uiPanel, uiBtnSelected, 0.8))
		strokeRect(dst, r.Inset(1), uiAccent)
	case hot:
		fillRect(dst, r.Inset(1), uiBtnHover)
	}
	cx, cy := r.X+16, r.Y+r.H/2
	uiFillCircle(dst, cx, cy, radioRadius, uiField, true)
	uiStrokeCircle(dst, cx, cy, radioRadius, 1, uiFieldBorder, true)
	if selected {
		uiFillCircle(dst, cx, cy, radioRadius-3, uiAccent, true)
	}
	drawText(dst, label, float64(r.X+32), float64(r.Y)+5, tsLabel)
	ds := tsSmall
	ds.Color = uiTextDim
	drawText(dst, fitText(desc, ds, r.W-40), float64(r.X+32), float64(r.Y)+5+lineH(tsLabel), ds)
}

// saveModeColumn shows the colony's save mode, read-only, and a summary of
// the story so far.
func (a *App) saveModeColumn(r rect, v game.StoryView) {
	a.uiLabel(r.X, r.Y, "Save mode", headingStyle())
	mode := saveModes[0]
	if v.Commitment {
		mode = saveModes[1]
	}
	y := r.Y + stHeadH
	a.uiLabel(r.X, y, mode.label, tsLabel)
	y += float32(lineH(tsLabel)) + 2
	y += a.textLines(r.X, y, r.W, mode.text, dimText()) + 8
	ns := tsSmall
	ns.Color = uiWarning
	y += a.textLines(r.X, y, r.W, "Fixed when the colony started. It can't be changed.", ns) + 24

	a.uiLabel(r.X, y, "Now", headingStyle())
	y += stHeadH
	for _, l := range storySummary(v) {
		y += a.textLines(r.X, y, r.W, l, tsBody) + 4
	}
}

// storySummary is the "Now" lines: who tells the story, how hard, and what
// the adaptation currently does to threat sizes.
func storySummary(v game.StoryView) []string {
	out := []string{fmt.Sprintf("%s, %s.", v.Storyteller, v.Title), fmt.Sprintf("Difficulty: %s.", v.Preset)}
	if v.Difficulty.Adaptation {
		out = append(out, fmt.Sprintf("Adaptation: threats are %+d%% in size from how the colony has fared.", v.Adaptation))
	}
	return out
}

// ---- the Custom difficulty dialog, in the colony ------------------------------

// openStoryCustom opens the dialog on the colony's current values.
func (a *App) openStoryCustom(v game.StoryView) {
	from, ok := difficulty.Parse(v.Preset)
	if !ok || from == difficulty.Custom {
		from = difficulty.Seasoned
	}
	cd := customDiff{s: v.Difficulty, from: from}
	formH := new(float32)
	a.uiPushModal(modal{
		id: stCustomMod, title: "Custom difficulty", w: diffModalW, h: diffModalH,
		draw: func(a *App, body rect) { a.storyCustomBody(body, &cd, formH) },
		buttons: []modalButton{
			{id: "accept", label: "Accept", style: btnPrimary, closes: true, onClick: func() { a.setDifficulty(customName(cd.s)) }},
			{id: "reset", label: "Reset", onClick: func() { cd.s = difficulty.Of(cd.from) }},
			{id: "cancel", label: "Cancel", closes: true},
		},
	})
}

// storyCustomBody is customDifficultyBody (screen_difficulty.go) over the
// dialog's own working copy instead of the wizard's draft.
func (a *App) storyCustomBody(body rect, cd *customDiff, formH *float32) {
	top, rest := body.SplitV(diffTopRowH)
	lab := dimText()
	a.uiLabel(top.X, top.Y+(top.H-float32(lineH(lab)))/2, "Start from:", lab)
	names := make([]string, 0, int(difficulty.Custom))
	for _, n := range difficulty.Presets() {
		names = append(names, n.Label)
	}
	a.ngChoice("diff.from", rect{X: top.X + 90, Y: top.Y + 2, W: 170, H: top.H - 4}, "Start from", names, int(cd.from), "", func(i int) { cd.from = difficulty.Preset(i) })
	a.ngButton("diff.copy", rect{X: top.X + 272, Y: top.Y + 2, W: 150, H: top.H - 4}, "Copy", btnOpts{Tip: "Copy\nCopy every value of the preset into Custom."},
		func() { cd.s = difficulty.Of(cd.from) })

	area := a.uiScrollArea(stFormID, rest, *formH, scrollOpts{Step: 40})
	colW := (area.View.W - 8 - diffColGap) / 2
	left := &formCtx{view: area.View, x: area.View.X + 4, w: colW, off: area.Off}
	right := &formCtx{view: area.View, x: area.View.X + 4 + colW + diffColGap, w: colW, off: area.Off}
	s := &cd.s

	left.sectionTitle(a, "Threats")
	a.diffSlider(left, "ts", "Threat scale", &s.ThreatScale, "ts", pctText, "", "Threat scale\nHow big raids and packs are, as a share of the normal size.")
	a.diffCheck(left, "bt", "Big threats", &s.BigThreats, "", "Big threats\nRaids and packs. Off leaves only small events.")
	a.diffStepper(left, "pd", "Peaceful start (days)", &s.PeacefulDays, 0, 30, "Peaceful start\nDays at the start with no big threat.")
	a.diffCheck(left, "ph", "Predators hunt colonists", &s.PredatorsHuntColonists, "", "Predators\nWild predators may attack colonists, not only prey.")
	a.diffSlider(left, "ar", "Hostile wildlife revenge", &s.AnimalRevenge, "ar", pctText, "", "Revenge\nHow fiercely wild animals fight back when hurt.")
	left.gap(8)

	left.sectionTitle(a, "Escalation")
	a.diffCheck(left, "ad", "Adaptation", &s.Adaptation, "", "Adaptation\nThreats grow while the colony loses nobody, and ease after a death.")
	adaptOff := ""
	if !s.Adaptation {
		adaptOff = "Only used with adaptation"
	}
	a.diffSlider(left, "as", "Adaptation strength", &s.AdaptStrength, "as", pctText, adaptOff, "Adaptation strength\nHow far adaptation can move the threat size.")
	a.diffCheck(left, "te", "Time-based escalation", &s.TimeEscalation, "", "Time-based escalation\nThreats also grow with the years that pass.")
	if !s.TimeEscalation {
		s.IgnoreWealth = false
	}
	timeOff := ""
	if !s.TimeEscalation {
		timeOff = disabledNote
	}
	a.diffSlider(left, "ep", "Escalation per year", &s.EscPerYear, "ep", pctText, timeOff, "Escalation per year\nHow much the threat size grows each year.")
	a.diffCheck(left, "iw", "Ignore wealth", &s.IgnoreWealth, timeOff, "Ignore wealth\nThreat size follows time and people only, not what you own.")

	right.sectionTitle(a, "Economy")
	a.diffSlider(right, "cy", "Crop yield", &s.CropYield, "cy", pctText, "", "")
	a.diffSlider(right, "my", "Mining yield", &s.MineYield, "my", pctText, "", "")
	a.diffSlider(right, "by", "Butchering yield", &s.ButcherYield, "by", pctText, "", "")
	a.diffSlider(right, "rs", "Research speed", &s.ResearchSpeed, "rs", pctText, "", "")
	right.gap(8)

	right.sectionTitle(a, "Health and mind")
	a.diffSlider(right, "mo", "Mood offset", &s.MoodOffset, "mo", signedText, "", "Mood offset\nAdded to every colonist's mood.")
	a.diffSlider(right, "if", "Infection chance", &s.Infection, "if", pctText, "", "")
	a.diffSlider(right, "df", "Disease frequency", &s.DiseaseFreq, "df", pctText, "", diseaseFreqTip)
	if row, ok := right.row(18); ok {
		ns := tsSmall
		ns.Color = uiTextDim
		a.uiLabel(row.X, row.Y, diseaseFreqNote, ns)
	}
	a.diffSlider(right, "pt", "Pain tolerance offset", &s.PainTolerance, "pt", signedText, "", "")
	right.gap(8)

	right.sectionTitle(a, "Nature")
	a.diffCheck(right, "ew", "Extreme weather", &s.ExtremeWeather, "", "Extreme weather\nHeat waves and cold snaps.")

	*formH = max(left.content(), right.content())
}
