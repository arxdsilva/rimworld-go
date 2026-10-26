package item

import "testing"

// TestEveryImplantHasValue: every implant is worth something (MED-00 gave
// the prosthetic foot, glass eye and wooden nose the values they lacked).
func TestEveryImplantHasValue(t *testing.T) {
	want := map[Kind]int{ProstheticFoot: 90, GlassEye: 40, WoodenNose: 5}
	for k := Kind(1); k < numKinds; k++ {
		if !k.IsImplant() {
			continue
		}
		if k.Value() <= 0 {
			t.Fatalf("%v has no value", k)
		}
		if v, ok := want[k]; ok && k.Value() != v {
			t.Fatalf("%v value %d, want %d", k, k.Value(), v)
		}
	}
}
