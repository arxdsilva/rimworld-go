package damage

import "testing"

func TestResisted(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		t    Type
		want Category
	}{
		{Sharp, ArmorSharp}, {Blunt, ArmorBlunt}, {Heat, ArmorHeat},
		{Bomb, ArmorSharp}, {Burn, ArmorHeat}, {EMP, NoArmor}, {Type(99), NoArmor},
	} {
		if got := Resisted(tc.t); got != tc.want {
			t.Errorf("Resisted(%v)=%v want %v", tc.t, got, tc.want)
		}
	}
	if Sharp.String() != "sharp" || Type(99).String() != "unknown" {
		t.Fatal("names")
	}
}
