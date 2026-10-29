package mentalbreak

import (
	"strings"
	"testing"
)

func TestRisk(t *testing.T) {
	if RiskPerHour(Threshold) != 0 || RiskPerHour(50) != 0 {
		t.Error("content colonists don't break")
	}
	if RiskPerHour(10) <= RiskPerHour(15) || RiskPerHour(0) != 400 {
		t.Error("risk should rise as mood falls")
	}
	for _, k := range Kinds() {
		if d := DefOf(k); d.Label == "" || d.Duration == 0 {
			t.Errorf("break %d incomplete", k)
		}
	}
}

func TestKindValuesAppendOnly(t *testing.T) {
	want := map[Kind]uint8{None: 0, SadWander: 1, FoodBinge: 2, HideInRoom: 3, SocialFight: 4,
		InsultingSpree: 5, TargetedInsult: 6, Berserk: 7, MurderousRage: 8, Catatonic: 9}
	for k, v := range want {
		if uint8(k) != v {
			t.Errorf("kind %q = %d, want %d", k, k, v)
		}
	}
}

func TestKindsUnchanged(t *testing.T) {
	got := Kinds()
	want := []Kind{SadWander, FoodBinge, HideInRoom}
	if len(got) != len(want) {
		t.Fatalf("Kinds() = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Kinds() = %v", got)
		}
	}
	if Threshold != 20 || RiskPerHour(0) != 400 || RiskPerHour(19) != 20 {
		t.Error("Threshold/RiskPerHour changed")
	}
}

func TestNewKindsDefined(t *testing.T) {
	for _, k := range []Kind{InsultingSpree, TargetedInsult, Berserk, MurderousRage, Catatonic} {
		d := DefOf(k)
		if d.Label == "" || d.Duration == 0 || d.Weight <= 0 {
			t.Errorf("kind %d incomplete: %+v", k, d)
		}
		if strings.Count(d.Message, "%s") != 1 {
			t.Errorf("kind %d message %q needs one %%s", k, d.Message)
		}
		if d.MinDuration == 0 || d.MaxDuration < d.MinDuration {
			t.Errorf("kind %d bad range %d..%d", k, d.MinDuration, d.MaxDuration)
		}
	}
}

func TestTierOf(t *testing.T) {
	cases := map[Kind]Tier{
		None: TierNone, SocialFight: TierNone,
		SadWander: Minor, FoodBinge: Minor, InsultingSpree: Minor,
		HideInRoom: Minor, TargetedInsult: Minor, Berserk: Extreme,
		MurderousRage: Extreme, Catatonic: Extreme,
	}
	for k, want := range cases {
		if got := TierOf(k); got != want {
			t.Errorf("TierOf(%d) = %d, want %d", k, got, want)
		}
	}
}

func TestTierAtBoundaries(t *testing.T) {
	th := Thresholds{Minor: 35, Major: 20, Extreme: 5}
	cases := []struct {
		mood int
		want Tier
	}{
		{100, TierNone}, {36, TierNone}, {35, TierNone}, {34, Minor},
		{21, Minor}, {20, Minor}, {19, Major}, {6, Major}, {5, Major}, {4, Extreme}, {0, Extreme},
	}
	for _, c := range cases {
		if got := TierAt(c.mood, th); got != c.want {
			t.Errorf("TierAt(%d) = %d, want %d", c.mood, got, c.want)
		}
	}
	d := DefaultThresholds()
	if !(d.Minor > d.Major && d.Major > d.Extreme && d.Extreme > 0) {
		t.Errorf("DefaultThresholds not ordered: %+v", d)
	}
}

func TestPickDeterministicAndTiered(t *testing.T) {
	all := func(Kind) bool { return true }
	for _, tier := range []Tier{Minor, Major, Extreme} {
		for roll := uint32(0); roll < 500; roll++ {
			k := Pick(tier, roll, all)
			if k == None || (tier != Major && TierOf(k) != tier) {
				t.Fatalf("Pick(%d,%d) = %d", tier, roll, k)
			}
			if k != Pick(tier, roll, all) {
				t.Fatal("Pick not deterministic")
			}
		}
	}
	if Pick(TierNone, 1, all) != None {
		t.Error("TierNone must pick nothing")
	}
}

func TestPickRespectsAllowedAndWeights(t *testing.T) {
	for _, tier := range []Tier{Minor, Major, Extreme} {
		var members []Kind
		total := 0
		for k := Kind(1); k < numKinds; k++ {
			if TierOf(k) == tier {
				members = append(members, k)
				total += DefOf(k).Weight
			}
		}
		if len(members) == 0 {
			continue
		}
		// every roll in one full weight cycle: counts match weights
		count := map[Kind]int{}
		for roll := 0; roll < total; roll++ {
			count[Pick(tier, uint32(roll), func(Kind) bool { return true })]++
		}
		for _, k := range members {
			if count[k] != DefOf(k).Weight {
				t.Errorf("tier %d kind %d picked %d times, weight %d", tier, k, count[k], DefOf(k).Weight)
			}
		}
		// banning one kind never yields it; banning all yields None
		for _, banned := range members {
			for roll := uint32(0); roll < 200; roll++ {
				if Pick(tier, roll, func(k Kind) bool { return k != banned }) == banned && len(members) > 0 {
					t.Fatalf("banned kind %d picked", banned)
				}
			}
		}
		if Pick(tier, 7, func(Kind) bool { return false }) != None {
			t.Error("nothing allowed must give None")
		}
	}
	if Pick(Minor, 3, nil) == None {
		t.Error("nil allowed means everything allowed")
	}
}

func TestDurationForRange(t *testing.T) {
	for k := Kind(1); k < numKinds; k++ {
		d := DefOf(k)
		for _, roll := range []uint32{0, 1, 7, 1000, 1<<32 - 1} {
			got := DurationFor(k, roll)
			if got < d.MinDuration || got > d.MaxDuration {
				t.Errorf("DurationFor(%d,%d)=%d outside %d..%d", k, roll, got, d.MinDuration, d.MaxDuration)
			}
		}
		if DurationFor(k, 5) != DurationFor(k, 5) {
			t.Error("not deterministic")
		}
	}
	if DurationFor(None, 5) != 0 {
		t.Error("None has no duration")
	}
}

func TestTargetsOthersAndDowns(t *testing.T) {
	for k := None; k < numKinds; k++ {
		wantT := k == InsultingSpree || k == TargetedInsult || k == Berserk || k == MurderousRage
		if TargetsOthers(k) != wantT {
			t.Errorf("TargetsOthers(%d) = %v", k, !wantT)
		}
		if Downs(k) != (k == Catatonic) {
			t.Errorf("Downs(%d) wrong", k)
		}
	}
}

func TestAllMoodKinds(t *testing.T) {
	got := AllMoodKinds()
	if len(got) != 8 {
		t.Fatalf("AllMoodKinds = %v", got)
	}
	for _, k := range got {
		if k == SocialFight || k == None {
			t.Errorf("AllMoodKinds contains %d", k)
		}
	}
}

func TestPickFallsBackToLowerTier(t *testing.T) {
	for roll := uint32(0); roll < 100; roll++ {
		if k := Pick(Major, roll, nil); TierOf(k) != Minor {
			t.Fatalf("Major fell back to %d", k)
		}
	}
	noExtreme := func(k Kind) bool { return TierOf(k) != Extreme }
	if k := Pick(Extreme, 3, noExtreme); k == None || TierOf(k) != Minor {
		t.Errorf("Extreme with none allowed gave %d", k)
	}
	if Pick(Extreme, 3, func(Kind) bool { return false }) != None {
		t.Error("nothing allowed must give None")
	}
}

func TestMTBTicks(t *testing.T) {
	if MTBTicks(Minor) != 240000 || MTBTicks(Major) != 48000 || MTBTicks(Extreme) != 30000 || MTBTicks(TierNone) != 0 {
		t.Error("MTB values")
	}
}

func TestShiftThresholds(t *testing.T) {
	d := DefaultThresholds()
	if got := ShiftThresholds(d, 0); got != (Thresholds{35, 20, 5}) {
		t.Errorf("no offset: %+v", got)
	}
	if got := ShiftThresholds(d, OffsetVolatile); got.Minor != 50 {
		t.Errorf("clamp high: %+v", got)
	}
	if got := ShiftThresholds(d, OffsetIronWilled); got != (Thresholds{17, 9, 2}) {
		t.Errorf("iron-willed: %+v", got)
	}
	if got := ShiftThresholds(d, -100); got.Minor != 1 {
		t.Errorf("clamp low: %+v", got)
	}
}

func TestLegacyDurationsKept(t *testing.T) {
	if DefOf(SadWander).Duration != 8*2500 || DefOf(FoodBinge).Duration != 4*2500 ||
		DefOf(HideInRoom).Duration != 10*2500 || DefOf(SocialFight).Duration != 420 {
		t.Error("legacy durations changed")
	}
}
