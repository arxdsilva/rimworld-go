package attack

import "testing"

func TestShooterAndSizeFactors(t *testing.T) {
	t.Parallel()
	if ShooterFactor(PerfectShooter, 30) != 1000 || ShooterFactor(ShooterPerTile(10), 0) != 1000 {
		t.Fatal("perfect shooter / no distance")
	}
	// 0.97^12 = 0.6938
	if got := ShooterFactor(ShooterPerTile(10), 12); got < 692 || got > 694 {
		t.Fatalf("skill 10 at 12: %d", got)
	}
	for in, want := range map[Permille]Permille{100: 500, 500: 500, 1000: 1000, 1800: 1800, 4000: 2000} {
		if SizeFactor(in) != want {
			t.Errorf("SizeFactor(%d) = %d", in, SizeFactor(in))
		}
	}
}

func TestReportAimAndTotal(t *testing.T) {
	t.Parallel()
	r := Report{Band: 800, Shooter: 1000, Weather: 1000, Size: 1000, Light: 1000}
	if r.Aim() != 800 || r.Total() != 800 {
		t.Fatalf("neutral %d %d", r.Aim(), r.Total())
	}
	r.Cover = 550
	if r.Total() != 800*450/1000 {
		t.Fatalf("cover %d", r.Total())
	}
	r.Weather, r.Size = FogFactor, 500
	if r.Aim() != 200 {
		t.Fatalf("fog and size %d", r.Aim())
	}
	r.Offset = 900
	if r.Aim() != 1000 {
		t.Fatal("offset clamps high")
	}
	r.Offset = -900
	if r.Aim() != 0 {
		t.Fatal("offset clamps low")
	}
	if m := (Report{Melee: true, Band: 850, Cover: 900}); m.Total() != 850 {
		t.Fatal("melee ignores cover")
	}
	// Report matches HitChance's product without cover
	b := DefaultBands(650, 800, 900, 800)
	rr := Report{Band: b.Accuracy(20), Shooter: ShooterFactor(ShooterPerTile(8), 20), Weather: 1000, Size: 1000, Light: 1000}
	if hc := HitChance(b, 20, ShooterPerTile(8), 0, 1000, 1000, 1000, 1000); rr.Aim() < hc-1 || rr.Aim() > hc+1 {
		t.Fatalf("aim %d vs HitChance %d", rr.Aim(), hc)
	}
}

func TestMissRadius(t *testing.T) {
	t.Parallel()
	for aim, want := range map[Permille]int{1000: 1, 999: 1, 220: 2, 110: 4, 90: 5, 70: 6, 40: 8, 20: 10, 0: 10} {
		if got := MissRadius(aim); got != want {
			t.Errorf("MissRadius(%d) = %d want %d", aim, got, want)
		}
	}
	for a := Permille(0); a < 1000; a += 7 {
		if MissRadius(a) < MissRadius(a+7) {
			t.Fatalf("miss radius grows with aim at %d", a)
		}
	}
}

func TestInterceptChance(t *testing.T) {
	t.Parallel()
	for size, want := range map[Permille]Permille{1000: 400, 50: 40, 3000: 800, 500: 200} {
		if InterceptChance(size) != want {
			t.Errorf("InterceptChance(%d) = %d", size, InterceptChance(size))
		}
	}
}
