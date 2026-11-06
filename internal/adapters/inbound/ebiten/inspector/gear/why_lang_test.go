package gear

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/application/game/sense"
)

// frFacts is the fake's facts speaking another language for the
// simulation's refusal reasons.
type frFacts struct{ *fake }

func (frFacts) T(src string) string {
	switch src {
	case sense.WhyDown:
		return "À terre ou mort"
	case sense.WhyPolicy:
		return "La politique l'interdit"
	}
	return src
}

// The Drop and Eat refusals are game/sense Why* keys, drawn through the
// catalog (English: the reason unchanged, TestDisabledButtonsSayWhy).
func TestRefusalReasonsAreTranslated(t *testing.T) {
	t.Parallel()
	f := newFake()
	p := kitted()
	p.Gear.NoDrop = sense.WhyDown
	for i := range p.Pack {
		if p.Pack[i].ID == 12 {
			p.Pack[i].Gear.Why = sense.WhyPolicy
		}
	}
	r := gfx.Rect{X: 0, Y: 0, W: 456, H: 2000}
	f.frameWith(func() { New(f, frFacts{f}).Declare(p, r) })
	for id, why := range map[string]string{DropID("weapon", 0): "À terre ou mort", EatID(12): "La politique l'interdit"} {
		if w, ok := f.kit.Find(id); !ok || !w.Disabled || w.Reason != why {
			t.Fatalf("%s: %+v, want disabled with %q", id, w, why)
		}
	}
}
