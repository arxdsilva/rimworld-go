package ebiten

import (
	"fmt"

	"rimworld-go/internal/domain/difficulty"
)

// Custom Difficulty (NG-1 §2.5): a modal over Choose Storyteller with
// every difficulty value as a slider or a toggle, in two columns of
// groups. It edits a working copy; Accept stores it in the draft as the
// Custom preset, Cancel (and Esc, and the X) drops it.

// customDiff is the dialog's working copy.
type customDiff struct {
	s    difficulty.Settings
	from difficulty.Preset // the preset Copy and Reset start from
}

const (
	diffModalW   = 900
	diffModalH   = 640
	diffSliderH  = 40
	diffCheckH   = 28
	diffHeadH    = 26
	diffColGap   = 28
	diffTopRowH  = 36
	diffFormID   = "modal.diff.scroll"
	disabledNote = "Only used with time-based escalation"
)

// openCustomDifficulty opens the dialog on the current values: the Custom
// settings when Custom is selected, else the selected preset's.
func (a *App) openCustomDifficulty() {
	ng := a.draft()
	from := ng.preset
	if from == difficulty.Custom {
		from = difficulty.Seasoned
		ng.cd = customDiff{s: ng.custom, from: from}
	} else {
		ng.cd = customDiff{s: difficulty.Of(ng.preset), from: from}
	}
	a.uiPushModal(modal{
		id: "diff.custom", title: "Custom difficulty", w: diffModalW, h: diffModalH,
		draw: func(a *App, body rect) { a.customDifficultyBody(body) },
		buttons: []modalButton{
			{id: "accept", label: "Accept", style: btnPrimary, closes: true, onClick: a.acceptCustomDifficulty},
			{id: "reset", label: "Reset", onClick: func() { a.draft().cd.s = difficulty.Of(a.draft().cd.from) }},
			{id: "cancel", label: "Cancel", closes: true},
		},
	})
}

// acceptCustomDifficulty stores the working copy as the Custom preset.
func (a *App) acceptCustomDifficulty() {
	ng := a.draft()
	ng.custom = difficulty.Clamp(ng.cd.s)
	ng.preset = difficulty.Custom
	ng.customOpen = true
}

// customDifficultyBody declares the form: the "Start from" row, then the
// groups in two columns inside one scroll view.
func (a *App) customDifficultyBody(body rect) {
	ng := a.draft()
	cd := &ng.cd
	top, rest := body.splitV(diffTopRowH)

	lab := dimText()
	a.uiLabel(top.X, top.Y+(top.H-float32(lineH(lab)))/2, "Start from:", lab)
	names := make([]string, 0, int(difficulty.Custom))
	for _, n := range difficulty.Presets() {
		names = append(names, n.Label)
	}
	a.ngChoice("diff.from", rect{top.X + 90, top.Y + 2, 170, top.H - 4}, "Start from", names, int(cd.from), "", func(i int) { cd.from = difficulty.Preset(i) })
	a.ngButton("diff.copy", rect{top.X + 272, top.Y + 2, 150, top.H - 4}, "Copy", btnOpts{Tip: "Copy\nCopy every value of the preset into Custom."},
		func() { cd.s = difficulty.Of(cd.from) })

	area := a.uiScrollArea(diffFormID, rest, ng.formHeight(diffFormID), scrollOpts{Step: 40})
	colW := (area.View.W - 8 - diffColGap) / 2
	left := &formCtx{view: area.View, x: area.View.X + 4, w: colW, off: area.Off}
	right := &formCtx{view: area.View, x: area.View.X + 4 + colW + diffColGap, w: colW, off: area.Off}
	s := &cd.s

	// Threats.
	left.sectionTitle(a, "Threats")
	a.diffSlider(left, "ts", "Threat scale", &s.ThreatScale, "ts", pctText, "", "Threat scale\nHow big raids and packs are, as a share of the normal size.")
	a.diffCheck(left, "bt", "Big threats", &s.BigThreats, "", "Big threats\nRaids and packs. Off leaves only small events.")
	a.diffStepper(left, "pd", "Peaceful start (days)", &s.PeacefulDays, 0, 30, "Peaceful start\nDays at the start with no big threat.")
	a.diffCheck(left, "ph", "Predators hunt colonists", &s.PredatorsHuntColonists, "", "Predators\nWild predators may attack colonists, not only prey.")
	a.diffSlider(left, "ar", "Hostile wildlife revenge", &s.AnimalRevenge, "ar", pctText, "", "Revenge\nHow fiercely wild animals fight back when hurt.")
	left.gap(8)

	// Escalation.
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

	// Economy.
	right.sectionTitle(a, "Economy")
	a.diffSlider(right, "cy", "Crop yield", &s.CropYield, "cy", pctText, "", "")
	a.diffSlider(right, "my", "Mining yield", &s.MineYield, "my", pctText, "", "")
	a.diffSlider(right, "by", "Butchering yield", &s.ButcherYield, "by", pctText, "", "")
	a.diffSlider(right, "rs", "Research speed", &s.ResearchSpeed, "rs", pctText, "", "")
	right.gap(8)

	// Health and mind.
	right.sectionTitle(a, "Health and mind")
	a.diffSlider(right, "mo", "Mood offset", &s.MoodOffset, "mo", signedText, "", "Mood offset\nAdded to every colonist's mood.")
	a.diffSlider(right, "if", "Infection chance", &s.Infection, "if", pctText, "", "")
	a.diffSlider(right, "df", "Disease frequency", &s.DiseaseFreq, "df", pctText, "", "Disease frequency\nNo diseases yet — takes effect when they arrive.")
	if row, ok := right.row(18); ok {
		ns := tsSmall
		ns.Color = uiTextDim
		a.uiLabel(row.X, row.Y, "No diseases yet — takes effect when they arrive.", ns)
	}
	a.diffSlider(right, "pt", "Pain tolerance offset", &s.PainTolerance, "pt", signedText, "", "")
	right.gap(8)

	// Nature.
	right.sectionTitle(a, "Nature")
	a.diffCheck(right, "ew", "Extreme weather", &s.ExtremeWeather, "", "Extreme weather\nHeat waves and cold snaps.")

	ng.setFormHeight(diffFormID, max(left.content(), right.content()))
}

func pctText(v int) string    { return fmt.Sprintf("%d%%", v) }
func signedText(v int) string { return fmt.Sprintf("%+d", v) }

// diffSlider is a labelled slider row over *v with the range of the
// difficulty field key. A reason disables it (dimmed, with the reason as
// its tooltip).
func (a *App) diffSlider(f *formCtx, id, label string, v *int, key string, format func(int) string, disabled, tip string) {
	row, ok := f.row(diffSliderH)
	if !ok {
		return
	}
	lim, _ := difficulty.Limit(key)
	ls := tsBody
	vs := tsLabel
	vs.Align = alignRight
	if disabled != "" {
		ls.Color, vs.Color = uiTextDisabled, uiTextDisabled
	}
	a.uiLabel(row.X, row.Y+1, label, ls)
	a.uiLabel(row.right(), row.Y+1, format(*v), vs)
	sr := rect{row.X + 2, row.Y + 20, row.W - 4, 18}
	a.uiSliderInt("diff.f."+id, sr, v, sliderSpec{
		Min: float64(lim.Min), Max: float64(lim.Max), Step: stepFor(lim), Title: label,
		Format:   func(x float64) string { return format(int(x)) },
		Disabled: disabled != "", Reason: disabled,
	})
	if tip != "" && disabled == "" {
		a.uiTip("diff.t."+id, rect{row.X, row.Y, row.W, 18}, tip)
	}
}

// stepFor is a slider's key step: 5 for wide ranges, 1 for narrow ones.
func stepFor(l difficulty.Limits) float64 {
	if l.Max-l.Min >= 100 {
		return 5
	}
	return 1
}

// diffCheck is a toggle row; a reason disables it.
func (a *App) diffCheck(f *formCtx, id, label string, v *bool, disabled, tip string) {
	row, ok := f.row(diffCheckH)
	if !ok {
		return
	}
	a.uiCheck("diff.f."+id, row, label, v, checkOpts{Disabled: disabled != "", Reason: disabled, Tip: tip})
}

// diffStepper is a labelled -/+ row.
func (a *App) diffStepper(f *formCtx, id, label string, v *int, lo, hi int, tip string) {
	row, ok := f.row(diffCheckH + 4)
	if !ok {
		return
	}
	ls := tsBody
	a.uiLabel(row.X, row.Y+(row.H-float32(lineH(ls)))/2, label, ls)
	a.ngStepper("diff.f."+id, rect{row.right() - 112, row.Y + 2, 112, row.H - 4}, v, lo, hi)
	if tip != "" {
		a.uiTip("diff.t."+id, rect{row.X, row.Y, row.W - 116, row.H}, tip)
	}
}
