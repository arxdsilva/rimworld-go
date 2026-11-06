package wizard

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/difficulty"
)

// Custom Difficulty (NG-1 §2.5): a modal over Choose Storyteller with
// every difficulty value as a slider or a toggle, in two columns of
// groups. It edits a working copy; Accept stores it in the draft as the
// Custom preset, Cancel (and Esc, and the X) drops it.

// CustomDiff is the dialog's working copy.
type CustomDiff struct {
	S    difficulty.Settings
	From difficulty.Preset // the preset Copy and Reset start from
}

const (
	DiffModalW   = 900
	DiffModalH   = 640
	diffSliderH  = 40
	diffCheckH   = 28
	diffHeadH    = 26
	DiffColGap   = 28
	DiffTopRowH  = 36
	diffFormID   = "modal.diff.scroll"
	DisabledNote = "Only used with time-based escalation"
)

// openCustomDifficulty opens the dialog on the current values: the Custom
// settings when Custom is selected, else the selected preset's.
func (w storyPage) openCustomDifficulty() {
	ng := w.draft()
	from := ng.Preset
	if from == difficulty.Custom {
		from = difficulty.Seasoned
		ng.Cd = CustomDiff{S: ng.Custom, From: from}
	} else {
		ng.Cd = CustomDiff{S: difficulty.Of(ng.Preset), From: from}
	}
	w.kit().Push(uikit.Modal{
		ID: "diff.custom", Title: w.sh.T("Custom difficulty"), W: DiffModalW, H: DiffModalH,
		Draw: func(body gfx.Rect) { w.customDifficultyBody(body) },
		Buttons: []uikit.ModalButton{
			{ID: "accept", Label: w.sh.T("Accept"), Style: uikit.BtnPrimary, Closes: true, OnClick: w.acceptCustomDifficulty},
			{ID: "reset", Label: w.sh.T("Reset"), OnClick: func() { w.draft().Cd.S = difficulty.Of(w.draft().Cd.From) }},
			{ID: "cancel", Label: w.sh.T("Cancel"), Closes: true},
		},
	})
}

// acceptCustomDifficulty stores the working copy as the Custom preset.
func (w storyPage) acceptCustomDifficulty() {
	ng := w.draft()
	ng.Custom = difficulty.Clamp(ng.Cd.S)
	ng.Preset = difficulty.Custom
	ng.CustomOpen = true
}

// customDifficultyBody declares the form: the "Start from" row, then the
// groups in two columns inside one scroll view.
func (w storyPage) customDifficultyBody(body gfx.Rect) {
	ng := w.draft()
	cd := &ng.Cd
	top, rest := body.SplitV(DiffTopRowH)

	lab := uikit.DimText()
	w.kit().Label(top.X, top.Y+(top.H-float32(gfx.LineH(lab)))/2, w.sh.T("Start from:"), lab)
	names := make([]string, 0, int(difficulty.Custom))
	for _, n := range difficulty.Presets() {
		names = append(names, n.Label)
	}
	uikit.Choice(w.kit(), "diff.from", gfx.Rect{X: top.X + 90, Y: top.Y + 2, W: 170, H: top.H - 4}, w.sh.T("Start from"), names, int(cd.From), "", func(i int) { cd.From = difficulty.Preset(i) })
	uikit.StyledButton(w.kit(), "diff.copy", gfx.Rect{X: top.X + 272, Y: top.Y + 2, W: 150, H: top.H - 4}, w.sh.T("Copy"), uikit.BtnOpts{Tip: w.sh.T("Copy\nCopy every value of the preset into Custom.")},
		func() { cd.S = difficulty.Of(cd.From) })

	area := w.kit().ScrollArea(diffFormID, rest, ng.formHeight(diffFormID), uikit.ScrollOpts{Step: 40})
	colW := (area.View.W - 8 - DiffColGap) / 2
	left := &uikit.Form{View: area.View, X: area.View.X + 4, W: colW, Off: area.Off}
	right := &uikit.Form{View: area.View, X: area.View.X + 4 + colW + DiffColGap, W: colW, Off: area.Off}
	s := &cd.S

	// Threats.
	left.SectionTitle(w.sh, w.sh.T("Threats"))
	w.diffSlider(left, "ts", w.sh.T("Threat scale"), &s.ThreatScale, "ts", PctText, "", w.sh.T("Threat scale\nHow big raids and packs are, as a share of the normal size."))
	left.Check(w.sh, "bt", w.sh.T("Big threats"), &s.BigThreats, "", w.sh.T("Big threats\nRaids and packs. Off leaves only small events."))
	left.Stepper(w.sh, "pd", w.sh.T("Peaceful start (days)"), &s.PeacefulDays, 0, 30, w.sh.T("Peaceful start\nDays at the start with no big threat."))
	left.Check(w.sh, "ph", w.sh.T("Predators hunt colonists"), &s.PredatorsHuntColonists, "", w.sh.T("Predators\nWild predators may attack colonists, not only prey."))
	w.diffSlider(left, "ar", w.sh.T("Hostile wildlife revenge"), &s.AnimalRevenge, "ar", PctText, "", w.sh.T("Revenge\nHow fiercely wild animals fight back when hurt."))
	left.Gap(8)

	// Escalation.
	left.SectionTitle(w.sh, w.sh.T("Escalation"))
	left.Check(w.sh, "ad", w.sh.T("Adaptation"), &s.Adaptation, "", w.sh.T("Adaptation\nThreats grow while the colony loses nobody, and ease after a death."))
	adaptOff := ""
	if !s.Adaptation {
		adaptOff = w.sh.T("Only used with adaptation")
	}
	w.diffSlider(left, "as", w.sh.T("Adaptation strength"), &s.AdaptStrength, "as", PctText, adaptOff, w.sh.T("Adaptation strength\nHow far adaptation can move the threat size."))
	left.Check(w.sh, "te", w.sh.T("Time-based escalation"), &s.TimeEscalation, "", w.sh.T("Time-based escalation\nThreats also grow with the years that pass."))
	if !s.TimeEscalation {
		s.IgnoreWealth = false
	}
	timeOff := ""
	if !s.TimeEscalation {
		timeOff = w.sh.T("Only used with time-based escalation")
	}
	w.diffSlider(left, "ep", w.sh.T("Escalation per year"), &s.EscPerYear, "ep", PctText, timeOff, w.sh.T("Escalation per year\nHow much the threat size grows each year."))
	left.Check(w.sh, "iw", w.sh.T("Ignore wealth"), &s.IgnoreWealth, timeOff, w.sh.T("Ignore wealth\nThreat size follows time and people only, not what you own."))

	// Economy.
	right.SectionTitle(w.sh, w.sh.T("Economy"))
	w.diffSlider(right, "cy", w.sh.T("Crop yield"), &s.CropYield, "cy", PctText, "", "")
	w.diffSlider(right, "my", w.sh.T("Mining yield"), &s.MineYield, "my", PctText, "", "")
	w.diffSlider(right, "by", w.sh.T("Butchering yield"), &s.ButcherYield, "by", PctText, "", "")
	w.diffSlider(right, "rs", w.sh.T("Research speed"), &s.ResearchSpeed, "rs", PctText, "", "")
	right.Gap(8)

	// Health and mind.
	right.SectionTitle(w.sh, w.sh.T("Health and mind"))
	w.diffSlider(right, "mo", w.sh.T("Mood offset"), &s.MoodOffset, "mo", SignedText, "", w.sh.T("Mood offset\nAdded to every colonist's mood."))
	w.diffSlider(right, "if", w.sh.T("Infection chance"), &s.Infection, "if", PctText, "", "")
	w.diffSlider(right, "df", w.sh.T("Disease frequency"), &s.DiseaseFreq, "df", PctText, "", w.sh.T("Disease frequency\nHow often flu, plague, malaria, sleeping sickness, gut worms and muscle parasites strike, from day 15 (malaria in warm lands, sleeping sickness in the tropics). 100% is the normal rate; 0% turns diseases off."))
	if row, ok := right.Row(18); ok {
		ns := gfx.Small
		ns.Color = uikit.TextDim
		w.kit().Label(row.X, row.Y, w.sh.T("How often diseases strike; 0% turns them off."), ns)
	}
	w.diffSlider(right, "pt", w.sh.T("Pain tolerance offset"), &s.PainTolerance, "pt", SignedText, "", "")
	right.Gap(8)

	// Nature.
	right.SectionTitle(w.sh, w.sh.T("Nature"))
	right.Check(w.sh, "ew", w.sh.T("Extreme weather"), &s.ExtremeWeather, "", w.sh.T("Extreme weather\nHeat waves and cold snaps."))

	ng.setFormHeight(diffFormID, max(left.Content(), right.Content()))
}

func PctText(v int) string    { return fmt.Sprintf("%d%%", v) }
func SignedText(v int) string { return fmt.Sprintf("%+d", v) }

// The disease frequency slider's tooltip and the dim line under it
// (MED-04): what the setting does, now that diseases strike.
const (
	DiseaseFreqTip  = "Disease frequency\nHow often flu, plague, malaria, sleeping sickness, gut worms and muscle parasites strike, from day 15 (malaria in warm lands, sleeping sickness in the tropics). 100% is the normal rate; 0% turns diseases off."
	DiseaseFreqNote = "How often diseases strike; 0% turns them off."
)

// stepFor is a slider's key step: 5 for wide ranges, 1 for narrow ones.
func stepFor(l difficulty.Limits) float64 {
	if l.Max-l.Min >= 100 {
		return 5
	}
	return 1
}

// DiffSpec is the slider range and key step of the difficulty field key.
func DiffSpec(key string) uikit.SliderSpec {
	lim, _ := difficulty.Limit(key)
	return uikit.SliderSpec{Min: float64(lim.Min), Max: float64(lim.Max), Step: stepFor(lim)}
}

// diffSlider is a labelled slider row over *v with the range of the
// difficulty field key; a reason disables it.
func (w storyPage) diffSlider(f *uikit.Form, id, label string, v *int, key string, format func(int) string, disabled, tip string) {
	f.Slider(w.sh, id, label, v, w.sh.SliderTick(DiffSpec(key)), format, disabled, tip)
}
