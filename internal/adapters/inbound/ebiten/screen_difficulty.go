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
	S    difficulty.Settings
	From difficulty.Preset // the preset Copy and Reset start from
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
	from := ng.Preset
	if from == difficulty.Custom {
		from = difficulty.Seasoned
		ng.Cd = customDiff{S: ng.Custom, From: from}
	} else {
		ng.Cd = customDiff{S: difficulty.Of(ng.Preset), From: from}
	}
	a.uiPushModal(modal{
		ID: "diff.custom", Title: "Custom difficulty", W: diffModalW, H: diffModalH,
		Draw: func(a *App, body rect) { a.customDifficultyBody(body) },
		Buttons: []modalButton{
			{ID: "accept", Label: "Accept", Style: btnPrimary, Closes: true, OnClick: a.acceptCustomDifficulty},
			{ID: "reset", Label: "Reset", OnClick: func() { a.draft().Cd.S = difficulty.Of(a.draft().Cd.From) }},
			{ID: "cancel", Label: "Cancel", Closes: true},
		},
	})
}

// acceptCustomDifficulty stores the working copy as the Custom preset.
func (a *App) acceptCustomDifficulty() {
	ng := a.draft()
	ng.Custom = difficulty.Clamp(ng.Cd.S)
	ng.Preset = difficulty.Custom
	ng.CustomOpen = true
}

// customDifficultyBody declares the form: the "Start from" row, then the
// groups in two columns inside one scroll view.
func (a *App) customDifficultyBody(body rect) {
	ng := a.draft()
	cd := &ng.Cd
	top, rest := body.SplitV(diffTopRowH)

	lab := dimText()
	a.uiLabel(top.X, top.Y+(top.H-float32(lineH(lab)))/2, "Start from:", lab)
	names := make([]string, 0, int(difficulty.Custom))
	for _, n := range difficulty.Presets() {
		names = append(names, n.Label)
	}
	a.ngChoice("diff.from", rect{X: top.X + 90, Y: top.Y + 2, W: 170, H: top.H - 4}, "Start from", names, int(cd.From), "", func(i int) { cd.From = difficulty.Preset(i) })
	a.ngButton("diff.copy", rect{X: top.X + 272, Y: top.Y + 2, W: 150, H: top.H - 4}, "Copy", btnOpts{Tip: "Copy\nCopy every value of the preset into Custom."},
		func() { cd.S = difficulty.Of(cd.From) })

	area := a.uiScrollArea(diffFormID, rest, ng.formHeight(diffFormID), scrollOpts{Step: 40})
	colW := (area.View.W - 8 - diffColGap) / 2
	left := &formCtx{View: area.View, X: area.View.X + 4, W: colW, Off: area.Off}
	right := &formCtx{View: area.View, X: area.View.X + 4 + colW + diffColGap, W: colW, Off: area.Off}
	s := &cd.S

	// Threats.
	left.SectionTitle(a, "Threats")
	a.diffSlider(left, "ts", "Threat scale", &s.ThreatScale, "ts", pctText, "", "Threat scale\nHow big raids and packs are, as a share of the normal size.")
	a.diffCheck(left, "bt", "Big threats", &s.BigThreats, "", "Big threats\nRaids and packs. Off leaves only small events.")
	a.diffStepper(left, "pd", "Peaceful start (days)", &s.PeacefulDays, 0, 30, "Peaceful start\nDays at the start with no big threat.")
	a.diffCheck(left, "ph", "Predators hunt colonists", &s.PredatorsHuntColonists, "", "Predators\nWild predators may attack colonists, not only prey.")
	a.diffSlider(left, "ar", "Hostile wildlife revenge", &s.AnimalRevenge, "ar", pctText, "", "Revenge\nHow fiercely wild animals fight back when hurt.")
	left.Gap(8)

	// Escalation.
	left.SectionTitle(a, "Escalation")
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
	right.SectionTitle(a, "Economy")
	a.diffSlider(right, "cy", "Crop yield", &s.CropYield, "cy", pctText, "", "")
	a.diffSlider(right, "my", "Mining yield", &s.MineYield, "my", pctText, "", "")
	a.diffSlider(right, "by", "Butchering yield", &s.ButcherYield, "by", pctText, "", "")
	a.diffSlider(right, "rs", "Research speed", &s.ResearchSpeed, "rs", pctText, "", "")
	right.Gap(8)

	// Health and mind.
	right.SectionTitle(a, "Health and mind")
	a.diffSlider(right, "mo", "Mood offset", &s.MoodOffset, "mo", signedText, "", "Mood offset\nAdded to every colonist's mood.")
	a.diffSlider(right, "if", "Infection chance", &s.Infection, "if", pctText, "", "")
	a.diffSlider(right, "df", "Disease frequency", &s.DiseaseFreq, "df", pctText, "", diseaseFreqTip)
	if row, ok := right.Row(18); ok {
		ns := tsSmall
		ns.Color = uiTextDim
		a.uiLabel(row.X, row.Y, diseaseFreqNote, ns)
	}
	a.diffSlider(right, "pt", "Pain tolerance offset", &s.PainTolerance, "pt", signedText, "", "")
	right.Gap(8)

	// Nature.
	right.SectionTitle(a, "Nature")
	a.diffCheck(right, "ew", "Extreme weather", &s.ExtremeWeather, "", "Extreme weather\nHeat waves and cold snaps.")

	ng.setFormHeight(diffFormID, max(left.Content(), right.Content()))
}

func pctText(v int) string    { return fmt.Sprintf("%d%%", v) }
func signedText(v int) string { return fmt.Sprintf("%+d", v) }

// diffSlider is a labelled slider row over *v with the range of the
// difficulty field key. A reason disables it (dimmed, with the reason as
// The disease frequency slider's tooltip and the dim line under it
// (MED-04): what the setting does, now that diseases strike.
const (
	diseaseFreqTip  = "Disease frequency\nHow often flu, plague, malaria, sleeping sickness, gut worms and muscle parasites strike, from day 15 (malaria in warm lands, sleeping sickness in the tropics). 100% is the normal rate; 0% turns diseases off."
	diseaseFreqNote = "How often diseases strike; 0% turns them off."
)

// its tooltip).
func (a *App) diffSlider(f *formCtx, id, label string, v *int, key string, format func(int) string, disabled, tip string) {
	row, ok := f.Row(diffSliderH)
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
	a.uiLabel(row.Right(), row.Y+1, format(*v), vs)
	sr := rect{X: row.X + 2, Y: row.Y + 20, W: row.W - 4, H: 18}
	a.uiSliderInt("diff.f."+id, sr, v, sliderSpec{
		Min: float64(lim.Min), Max: float64(lim.Max), Step: stepFor(lim), Title: label,
		Format:   func(x float64) string { return format(int(x)) },
		Disabled: disabled != "", Reason: disabled,
	})
	if tip != "" && disabled == "" {
		a.uiTip("diff.t."+id, rect{X: row.X, Y: row.Y, W: row.W, H: 18}, tip)
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
	row, ok := f.Row(diffCheckH)
	if !ok {
		return
	}
	a.uiCheck("diff.f."+id, row, label, v, checkOpts{Disabled: disabled != "", Reason: disabled, Tip: tip})
}

// diffStepper is a labelled -/+ row.
func (a *App) diffStepper(f *formCtx, id, label string, v *int, lo, hi int, tip string) {
	row, ok := f.Row(diffCheckH + 4)
	if !ok {
		return
	}
	ls := tsBody
	a.uiLabel(row.X, row.Y+(row.H-float32(lineH(ls)))/2, label, ls)
	a.ngStepper("diff.f."+id, rect{X: row.Right() - 112, Y: row.Y + 2, W: 112, H: row.H - 4}, v, lo, hi)
	if tip != "" {
		a.uiTip("diff.t."+id, rect{X: row.X, Y: row.Y, W: row.W - 116, H: row.H}, tip)
	}
}
