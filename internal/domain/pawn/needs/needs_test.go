package needs

import "testing"

func TestDecay_ReachesZero(t *testing.T) {
	p := Pair{Hunger: 3, Rest: 3}
	for i := 0; i < HungerPeriod*5; i++ {
		p = Decay(p, uint64(i+1))
	}
	if p.Hunger != 0 {
		t.Fatalf("hunger = %d", p.Hunger)
	}
}

func TestSleepTick_RestoresRest(t *testing.T) {
	p := Pair{Hunger: Full, Rest: 10}
	p = SleepTick(p, 1)
	if p.Rest != 10+SleepGain {
		t.Fatalf("rest = %d", p.Rest)
	}
}

func TestPercent(t *testing.T) {
	if Percent(Full) != 100 || Percent(0) != 0 {
		t.Fatal("percent bounds")
	}
}
