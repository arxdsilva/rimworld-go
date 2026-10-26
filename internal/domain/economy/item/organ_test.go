package item

import "testing"

// MED-05: the nanomed kit, the servo tier, the valve heart and the
// natural organs: names, groups, categories and wiki values.
func TestMedicalKindsMED05(t *testing.T) {
	t.Parallel()
	cases := []struct {
		k                    Kind
		name                 string
		implant, organ, meds bool
		cat                  string
		value                int
	}{
		{NanomedKit, "nanomed kit", false, false, true, "Medicine", 50},
		{ServoArm, "servo arm", true, false, false, "Manufactured", 1030},
		{ServoLeg, "servo leg", true, false, false, "Manufactured", 1030},
		{OpticImplant, "optic implant", true, false, false, "Manufactured", 1030},
		{CochlearImplant, "cochlear implant", true, false, false, "Manufactured", 785},
		{SpinalBrace, "spinal brace", true, false, false, "Manufactured", 1030},
		{PumpHeart, "pump heart", true, false, false, "Manufactured", 1030},
		{SynthStomach, "synth stomach", true, false, false, "Manufactured", 785},
		{ValveHeart, "valve heart", true, false, false, "Manufactured", 230},
		{Kidney, "kidney", false, true, false, "Manufactured", 900},
		{Lung, "lung", false, true, false, "Manufactured", 1000},
		{Heart, "heart", false, true, false, "Manufactured", 1200},
		{Liver, "liver", false, true, false, "Manufactured", 1200},
	}
	for _, c := range cases {
		if c.k.String() != c.name || c.k.IsImplant() != c.implant || c.k.IsOrgan() != c.organ || c.k.IsMedicine() != c.meds {
			t.Errorf("%v: implant %v organ %v medicine %v", c.k, c.k.IsImplant(), c.k.IsOrgan(), c.k.IsMedicine())
		}
		if got, ok := ParseKind(c.name); !ok || got != c.k {
			t.Errorf("ParseKind(%q) = %v", c.name, got)
		}
		if c.k.Category() != c.cat || c.k.Value() != c.value {
			t.Errorf("%v: category %q value %d", c.k, c.k.Category(), c.k.Value())
		}
		if c.k.IsFood() || c.k.IsDrug() {
			t.Errorf("%v is food or a drug", c.k)
		}
	}
	for _, k := range []Kind{HerbalMedicine, IndustrialMedicine} {
		if !k.IsMedicine() {
			t.Errorf("%v is medicine", k)
		}
	}
	if Components.IsMedicine() || PegLeg.IsOrgan() || !PegLeg.IsImplant() || !WoodenNose.IsImplant() {
		t.Fatal("older kinds changed group")
	}
}
