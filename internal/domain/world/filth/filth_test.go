package filth

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func TestGainWetnessAndGrass(t *testing.T) {
	t.Parallel()
	if Gain(StepGain, 0, false) != 6 || Gain(StepGain, 1000, false) != 18 || Gain(StepGain, 500, false) != 12 {
		t.Fatalf("wet gain %d %d %d", Gain(StepGain, 0, false), Gain(StepGain, 500, false), Gain(StepGain, 1000, false))
	}
	if Gain(StepGain, 0, true) != 3 || Gain(StepGain, 5000, false) != 18 || Gain(StepGain, -3, false) != 6 {
		t.Fatal("grass halves; wetness clamps")
	}
	if Carry(995, 18) != Full || Carry(0, 6) != 6 {
		t.Fatal("carry caps at Full")
	}
	if Shed(100) != 97 || Shed(0) != 0 {
		t.Fatal("shed")
	}
}

func TestDepositGeometric(t *testing.T) {
	t.Parallel()
	if k, d := Deposit(DepositMin - 1); k != DepositMin-1 || d != 0 {
		t.Fatal("too little to fall")
	}
	c, total, steps := 500, 0, 0
	for c >= DepositMin {
		var d int
		c, d = Deposit(c)
		if d < 1 {
			t.Fatal("a deposit drops at least 1")
		}
		total += d
		steps++
	}
	if total+c != 500 || steps < 20 || steps > 80 {
		t.Fatalf("trail: %d dropped over %d steps, %d kept", total, steps, c)
	}
}

func TestWipeOrderClothLeatherStrawWood(t *testing.T) {
	t.Parallel()
	prev := Full + 1
	for _, k := range []item.Kind{item.CarpetWood, item.CarpetStraw, item.CarpetLeather, item.CarpetCloth} {
		keep, soil := Wipe(Full, Catch(k))
		if keep >= prev || soil != (Full-keep)*MatSoilPct/100 {
			t.Fatalf("%s: keep %d soil %d", k, keep, soil)
		}
		prev = keep
	}
	if Catch(item.Wood) != 0 {
		t.Fatal("only carpets catch")
	}
	if keep, soil := Wipe(Full, Catch(item.CarpetCloth)); keep != 300 || soil != 105 {
		t.Fatalf("cloth: keep %d soil %d", keep, soil)
	}
}

func TestPenaltyMonotonic(t *testing.T) {
	t.Parallel()
	if Penalty(0, 20) != 0 || Penalty(100, 0) != 0 {
		t.Fatal("clean or empty rooms lose nothing")
	}
	prev := 0
	for sum := 0; sum <= 20*penaltyAvg*2; sum += 50 {
		p := Penalty(sum, 20)
		if p < prev || p > MaxPenalty {
			t.Fatalf("penalty %d at %d", p, sum)
		}
		prev = p
	}
	if prev != MaxPenalty || Penalty(20*penaltyAvg, 20) != MaxPenalty {
		t.Fatal("filthy rooms lose the most")
	}
	for avg, want := range map[int]string{0: "clean", 14: "clean", 15: "slightly dirty", 60: "dirty", 150: "filthy"} {
		if Label(avg) != want {
			t.Fatalf("label(%d) = %s", avg, Label(avg))
		}
	}
}

func TestCleanStepReachesZero(t *testing.T) {
	t.Parallel()
	d, s := 35, 50
	for i := 0; i < 10 && d+s > 0; i++ {
		nd, ns := CleanStep(d, s)
		if nd+ns != max(0, d+s-CleanPerTick) || (d >= CleanPerTick && ns != s) {
			t.Fatalf("step %d: %d,%d -> %d,%d", i, d, s, nd, ns)
		}
		d, s = nd, ns
	}
	if d != 0 || s != 0 {
		t.Fatalf("left %d,%d", d, s)
	}
}
