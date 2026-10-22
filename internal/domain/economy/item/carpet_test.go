package item

import "testing"

func TestCarpetKinds(t *testing.T) {
	t.Parallel()
	if len(Carpets()) != 5 { // straw, wood, leather, cloth, wool (SIM-026)
		t.Fatalf("carpets = %v", Carpets())
	}
	for _, c := range Carpets() {
		if !c.IsCarpet() || c.IsStuff() || c.IsFood() || c.Value() <= 0 {
			t.Fatalf("%s: carpet=%v stuff=%v value=%d", c, c.IsCarpet(), c.IsStuff(), c.Value())
		}
		if k, ok := ParseKind(c.String()); !ok || k != c {
			t.Fatalf("parse %q = %v %v", c.String(), k, ok)
		}
	}
	for _, k := range []Kind{Cloth, Straw, Wood, Leather} {
		if k.IsCarpet() {
			t.Fatalf("%s is not a carpet", k)
		}
		if k == Cloth || k == Straw {
			if k.IsStuff() || k.Value() <= 0 {
				t.Fatalf("%s stuff=%v value=%d", k, k.IsStuff(), k.Value())
			}
		}
	}
}
