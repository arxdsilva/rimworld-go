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
		ID: "diff.custom", Title: "Custom difficulty", W: DiffModalW, H: DiffModalH,
		Draw: func(body gfx.Rect) { w.customDifficultyBody(body) },
		Buttons: []uikit.ModalButton{
			{ID: "accept", Label: "Accept", Style: uikit.BtnPrimary, Closes: true, OnClick: w.acceptCustomDifficulty},
			{ID: "reset", Label: "Reset", OnClick: func() { w.draft().Cd.S = difficulty.Of(w.draft().Cd.From) }},
			{ID: "cancel", Label: "Cancel", Closes: true},
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
	w.kit().Label(top.X, top.Y+(top.H-float32(gfx.LineH(lab)))/2, "Start from:", lab)
	names := make([]string, 0, int(difficulty.Custom))
	for _, n := range difficulty.Presets() {
		names = append(names, n.Label)
	}
	uikit.Choice(w.kit(), "diff.from", gfx.Rect{X: top.X + 90, Y: top.Y + 2, W: 170, H: top.H - 4}, "Start from", names, int(cd.From), "", func(i int) { cd.From = difficulty.Preset(i) })
	uikit.StyledButton(w.kit(), "diff.copy", gfx.Rect{X: top.X + 272, Y: top.Y + 2, W: 150, H: top.H - 4}, "Copy", uikit.BtnOpts{Tip: "Copy\nCopy every value of the preset into Custom."},
		func() { cd.S = difficulty.Of(cd.From) })

	area := w.kit().ScrollArea(diffFormID, rest, ng.formHeight(diffFormID), uikit.ScrollOpts{Step: 40})
	colW := (area.View.W - 8 - DiffColGap) / 2
	left := &uikit.Form{View: area.View, X: area.View.X + 4, W: colW, Off: area.Off}
	right := &uikit.Form{View: area.View, X: area.View.X + 4 + colW + DiffColGap, W: colW, Off: area.Off}
	s := &cd.S

	// Threats.
	left.SectionTitle(w.sh, "Threats")
	w.diffSlider(left, "ts", "Threat scale", &s.ThreatScale, "ts", PctText, "", "Threat scale\nHow big raids and packs are, as a share of the normal size.")
	left.Check(w.sh, "bt", "Big threats", &s.BigThreats, "", "Big threats\nRaids and packs. Off leaves only small events.")
	left.Stepper(w.sh, "pd", "Peaceful start (days)", &s.PeacefulDays, 0, 30, "Peaceful start\nDays at the start with no big threat.")
	left.Check(w.sh, "ph", "Predators hunt colonists", &s.PredatorsHuntColonists, "", "Predators\nWild predators may attack colonists, not only prey.")
	w.diffSlider(left, "ar", "Hostile wildlife revenge", &s.AnimalRevenge, "ar", PctText, "", "Revenge\nHow fiercely wild animals fight back when hurt.")
	left.Gap(8)

	// Escalation.
	left.SectionTitle(w.sh, "Escalation")
	left.Check(w.sh, "ad", "Adaptation", &s.Adaptation, "", "Adaptation\nThreats grow while the colony loses nobody, and ease after a death.")
	adaptOff := ""
	if !s.Adaptation {
		adaptOff = "Only used with adaptation"
	}
	w.diffSlider(left, "as", "Adaptation strength", &s.AdaptStrength, "as", PctText, adaptOff, "Adaptation strength\nHow far adaptation can move the threat size.")
	left.Check(w.sh, "te", "Time-based escalation", &s.TimeEscalation, "", "Time-based escalation\nThreats also grow with the years that pass.")
	if !s.TimeEscalation {
		s.IgnoreWealth = false
	}
	timeOff := ""
	if !s.TimeEscalation {
		timeOff = DisabledNote
	}
	w.diffSlider(left, "ep", "Escalation per year", &s.EscPerYear, "ep", PctText, timeOff, "Escalation per year\nHow much the threat size grows each year.")
	left.Check(w.sh, "iw", "Ignore wealth", &s.IgnoreWealth, timeOff, "Ignore wealth\nThreat size follows time and people only, not what you own.")

	// Economy.
	right.SectionTitle(w.sh, "Economy")
	w.diffSlider(right, "cy", "Crop yield", &s.CropYield, "cy", PctText, "", "")
	w.diffSlider(right, "my", "Mining yield", &s.MineYield, "my", PctText, "", "")
	w.diffSlider(right, "by", "Butchering yield", &s.ButcherYield, "by", PctText, "", "")
	w.diffSlider(right, "rs", "Research speed", &s.ResearchSpeed, "rs", PctText, "", "")
	right.Gap(8)

	// Health and mind.
	right.SectionTitle(w.sh, "Health and mind")
	w.diffSlider(right, "mo", "Mood offset", &s.MoodOffset, "mo", SignedText, "", "Mood offset\nAdded to every colonist's mood.")
	w.diffSlider(right, "if", "Infection chance", &s.Infection, "if", PctText, "", "")
	w.diffSlider(right, "df", "Disease frequency", &s.DiseaseFreq, "df", PctText, "", DiseaseFreqTip)
	if row, ok := right.Row(18); ok {
		ns := gfx.Small
		ns.Color = uikit.TextDim
		w.kit().Label(row.X, row.Y, DiseaseFreqNote, ns)
	}
	w.diffSlider(right, "pt", "Pain tolerance offset", &s.PainTolerance, "pt", SignedText, "", "")
	right.Gap(8)

	// Nature.
	right.SectionTitle(w.sh, "Nature")
	right.Check(w.sh, "ew", "Extreme weather", &s.ExtremeWeather, "", "Extreme weather\nHeat waves and cold snaps.")

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
