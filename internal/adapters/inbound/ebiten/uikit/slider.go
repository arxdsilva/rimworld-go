package uikit

import (
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
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

// SliderSpec describes a slider's range and look.
type SliderSpec struct {
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
	// Tick is the host's step sound, called on every change of the value
	// from old to cur (nil: silent).
	Tick func(lo, hi, old, cur float64)
}

func (sp SliderSpec) bounds() (lo, hi float64) {
	if len(sp.Stops) >= 2 {
		return 0, float64(len(sp.Stops) - 1)
	}
	lo, hi = sp.Min, sp.Max
	if hi <= lo {
		hi = lo + 1
	}
	return lo, hi
}

func (sp SliderSpec) Discrete() bool { return len(sp.Stops) >= 2 }

func (sp SliderSpec) format(v float64) string {
	switch {
	case sp.Format != nil:
		return sp.Format(v)
	case sp.Discrete():
		return sp.Stops[int(math.Round(v))]
	}
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// SliderTrackRect is the track inside r: inset by half a knob so the knob
// stays inside r at both ends.
func SliderTrackRect(r gfx.Rect) gfx.Rect {
	return gfx.Rect{X: r.X + sliderKnobW/2, Y: r.Y + (sliderKnobH-sliderTrackH)/2, W: max(0, r.W-sliderKnobW), H: sliderTrackH}
}

// sliderFrac is v's position along the track, 0..1.
func (sp SliderSpec) frac(v float64) float64 {
	lo, hi := sp.bounds()
	return math.Max(0, math.Min(1, (v-lo)/(hi-lo)))
}

// sliderValueAt is the value for a cursor x on the track, snapped to the
// stops of a discrete slider.
func (sp SliderSpec) valueAt(x float32, track gfx.Rect) float64 {
	lo, hi := sp.bounds()
	t := 0.0
	if track.W > 0 {
		t = math.Max(0, math.Min(1, float64(x-track.X)/float64(track.W)))
	}
	v := lo + t*(hi-lo)
	if sp.Discrete() {
		v = math.Round(v)
	}
	return v
}

// Slider is a horizontal slider bound to *val. Pressing the knob or the
// track captures the mouse and the knob follows the cursor until release
// (which is consumed). Left and Right move one step while focused, Shift
// five, Home and End jump to the ends. It reports the value changed.
func (k *Kit) Slider(id string, r gfx.Rect, val *float64, sp SliderSpec) bool {
	lo, hi := sp.bounds()
	track := SliderTrackRect(r)
	*val = math.Max(lo, math.Min(hi, *val))
	if sp.Discrete() {
		*val = math.Round(*val)
	}
	changed := false
	set := func(v float64) {
		v = math.Max(lo, math.Min(hi, v))
		if sp.Discrete() {
			v = math.Round(v)
		}
		if v != *val {
			if sp.Tick != nil {
				sp.Tick(lo, hi, *val, v)
			}
			*val, changed = v, true
		}
	}
	w := Widget{ID: id, R: r, Focusable: true, Disabled: sp.Disabled, Reason: sp.Reason}
	live := k.Add(w)
	idx := len(k.Cur) - 1
	if live && !sp.Disabled {
		mx, _ := k.host.Cursor()
		if k.Press(r) {
			k.Active, k.Focus = id, id
			set(sp.valueAt(float32(mx), track))
		}
		if k.Active == id {
			set(sp.valueAt(float32(mx), track))
			if !k.in().Down(inputframe.Left) {
				k.Active = ""
				k.release() // the release must not click what is under the cursor
			}
		} else if k.Focus == id {
			step := sp.Step
			if sp.Discrete() {
				step = 1
			} else if step <= 0 {
				step = (hi - lo) / 20
			}
			if k.in().Shift() {
				step *= 5
			}
			switch {
			case k.host.KeyRepeat("ArrowLeft"):
				set(*val - step)
			case k.host.KeyRepeat("ArrowRight"):
				set(*val + step)
			case k.in().Key("Home"):
				set(lo)
			case k.in().Key("End"):
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
		k.Cur[idx].Tip = tip
	}
	k.Queue(func(dst *ebiten.Image) { drawSlider(dst, r, track, *val, sp, k.Hot == id || k.Active == id) })
	return changed
}

// SliderInt is Slider over an int.
func (k *Kit) SliderInt(id string, r gfx.Rect, val *int, sp SliderSpec) bool {
	f := float64(*val)
	changed := k.Slider(id, r, &f, sp)
	*val = int(math.Round(f))
	return changed
}

func drawSlider(dst *ebiten.Image, r, track gfx.Rect, v float64, sp SliderSpec, hot bool) {
	FillRect(dst, track, SliderTrack)
	t := float32(sp.frac(v))
	FillRect(dst, gfx.Rect{X: track.X, Y: track.Y, W: track.W * t, H: track.H}, SliderFill)
	StrokeRect(dst, track, PanelBorder)
	n := len(sp.Stops)
	for i, l := range sp.Stops {
		x := track.X + track.W*float32(i)/float32(n-1)
		gfx.Line(dst, x, track.Bottom()+1, x, track.Bottom()+4, 1, PanelBorder, false)
		st := gfx.Small
		st.Color = TextDim
		if math.Round(v) == float64(i) {
			st.Color = Text
		}
		ax := float64(x)
		switch {
		case i == 0:
			st.Align, ax = gfx.AlignLeft, float64(r.X)
		case i == n-1:
			st.Align, ax = gfx.AlignRight, float64(r.Right())
		default:
			st.Align = gfx.AlignCenter
		}
		gfx.Text(dst, l, ax, float64(r.Y+sliderKnobH+3), st)
	}
	kx := track.X + track.W*t
	knob := gfx.Rect{X: kx - sliderKnobW/2, Y: r.Y, W: sliderKnobW, H: sliderKnobH}
	c := SliderKnob
	if sp.Disabled {
		c = BtnDisabled
	} else if hot {
		c = LerpRGBA(c, Text, 0.35)
	}
	FillRect(dst, knob, c)
	StrokeRect(dst, knob, BtnBorder)
}
