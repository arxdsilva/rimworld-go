package work

import "testing"

// Every work type has a distinct key that parses back (PLG-025).
func TestKeysRoundTrip(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for ty := Type(0); ty < NumTypes; ty++ {
		k := ty.Key()
		if k == "" || seen[k] {
			t.Fatalf("type %d: key %q empty or repeated", ty, k)
		}
		seen[k] = true
		if got, ok := ParseType(k); !ok || got != ty {
			t.Fatalf("ParseType(%q) = %d, %v", k, got, ok)
		}
	}
	if _, ok := ParseType("juggling"); ok || NumTypes.Key() != "" {
		t.Fatal("unknown keys and types must not resolve")
	}
}
