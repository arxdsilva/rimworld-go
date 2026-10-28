package ebiten

import (
	"math"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Slider geometry (NG-1 §1.4): a 6 px track under a 14x18 knob. The
// widget's rectangle also holds the stop labels under the track.
const (
	sliderTrackH = 6
	sliderKnobW  = 14
	sliderKnobH  = 18
)

// sliderSpec describes a slider's range and look.
type sliderSpec struct {
	Min, Max float64
	// Stops, when it has two or more labels, makes the slider discrete:
	// the value is the stop index (Min and Max are ignored).
	Stops []string
	// Step is a continuous slider's arrow-key step (default 1/20 of the range).
	Step float64
	// Format renders the value for the tooltip (default: the stop label or
	// the number).
	Format   func(float64) string
	Title    string // tooltip title above the value
	Disabled bool
	Reason   string
}

func (sp sliderSpec) bounds() (lo, hi float64) {
	if len(sp.Stops) >= 2 {
		return 0, float64(len(sp.Stops) - 1)
	}
	lo, hi = sp.Min, sp.Max
	if hi <= lo {
		hi = lo + 1
	}
	return lo, hi
}

func (sp sliderSpec) discrete() bool { return len(sp.Stops) >= 2 }

func (sp sliderSpec) format(v float64) string {
	switch {
	case sp.Format != nil:
		return sp.Format(v)
	case sp.discrete():
		return sp.Stops[int(math.Round(v))]
	}
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// sliderTrack is the track inside r: inset by half a knob so the knob
// stays inside r at both ends.
func sliderTrack(r rect) rect {
	return rect{X: r.X + sliderKnobW/2, Y: r.Y + (sliderKnobH-sliderTrackH)/2, W: max(0, r.W-sliderKnobW), H: sliderTrackH}
}

// sliderFrac is v's position along the track, 0..1.
func (sp sliderSpec) frac(v float64) float64 {
	lo, hi := sp.bounds()
	return math.Max(0, math.Min(1, (v-lo)/(hi-lo)))
}

// sliderValueAt is the value for a cursor x on the track, snapped to the
// stops of a discrete slider.
func (sp sliderSpec) valueAt(x float32, track rect) float64 {
	lo, hi := sp.bounds()
	t := 0.0
	if track.W > 0 {
		t = math.Max(0, math.Min(1, float64(x-track.X)/float64(track.W)))
	}
	v := lo + t*(hi-lo)
	if sp.discrete() {
		v = math.Round(v)
	}
	return v
}

// uiSlider is a horizontal slider bound to *val. Pressing the knob or the
// track captures the mouse and the knob follows the cursor until release
// (which is consumed). Left and Right move one step while focused, Shift
// five, Home and End jump to the ends. It reports the value changed.
func (a *App) uiSlider(id string, r rect, val *float64, sp sliderSpec) bool {
	u := a.uiSt()
	lo, hi := sp.bounds()
	track := sliderTrack(r)
	*val = math.Max(lo, math.Min(hi, *val))
	if sp.discrete() {
		*val = math.Round(*val)
	}
	changed := false
	set := func(v float64) {
		v = math.Max(lo, math.Min(hi, v))
		if sp.discrete() {
			v = math.Round(v)
		}
		if v != *val {
			a.sliderCue(sp, lo, hi, *val, v)
			*val, changed = v, true
		}
	}
	w := uiWidget{id: id, r: r, focusable: true, disabled: sp.Disabled, reason: sp.Reason}
	live := u.add(w)
	idx := len(u.cur) - 1
	if live && !sp.Disabled {
		mx, _ := a.currentCursor()
		if a.uiPress(r) {
			u.active, u.focus = id, id
			set(sp.valueAt(float32(mx), track))
		}
		if u.active == id {
			set(sp.valueAt(float32(mx), track))
			if !a.in.down(mbLeft) {
				u.active = ""
				a.uiRelease() // the release must not click what is under the cursor
			}
		} else if u.focus == id {
			step := sp.Step
			if sp.discrete() {
				step = 1
			} else if step <= 0 {
				step = (hi - lo) / 20
			}
			if a.in.shift() {
				step *= 5
			}
			switch {
			case a.keyRepeat("ArrowLeft"):
				set(*val - step)
			case a.keyRepeat("ArrowRight"):
				set(*val + step)
			case a.in.key("Home"):
				set(lo)
			case a.in.key("End"):
				set(hi)
			}
		}
	}
	if live {
		tip := sp.format(*val)
		if sp.Title != "" {
			tip = sp.Title + "\n" + tip
		}
		if sp.Disabled && sp.Reason != "" {
			tip = sp.Reason
		}
		u.cur[idx].tip = tip
	}
	u.queue(func(dst *ebiten.Image) { drawSlider(dst, r, track, *val, sp, u.hot == id || u.active == id) })
	return changed
}

// uiSliderInt is uiSlider over an int.
func (a *App) uiSliderInt(id string, r rect, val *int, sp sliderSpec) bool {
	f := float64(*val)
	changed := a.uiSlider(id, r, &f, sp)
	*val = int(math.Round(f))
	return changed
}

func drawSlider(dst *ebiten.Image, r, track rect, v float64, sp sliderSpec, hot bool) {
	fillRect(dst, track, uiSliderTrack)
	t := float32(sp.frac(v))
	fillRect(dst, rect{X: track.X, Y: track.Y, W: track.W * t, H: track.H}, uiSliderFill)
	strokeRect(dst, track, uiPanelBorder)
	n := len(sp.Stops)
	for i, l := range sp.Stops {
		x := track.X + track.W*float32(i)/float32(n-1)
		uiLine(dst, x, track.Bottom()+1, x, track.Bottom()+4, 1, uiPanelBorder, false)
		st := tsSmall
		st.Color = uiTextDim
		if math.Round(v) == float64(i) {
			st.Color = uiText
		}
		ax := float64(x)
		switch {
		case i == 0:
			st.Align, ax = alignLeft, float64(r.X)
		case i == n-1:
			st.Align, ax = alignRight, float64(r.Right())
		default:
			st.Align = alignCenter
		}
		drawText(dst, l, ax, float64(r.Y+sliderKnobH+3), st)
	}
	kx := track.X + track.W*t
	knob := rect{X: kx - sliderKnobW/2, Y: r.Y, W: sliderKnobW, H: sliderKnobH}
	c := uiSliderKnob
	if sp.Disabled {
		c = uiBtnDisabled
	} else if hot {
		c = lerpRGBA(c, uiText, 0.35)
	}
	fillRect(dst, knob, c)
	strokeRect(dst, knob, uiBtnBorder)
}
