package item

import "testing"

// TestEveryKindHasAMass: every kind weighs something (a pod's capacity
// counts it), the weighed ones keep their wiki figures.
func TestEveryKindHasAMass(t *testing.T) {
	for k := Kind(0); k < numKinds; k++ {
		if k.MassGrams() <= 0 {
			t.Errorf("%s has no mass", k)
		}
	}
	for k, want := range map[Kind]int{Steel: 500, Silver: 8, Wood: 400, GraniteChunk: 25000, MarbleBlocks: 1000} {
		if got := k.MassGrams(); got != want {
			t.Errorf("%s weighs %d g, want %d", k, got, want)
		}
	}
}
