package body

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func factorText(fs []Factor) string {
	var s []string
	for _, f := range fs {
		s = append(s, f.Label)
	}
	return strings.Join(s, "\n")
}

// TestCapacityFactorsExplainCapacity: the lines name the missing leg, the
// implant, pain and a condition, and the last one is Capacity itself.
func TestCapacityFactorsExplainCapacity(t *testing.T) {
	var b Body
	b.destroy(LegL)
	b.destroy(LegR)
	b.SetImplant(LegR, item.PegLeg)
	b.Hurt(fixed(0), 10, Cut, 1)
	b.AddCond(Condition{Kind: CondBadBack})
	fs := b.CapacityFactors(Moving)
	txt := factorText(fs)
	for _, want := range []string{"Left leg: missing", "Right leg: peg leg", "Bad back"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("missing %q in\n%s", want, txt)
		}
	}
	if last := fs[len(fs)-1]; last.Label != "=" || last.Pct != b.Capacity(Moving)/10 {
		t.Fatalf("last %v, capacity %d", last, b.Capacity(Moving))
	}
	var b2 Body
	b2.Hurt(fixed(0), 10, Cut, 1)
	if !strings.Contains(factorText(b2.CapacityFactors(Consciousness)), "Pain") {
		t.Fatal("consciousness names pain")
	}
	for c := Capacity(0); c < NumCaps; c++ {
		fs := b.CapacityFactors(c)
		if fs[len(fs)-1].Pct != b.Capacity(c)/10 {
			t.Fatalf("capacity %d", c)
		}
	}
}
