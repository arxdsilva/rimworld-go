package body

import (
	"fmt"
	"strings"
)

// partsOf are the body parts whose efficiency feeds capacity c directly.
func partsOf(c Capacity) []Part {
	switch c {
	case Moving:
		return []Part{LegL, FootL, LegR, FootR, Spine}
	case Manipulation:
		return []Part{ArmL, HandL, ArmR, HandR}
	case Sight:
		return []Part{EyeL, EyeR}
	case Hearing:
		return []Part{EarL, EarR}
	case Talking:
		return []Part{Jaw}
	case Breathing:
		return []Part{LungL, LungR}
	case BloodPumping:
		return []Part{Heart}
	case BloodFiltration:
		return []Part{KidneyL, KidneyR, Liver}
	case Digestion:
		return []Part{Stomach, Liver}
	case Consciousness:
		return []Part{Brain}
	}
	return nil
}

// partFactor is the line of an imperfect part ("Left leg: peg leg 60%").
func (b *Body) partFactor(p Part) (Factor, bool) {
	e := b.eff(p)
	if e == 1000 {
		return Factor{}, false
	}
	state := "hurt"
	switch {
	case b.isMissing(p) && b.fitted(p):
		state = strings.ToLower(b.Implant(p).String())
	case b.isMissing(p) && b.replaced(p):
		state = "prosthetic limb"
	case b.isMissing(p):
		state = "missing"
	}
	return Factor{Label: upperFirst(p.Label()) + ": " + state, Pct: e / 10}, true
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// CapacityFactors explains Capacity(c) line by line, in percent: the
// parts working below full, the spine, pain, blood pumping and filtration
// and blood loss that weigh on it, and each condition stage's change. The
// last line is the capacity itself.
func (b *Body) CapacityFactors(c Capacity) []Factor {
	var out []Factor
	for _, p := range partsOf(c) {
		if f, ok := b.partFactor(p); ok {
			out = append(out, f)
		}
	}
	if len(out) == 0 && len(partsOf(c)) > 0 {
		out = append(out, Factor{Label: "Parts healthy", Pct: 100})
	}
	if c == Moving || c == Manipulation || c == Talking {
		out = append(out, Factor{Label: "Capped by consciousness", Pct: b.consciousness(DefOfCond) / 10})
	}
	if c == Moving {
		if v := b.pumping(DefOfCond); v < 1000 {
			out = append(out, Factor{Label: fmt.Sprintf("Blood pumping (weight %d%%)", pumpingWeight/10), Pct: v / 10})
		}
	}
	if c == Consciousness {
		if pn := b.Pain(); pn > 0 {
			out = append(out, Factor{Label: "Pain (half counts)", Pct: -pn / 20})
		}
		if v := b.pumping(DefOfCond); v < 1000 {
			out = append(out, Factor{Label: fmt.Sprintf("Blood pumping (weight %d%%)", pumpingWeight/10), Pct: v / 10})
		}
		if v := b.filtration(DefOfCond); v < 1000 {
			out = append(out, Factor{Label: fmt.Sprintf("Blood filtration (weight %d%%)", filtrationWeight/10), Pct: v / 10})
		}
		if s := b.bloodStage(); s > 0 {
			out = append(out, Factor{Label: "Blood loss: " + bloodStageLabels[s], Pct: -bloodStages[s-1].Cons / 10})
		}
	}
	for _, x := range b.Conds {
		st, ok := stageOf(x, DefOfCond)
		if !ok {
			continue
		}
		for _, m := range st.Mods {
			if m.Cap != c {
				continue
			}
			name := upperFirst(DefOfCond(x.Kind).Label)
			if st.Label != "" {
				name += " (" + st.Label + ")"
			}
			if m.Off != 0 {
				out = append(out, Factor{Label: name, Pct: m.Off / 10})
			}
			if m.HasMax {
				out = append(out, Factor{Label: name + ": at most", Pct: m.Max / 10})
			}
		}
	}
	return append(out, Factor{Label: "=", Pct: b.Capacity(c) / 10})
}
