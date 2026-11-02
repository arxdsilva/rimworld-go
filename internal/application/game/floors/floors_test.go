package floors

import "testing"

// TestLine (AX-3): the floor stats line is the room tally's numbers.
func TestLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ kind, stuff, want string }{
		{"sterile tile", "steel", "Beauty -1   Cleanliness +0.6"},
		{"gold tile", "gold", "Beauty +11   Cleanliness +0.2"},
		{"straw matting", "hay", "Beauty -1   Cleanliness -0.1"},
		{"stone tile", "marble blocks", "Beauty +1   Cleanliness 0"},
		{"floor", "wood", "Beauty 0   Cleanliness 0"},
		{"wall", "wood", ""},
	} {
		if got := Line(c.kind, c.stuff); got != c.want {
			t.Errorf("Line(%s, %s) = %q, want %q", c.kind, c.stuff, got, c.want)
		}
	}
}
