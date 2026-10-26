package body

import (
	"math/rand"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
)

// TestTendQualityFormula pins the wiki formula: the Medicine stat at
// levels 0, 8 and 20 × each medicine's potency, clamped to its max, with
// the random factor held at 100 %, then the bed, self-tend and spread.
func TestTendQualityFormula(t *testing.T) {
	t.Parallel()
	mid := func(int) int { return 250 } // 750 + 250 = ×1000
	tests := []struct {
		lvl  int
		med  item.Kind
		want int32
	}{
		{0, item.Food, 60}, {0, item.HerbalMedicine, 120}, {0, item.IndustrialMedicine, 200},
		{8, item.Food, 300}, {8, item.HerbalMedicine, 600}, {8, item.IndustrialMedicine, 1000},
		{20, item.Food, 465}, {20, item.HerbalMedicine, 700}, {20, item.IndustrialMedicine, 1000},
	}
	for _, tc := range tests {
		got := TendQuality(TendInput{Skill: tc.lvl, Manip: 1000, Sight: 1000, Medicine: tc.med, Roll: mid})
		if got != tc.want {
			t.Errorf("lvl %d %v: %d, want %d", tc.lvl, tc.med, got, tc.want)
		}
	}
	for lvl, want := range map[int]int32{0: 200, 1: 300, 8: 1000, 9: 1050, 18: 1500, 19: 1525, 20: 1550, 25: 1550} {
		if got := tendBase(lvl); got != want {
			t.Errorf("tendBase(%d) = %d, want %d", lvl, got, want)
		}
	}
	base := TendInput{Skill: 8, Manip: 1000, Sight: 1000, Medicine: item.HerbalMedicine, Roll: mid}
	in := base
	in.BedOffset = HospitalBedTendOffset
	if got := TendQuality(in); got != 700 {
		t.Errorf("herbal in a hospital bed: %d, want 600+100", got)
	}
	in = base
	in.Self = true
	if got := TendQuality(in); got != 420 {
		t.Errorf("self-tend: %d, want 600×0.7", got)
	}
	in = base
	in.Manip, in.Sight = 500, 0
	if got := TendQuality(in); got != 90 { // 1000×0.5×0.3×0.6
		t.Errorf("half hands, blind: %d, want 90", got)
	}
	in = base
	in.Roll = func(int) int { return 0 }
	if got := TendQuality(in); got != 450 {
		t.Errorf("low roll: %d, want 600×0.75", got)
	}
	in.Roll = func(n int) int { return n - 1 }
	if got := TendQuality(in); got != 700 {
		t.Errorf("high roll: %d, want 750 clamped to herbal's 700", got)
	}
}

func TestTendAllOncePerWindow(t *testing.T) {
	t.Parallel()
	b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 5, Kind: Cut}}, Fresh: bit(HandR)}
	b.AddCond(Condition{Kind: CondInfection, Part: ArmL, Sev: 100})
	if !b.TendDue(0) {
		t.Fatal("an untended body is due")
	}
	if n := b.TendAll(640, 10); n != 3 {
		t.Fatalf("tended %d things, want wound, stump and infection", n)
	}
	w, c := b.Wounds[0], b.Conds[0]
	if !w.Tended || w.TendQ != 640 || w.TendAt != 10 || c.TendQ != 640 || c.TendSum != 640 || c.TendUntil != 10+12*clock.TicksPerHour {
		t.Fatalf("wound %+v cond %+v", w, c)
	}
	if b.TendDue(11) || b.TendAll(900, 11) != 0 || b.Wounds[0].TendQ != 640 {
		t.Fatal("a second tend in the window changes nothing")
	}
	if !b.TendDue(c.TendUntil) {
		t.Fatal("the infection is due again once its window ends")
	}
	b.Wounds = append(b.Wounds, Wound{Part: LegL, Dmg: 2, Kind: Bite})
	if !b.TendDue(11) {
		t.Fatal("a new wound is due at once")
	}
}

func TestHealHourRates(t *testing.T) {
	t.Parallel()
	never := func(int) int { return 999 }
	day := func(b *Body, lying int32) {
		for h := 0; h < 24; h++ {
			b.HealHour(never, lying)
		}
	}
	b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 30, Kind: Cut}, {Part: LegL, Dmg: 30, Kind: Cut, Tended: true, TendQ: 500}}}
	day(&b, 0)
	if b.Wounds[0].Dmg != 22 || b.Wounds[1].Dmg != 30-16 {
		t.Fatalf("a day up: untended −8, tended at 50%% −16: %+v", b.Wounds)
	}
	for _, tc := range []struct {
		lying int32
		want  int
	}{{LieGround, 30 - 12}, {LieBed, 30 - 16}, {LieHospital, 30 - 22}} {
		b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 30, Kind: Cut}}}
		day(&b, tc.lying)
		if b.Wounds[0].Dmg != tc.want {
			t.Errorf("lying %d: %d left, want %d", tc.lying, b.Wounds[0].Dmg, tc.want)
		}
	}
	b = Body{Wounds: []Wound{{Part: ArmL, Dmg: 1, Kind: Cut, Size: 14}, {Part: Torso, Dmg: 4, Kind: Cut, Permanent: true}}}
	for h := 0; h < 3; h++ {
		b.HealHour(never, 0)
	}
	if len(b.Wounds) != 1 || !b.Wounds[0].Permanent {
		t.Fatalf("the cut closes clean, the scar stays: %+v", b.Wounds)
	}
	b = Body{Wounds: []Wound{{Part: ArmL, Dmg: 1, Kind: Cut, Size: 14}}}
	for h := 0; h < 3; h++ {
		if s := b.HealHour(func(int) int { return 0 }, 0); len(s) > 0 && s[0] != ArmL {
			t.Fatalf("scarred %v", s)
		}
	}
	if len(b.Wounds) != 1 || !b.Wounds[0].Permanent || b.Wounds[0].Dmg != scarDmg {
		t.Fatalf("a low roll on a deep cut scars: %+v", b.Wounds)
	}
}

// TestScarOddsFollowDamage: 20‰ × clamp((dmg−4)/10), only for wounds
// that can scar, and a seeded run of deep cuts scars at about that rate.
func TestScarOddsFollowDamage(t *testing.T) {
	t.Parallel()
	for dmg, want := range map[int]int32{0: 0, 4: 0, 5: 2, 9: 10, 14: 20, 40: 20} {
		if got := ScarOdds(Cut, dmg); got != want {
			t.Errorf("ScarOdds(cut, %d) = %d, want %d", dmg, got, want)
		}
	}
	if ScarOdds(Bruise, 30) != 0 || ScarOdds(Surgical, 30) != 0 || ScarOdds(Burn, 30) != 20 {
		t.Fatal("bruises and surgical cuts never scar; burns do")
	}
	r := rand.New(rand.NewSource(42))
	count := func(size int32) int {
		scars := 0
		for i := 0; i < 20000; i++ {
			b := Body{Wounds: []Wound{{Part: ArmL, Dmg: 1, Kind: Cut, Size: size}}}
			for h := 0; h < 3; h++ {
				scars += len(b.HealHour(r.Intn, 0))
			}
		}
		return scars
	}
	deep, shallow := count(20), count(4)
	if deep < 300 || deep > 500 || shallow != 0 {
		t.Fatalf("scars: deep %d/20000 (want ~400), shallow %d", deep, shallow)
	}
}
