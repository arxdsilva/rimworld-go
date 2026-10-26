package drug

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/trait"
)

// TestMedicalDefs (MED-07): numbroot is a mild painkiller (no addiction,
// tolerance or overdose), prophyl protects for five days with a small
// overdose, and the spark tablet is a hard drug.
func TestMedicalDefs(t *testing.T) {
	n, _ := DefOf(item.NumbrootTincture)
	if !n.Painkiller || n.Recreational || n.AddictChance != 0 || n.ToleranceGain != 0 || n.OverdoseMax != 0 {
		t.Fatalf("numbroot %+v", n)
	}
	p, _ := DefOf(item.ProphylTablet)
	if !p.Protects || p.Recreational || p.ScheduleDays != 5 || p.HighHours != 120 || p.OverdoseMin != 80 || p.OverdoseMax != 140 {
		t.Fatalf("prophyl %+v", p)
	}
	s, _ := DefOf(item.SparkTablet)
	if s.OverdoseMin != 180 || s.OverdoseMax != 350 {
		t.Fatalf("spark %+v", s)
	}
	if m := Medical(); len(m) != 2 || m[0].Kind != item.NumbrootTincture || m[1].Kind != item.ProphylTablet {
		t.Fatalf("medical %+v", m)
	}
}

// TestProphylProtectsFiveDays: a dose protects (no high thought, a
// "protected" line with the share left) until five days are up.
func TestProphylProtectsFiveDays(t *testing.T) {
	var s State
	if s.Protected(0) || (*State)(nil).Protected(0) {
		t.Fatal("no dose, no protection")
	}
	s.Ingest(item.ProphylTablet, always, 10)
	if len(s.Addictions) != 0 || s.Tolerance != nil {
		t.Fatalf("prophyl hooks no one: %+v", s)
	}
	if !s.Protected(10) || !s.Protected(10+120*hr-1) || s.Protected(10+120*hr) {
		t.Fatal("protected for exactly five days")
	}
	for _, k := range s.Thoughts() {
		if k == mood.HighOnDrug {
			t.Fatal("prophyl is no high")
		}
	}
	s.Tick(60, 10+60*hr)
	l := s.Lines()
	if len(l) != 1 || l[0].Kind != LineProtected || l[0].Label != "protected by prophyl" || l[0].Percent != 50 {
		t.Fatalf("lines %+v", l)
	}
	s.Tick(60, 10+120*hr)
	if s.Protected(10+120*hr) || len(s.Lines()) != 0 {
		t.Fatal("worn off")
	}
}

// TestAllowsMedical: "no drugs" refuses both; otherwise the painkiller
// needs pain at the line and no dose acting, prophyl is due every five
// days, abstainers included; recreation never picks a medical drug.
func TestAllowsMedical(t *testing.T) {
	s := &State{}
	for _, p := range []Preset{SocialDrinking, Unrestricted} {
		if !p.AllowsMedical(item.NumbrootTincture, PainkillerAt, false, s, 0) {
			t.Fatalf("%v: in pain", p)
		}
		if p.AllowsMedical(item.NumbrootTincture, PainkillerAt-1, false, s, 0) {
			t.Fatalf("%v: under the pain line", p)
		}
		if p.AllowsMedical(item.NumbrootTincture, 1000, true, s, 0) {
			t.Fatalf("%v: a dose still acts", p)
		}
		if !p.AllowsMedical(item.ProphylTablet, 0, false, s, 0) {
			t.Fatalf("%v: never taken: due", p)
		}
		if p.Allows(item.ProphylTablet, true, s, []trait.ID{trait.Hedonist}, 0) || p.Allows(item.NumbrootTincture, true, s, nil, 0) {
			t.Fatalf("%v: medical drugs are not for fun", p)
		}
	}
	if NoDrugs.AllowsMedical(item.NumbrootTincture, 1000, false, s, 0) || NoDrugs.AllowsMedical(item.ProphylTablet, 0, false, s, 0) {
		t.Fatal("no drugs means none")
	}
	if SocialDrinking.AllowsMedical(item.Beer, 1000, false, s, 0) || SocialDrinking.AllowsMedical(item.Wood, 1000, false, s, 0) ||
		SocialDrinking.AllowsMedical(item.ProphylTablet, 0, false, nil, 0) {
		t.Fatal("beer, wood and no state are no medicine")
	}
	s.Ingest(item.ProphylTablet, always, 100)
	day := 24 * hr
	if SocialDrinking.AllowsMedical(item.ProphylTablet, 0, false, s, 100+5*day-1) {
		t.Fatal("not due before five days")
	}
	if !SocialDrinking.AllowsMedical(item.ProphylTablet, 0, false, s, 100+5*day) {
		t.Fatal("due after five days")
	}
}

// TestNumbrootHasNoHigh: numbroot leaves no high and no tolerance; its
// effect lives on the body.
func TestNumbrootHasNoHigh(t *testing.T) {
	var s State
	s.Ingest(item.NumbrootTincture, always, 5)
	if len(s.Highs) != 0 || len(s.Addictions) != 0 || s.Tolerance != nil || s.LastDoseOf(item.NumbrootTincture) != 5 {
		t.Fatalf("%+v", s)
	}
}
