package construction

import "testing"

func TestDefsAreConsistent(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for i, d := range All() {
		if Kind(i) != d.Kind {
			t.Fatalf("def %d has kind %d", i, d.Kind)
		}
		if d.Name == "" || seen[d.Name] {
			t.Fatalf("def %d: empty or duplicate name %q", i, d.Name)
		}
		seen[d.Name] = true
		if d.Wood <= 0 || d.Work <= 0 {
			t.Fatalf("%s: cost and work must be positive", d.Name)
		}
		if k, ok := Parse(d.Name); !ok || k != d.Kind {
			t.Fatalf("Parse(%q) = %v %v", d.Name, k, ok)
		}
	}
	if !DefOf(Wall).BlocksMove || DefOf(Door).BlocksMove {
		t.Fatal("walls block movement, doors do not")
	}
	if DefOf(Floor).Layer != LayerFloor || DefOf(Table).Layer != LayerObject {
		t.Fatal("floor is the floor layer, furniture the object layer")
	}
	if _, ok := Parse("castle"); ok {
		t.Fatal("unknown names must not parse")
	}
}

func TestRefundAndFacing(t *testing.T) {
	t.Parallel()
	if Refund(Bed) != 1 || Refund(Floor) != 0 {
		t.Fatalf("refund bed=%d floor=%d", Refund(Bed), Refund(Floor))
	}
	if ParseFacing(9) != South || ParseFacing(uint8(West)) != West {
		t.Fatal("facing parse")
	}
	if West.String() != "west" {
		t.Fatal("facing string")
	}
}
