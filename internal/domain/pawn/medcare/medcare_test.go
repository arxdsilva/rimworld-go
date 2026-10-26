package medcare

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestAllowsCapsByPolicy(t *testing.T) {
	cases := []struct {
		p              Policy
		herbal, indust bool
	}{
		{NoCare, false, false},
		{NoMeds, false, false},
		{HerbalMax, true, false},
		{IndustrialMax, true, true},
		{Best, true, true},
	}
	for _, c := range cases {
		if got := Allows(c.p, item.HerbalMedicine); got != c.herbal {
			t.Errorf("Allows(%s, herbal) = %v, want %v", c.p.Label(), got, c.herbal)
		}
		if got := Allows(c.p, item.IndustrialMedicine); got != c.indust {
			t.Errorf("Allows(%s, industrial) = %v, want %v", c.p.Label(), got, c.indust)
		}
		if Allows(c.p, item.Wood) || Allows(c.p, item.Food) {
			t.Errorf("Allows(%s) accepts something that is not medicine", c.p.Label())
		}
	}
	if Allows(NumPolicies, item.HerbalMedicine) {
		t.Error("an invalid policy allows medicine")
	}
}

func TestMaxMedicineMatchesAllows(t *testing.T) {
	for p := Policy(0); p < NumPolicies; p++ {
		m := MaxMedicine(p)
		if p <= NoMeds {
			if m != item.Food {
				t.Errorf("MaxMedicine(%s) = %v, want none", p.Label(), m)
			}
			continue
		}
		if !Allows(p, m) {
			t.Errorf("MaxMedicine(%s) = %v, which it does not allow", p.Label(), m)
		}
	}
}

func TestLabelsAndDefaults(t *testing.T) {
	seen := map[string]bool{}
	for p := Policy(0); p < NumPolicies; p++ {
		l := p.Label()
		if l == "?" || seen[l] {
			t.Errorf("policy %d label %q", p, l)
		}
		seen[l] = true
		if p.Tends() != (p != NoCare) {
			t.Errorf("%s.Tends() = %v", l, p.Tends())
		}
	}
	if NumPolicies.Label() != "?" || NumPolicies.Valid() || NumPolicies.Tends() {
		t.Error("NumPolicies is not a policy")
	}
	want := [NumGroups]Policy{Best, HerbalMax, HerbalMax, HerbalMax}
	if Defaults() != want {
		t.Errorf("Defaults() = %v, want %v", Defaults(), want)
	}
	for g := Group(0); g < NumGroups; g++ {
		if g.Label() == "?" {
			t.Errorf("group %d has no label", g)
		}
	}
	if NumGroups.Label() != "?" {
		t.Error("NumGroups has a label")
	}
}
