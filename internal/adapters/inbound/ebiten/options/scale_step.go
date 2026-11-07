package options

import (
	"math"
	"strconv"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// The UI scale row (UIS-001, 003, 004, 007, 008): a click opens a list of
// Auto and the nine steps; a step the window cannot hold is refused with a
// warning and the old value stays; a step that is accepted applies at once
// and asks to keep it, reverting on its own when the countdown runs out.

const (
	// scaleKeepSecs is how long the keep-or-revert question waits.
	scaleKeepSecs = 10
	scalePickID   = "options.uiscale.pick"
	scaleKeepID   = "options.uiscale.keep"
)

// ScaleControl is the UI scale row's behaviour: the list, the refusal, the
// keep-or-revert question and its countdown. The Screen holds one (Scale).
type ScaleControl struct {
	sh Shell
	// Now is the clock the countdown reads (nil: time.Now); a test sets it. It
	// never reaches the simulation.
	Now  func() time.Time
	keep *scaleKeep
}

// scaleKeep is the open keep-or-revert question: the settings to go back to
// and when the countdown ends.
type scaleKeep struct {
	prev     config.Accessibility
	deadline time.Time
}

// stepText names a step in percent: 125 is "1.25×".
func stepText(percent int) string {
	return strconv.FormatFloat(float64(percent)/100, 'f', -1, 64) + "×"
}

// stepTextF names a step in use (a float step) the same way.
func stepTextF(step float64) string { return stepText(int(math.Round(step * 100))) }

// uiScaleLabel names the stored choice. Auto shows the step it uses; a
// stored step the window cannot hold shows both (UIS-004).
func (o *ScaleControl) uiScaleLabel() string {
	ac := o.sh.Settings().Accessibility
	used := o.sh.Env().UIScale
	if ac.UIScaleAuto {
		return o.sh.Tf("Auto (%s)", stepTextF(used))
	}
	stored := stepText(ac.UIScale)
	if used > 0 && math.Abs(used-float64(ac.UIScale)/100) > 1e-6 {
		return o.sh.Tf("%s (using %s)", stored, stepTextF(used))
	}
	return stored
}

// window is the window's physical size: the program's when it says, else the
// logical size times the step in use (what the layout was made from).
func (o *ScaleControl) window() (w, h float64) {
	env := o.sh.Env()
	if env.Window != nil {
		pw, ph := env.Window()
		return float64(pw), float64(ph)
	}
	lw, lh := o.sh.Size()
	st := env.UIScale
	if st <= 0 {
		st = 1
	}
	return float64(lw) * st, float64(lh) * st
}

// stepAllowed reports whether the window can hold a step (UIS-003).
func (o *ScaleControl) stepAllowed(percent int) bool {
	if percent <= 100 {
		return true
	}
	w, h := o.window()
	st := float64(percent) / 100
	return w/st >= uikit.MinStepW-1e-6 && h/st >= uikit.MinStepH-1e-6
}

func (o *ScaleControl) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// uiScaleRow is the UI scale row; its click opens the list.
func uiScaleRow(t lang.Tr) OptionRow {
	return OptionRow{Key: "uiscale", Label: t.T("UI scale"), kind: rowChoice,
		Tip:  t.T("Scales text and every panel. Auto fits the window."),
		live: func(o *Screen) string { return o.Scale.uiScaleLabel() },
		run:  func(o *Screen) { o.Scale.openScaleList(o.rowRect(ids.PrefixOptions + "uiscale")) }}
}

// Choose picks Auto (0) or a step in percent. Auto applies at once.
// A step the window cannot hold is refused: nothing changes and a one-line
// warning shows. Any other step applies at once and asks to keep it.
func (o *ScaleControl) Choose(percent int) {
	s := o.sh.Settings()
	ac := &s.Accessibility
	prev := *ac
	if percent == 0 {
		ac.UIScaleLegacy, ac.UIScaleAuto = 0, true
		o.sh.SetSettings(s)
		return
	}
	if !ac.UIScaleAuto && ac.UIScale == percent && ac.UIScaleLegacy == 0 {
		return
	}
	if !o.stepAllowed(percent) {
		o.sh.Toast(o.sh.Tf("This window is too small for %s; keeping %s", stepText(percent), o.currentChoice(prev)))
		return
	}
	ac.UIScaleLegacy, ac.UIScaleAuto, ac.UIScale = 0, false, percent
	o.sh.SetSettings(s)
	o.keep = &scaleKeep{prev: prev, deadline: o.now().Add(scaleKeepSecs * time.Second)}
	o.openScaleKeep()
}

// currentChoice names a stored choice for a warning.
func (o *ScaleControl) currentChoice(ac config.Accessibility) string {
	if ac.UIScaleAuto {
		return o.sh.T("Auto")
	}
	return stepText(ac.Step(1))
}

// Keep ends the question and keeps the new step.
func (o *ScaleControl) Keep() { o.keep = nil }

// Revert ends the question and restores the previous choice.
func (o *ScaleControl) Revert() {
	if o.keep == nil {
		return
	}
	prev := o.keep.prev
	o.keep = nil
	s := o.sh.Settings()
	s.Accessibility = prev
	o.sh.SetSettings(s)
}

// Tick ends the countdown when it has run out (called every Update).
func (o *ScaleControl) Tick() {
	if o.keep != nil && !o.now().Before(o.keep.deadline) {
		o.Revert()
		o.sh.Kit().CloseModal(scaleKeepID)
	}
}

func (o *ScaleControl) openScaleKeep() {
	k := o.sh.Kit()
	k.Push(uikit.Modal{ID: scaleKeepID, Title: o.sh.T("Keep this UI scale?"), W: 500, H: 300,
		Draw: func(body gfx.Rect) {
			left := 0
			if o.keep != nil {
				left = max(0, int(math.Ceil(o.keep.deadline.Sub(o.now()).Seconds())))
			}
			uikit.TextLines(k, body.X, body.Y, body.W,
				o.sh.Tf("The interface now uses %s. Keep it?", o.uiScaleLabel()), uikit.DimText())
			uikit.TextLines(k, body.X, body.Y+48, body.W,
				o.sh.Tf("Going back to the old size in %d s.", left), uikit.DimText())
		},
		Buttons: []uikit.ModalButton{
			{ID: "keep", Label: o.sh.T("Keep"), Style: uikit.BtnPrimary, Closes: true, OnClick: o.Keep},
			{ID: "revert", Label: o.sh.T("Revert"), Closes: true, OnClick: o.Revert},
		},
		OnClose: o.Revert})
}

// scaleChoices are the list's entries: 0 is Auto, then every step.
func scaleChoices() []int { return append([]int{0}, config.UIScalePresets...) }

// openScaleList opens the drop list under the row's value control: Auto and
// every step, the selected one marked, a step the window refuses disabled
// with its reason.
func (o *ScaleControl) openScaleList(anchor gfx.Rect) {
	choices := scaleChoices()
	ac := o.sh.Settings().Accessibility
	opts := make([]uikit.DropOpt, len(choices))
	cur := 0
	for i, c := range choices {
		label := o.sh.T("Auto")
		if c != 0 {
			label = stepText(c)
		}
		if (c == 0 && ac.UIScaleAuto) || (c != 0 && !ac.UIScaleAuto && c == ac.UIScale) {
			cur = i
		}
		refused := c != 0 && !o.stepAllowed(c)
		opts[i] = uikit.DropOpt{Label: label, Tip: o.scaleTip(c, refused), Disabled: refused}
	}
	uikit.DropList(o.sh.Kit(), scalePickID, anchor, opts, cur, func(i int) { o.Choose(choices[i]) })
}

func (o *ScaleControl) scaleTip(c int, refused bool) string {
	switch {
	case c == 0:
		return o.sh.T("Auto\nPick the largest size that leaves room for the whole interface.")
	case refused:
		return o.sh.Tf("%s\nThis window is too small for this size.", stepText(c))
	}
	return o.sh.Tf("%s\nThe interface is drawn %s as large as at 1×.", stepText(c), stepText(c))
}
