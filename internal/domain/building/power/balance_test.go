package power

import (
	"math/rand/v2"
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

const wh = WTicksPerWh

func gen(id uint64, w int) Dev { return Dev{ID: id, Role: Producer, Out: w} }

func use(id uint64, c Class, w int) Dev { return Dev{ID: id, Role: Consumer, Class: c, Want: w} }

func bat(id uint64, stored, capacity int64) Dev {
	return Dev{ID: id, Role: Storage, Stored: stored, Cap: capacity}
}

func byID(devs []Dev, id uint64) Dev {
	for _, d := range devs {
		if d.ID == id {
			return d
		}
	}
	panic("no device")
}

func wantStored(t *testing.T, devs []Dev, want map[uint64]int64) {
	t.Helper()
	for id, s := range want {
		if got := byID(devs, id).Stored; got != s {
			t.Errorf("battery %d holds %d W·ticks, want %d", id, got, s)
		}
	}
}

// TestBalanceEnergy: generation covers the load; a surplus charges the
// batteries at 75% (split equally, the remainder to the lowest ids, the
// overflow of a full one to the others), a deficit drains them equally,
// and each battery leaks 8 W first.
func TestBalanceEnergy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		devs   []Dev
		dt     int
		out    NetOut
		stored map[uint64]int64
	}{
		{"surplus charges with the loss",
			[]Dev{gen(1, 500), use(2, ClassLamp, 100), bat(3, 0, 1200*wh)}, 50,
			NetOut{Gen: 500, Load: 100, Stored: 15000, Cap: 1200 * wh}, map[uint64]int64{3: 15000}},
		{"equal split, the remainder to the lowest ids",
			[]Dev{bat(9, 1000, wh), gen(1, 15), bat(3, 1000, wh), bat(7, 1000, wh)}, 1,
			// leak 8 each: 992; 15 W·ticks × 75% = 11: 4, 4, 3
			NetOut{Gen: 15, Stored: 3*992 + 11, Cap: 3 * wh}, map[uint64]int64{3: 996, 7: 996, 9: 995}},
		{"a full battery's share goes to the others",
			[]Dev{gen(90, 160), bat(1, 0, 1000), bat(2, 0, 10*wh), bat(3, 0, 10*wh)}, 50,
			// 160×50×75% = 6000: 2000 each, but 1 holds 1000; the 1000 left split 500/500
			NetOut{Gen: 160, Stored: 6000, Cap: 1000 + 20*wh}, map[uint64]int64{1: 1000, 2: 2500, 3: 2500}},
		{"full batteries waste the surplus",
			[]Dev{gen(90, 1000), bat(1, 1000, 1000)}, 50,
			NetOut{Gen: 1000, Stored: 1000, Cap: 1000}, map[uint64]int64{1: 1000}},
		{"a deficit drains equally",
			[]Dev{gen(1, 100), use(2, ClassBench, 300), bat(3, 10400, 1200*wh), bat(4, 30400, 1200*wh)}, 50,
			// leak 400 each: 10000 and 30000; 200 W × 50 = 10000 drawn, 5000 each
			NetOut{Gen: 100, Load: 300, Stored: 30000, Cap: 2400 * wh}, map[uint64]int64{3: 5000, 4: 25000}},
		{"an emptied battery's share comes from the others",
			[]Dev{use(2, ClassHeater, 700), bat(3, 10000, 1200*wh), bat(4, 30000, 1200*wh)}, 50,
			// 9600 + 29600 after the leak; 35000 drawn: 9600 from 3, 25400 from 4
			NetOut{Load: 700, Stored: 4200, Cap: 2400 * wh}, map[uint64]int64{3: 0, 4: 4200}},
		{"self-discharge 8 W each, never below 0",
			[]Dev{bat(1, 1000, wh), bat(2, 300, wh)}, 50,
			NetOut{Stored: 600, Cap: 2 * wh}, map[uint64]int64{1: 600, 2: 0}},
		{"over-full and negative charge are clamped",
			[]Dev{bat(1, 5000, 1000), bat(2, -50, 1000), bat(3, 10, -5)}, 1,
			NetOut{Stored: 992, Cap: 2000}, map[uint64]int64{1: 992, 2: 0, 3: 0}},
		{"no time, no energy moves",
			[]Dev{gen(90, 500), bat(1, 700, wh)}, 0,
			NetOut{Gen: 500, Stored: 700, Cap: wh}, map[uint64]int64{1: 700}},
		{"a negative span is no time",
			[]Dev{use(2, ClassLamp, 50), bat(1, 700, wh)}, -10,
			NetOut{Load: 50, Stored: 700, Cap: wh}, map[uint64]int64{1: 700}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			devs := slices.Clone(c.devs)
			if out := Balance(devs, c.dt, 1000); out != c.out {
				t.Errorf("Balance = %+v, want %+v", out, c.out)
			}
			wantStored(t, devs, c.stored)
		})
	}
}

// TestBalanceShedOrder: when the load exceeds generation plus what the
// batteries can give over the span, consumers are shed by class (lamps,
// benches, coolers, heaters), the newest first within a class, only until
// the rest fits; each stays off for ShedTicks.
func TestBalanceShedOrder(t *testing.T) {
	t.Parallel()
	consumers := []Dev{use(3, ClassHeater, 180), use(1, ClassLamp, 50), use(5, ClassCooler, 220),
		use(8, ClassLamp, 50), use(4, ClassBench, 200), use(6, ClassHeater, 180), use(2, ClassLamp, 50)}
	cases := []struct {
		name string
		devs []Dev
		shed []uint64
		load int
	}{
		{"short by 430 W: three lamps, the bench and the cooler", append([]Dev{gen(10, 500)}, consumers...),
			[]uint64{1, 2, 4, 5, 8}, 360},
		{"short by 50 W: the newest lamp", append([]Dev{gen(10, 880)}, consumers...), []uint64{8}, 880},
		{"a battery's charge counts over the span",
			append([]Dev{gen(10, 500), bat(11, 330*50+400, 1200*wh)}, consumers...), []uint64{8, 2}, 830},
		{"enough: nothing shed", append([]Dev{gen(10, 930)}, consumers...), nil, 930},
		{"nothing at all: everything off", slices.Clone(consumers), []uint64{1, 2, 3, 4, 5, 6, 8}, 0},
	}
	const now = 7000
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			devs := slices.Clone(c.devs)
			out := Balance(devs, 50, now)
			if out.Load != c.load || out.Brownout != (len(c.shed) > 0) {
				t.Errorf("Load %d brownout %v; want %d, %v", out.Load, out.Brownout, c.load, len(c.shed) > 0)
			}
			for _, d := range devs {
				if d.Role != Consumer {
					if d.Powered || d.ShedUntil != 0 {
						t.Errorf("%d: a %s is never powered or shed", d.ID, d.Role)
					}
					continue
				}
				shed := slices.Contains(c.shed, d.ID)
				if d.Powered == shed || (shed && d.ShedUntil != now+ShedTicks) || (!shed && d.ShedUntil != 0) {
					t.Errorf("consumer %d: powered %v until %d (shed %v)", d.ID, d.Powered, d.ShedUntil, shed)
				}
			}
		})
	}
}

// TestBalanceShedHold: a shed consumer stays off until ShedUntil however
// much power returns, then comes back only when the net carries it with
// 10% headroom (the batteries counted over ShedTicks, not just the span).
func TestBalanceShedHold(t *testing.T) {
	t.Parallel()
	heater := []Dev{gen(1, 100), use(7, ClassHeater, 180)}
	step := func(out int, now uint64) NetOut {
		heater[0].Out = out
		return Balance(heater, 50, now)
	}
	if o := step(100, 10000); o.Load != 0 || !o.Brownout || heater[1].Powered || heater[1].ShedUntil != 12500 {
		t.Fatalf("shed: %+v %+v", o, heater[1])
	}
	if o := step(10000, 10050); o.Load != 0 || !o.Brownout || heater[1].Powered || heater[1].ShedUntil != 12500 {
		t.Fatalf("held: %+v %+v", o, heater[1])
	}
	if o := step(197, 12500); o.Load != 0 || heater[1].Powered || heater[1].ShedUntil != 12500 {
		t.Fatalf("197 W lacks 10%% headroom over 180: %+v %+v", o, heater[1])
	}
	if o := step(198, 12550); o.Load != 180 || o.Brownout || !heater[1].Powered || heater[1].ShedUntil != 0 {
		t.Fatalf("198 W carries it: %+v %+v", o, heater[1])
	}

	// Batteries alone: 9899 W over the next 50 ticks, but only 197.99 W
	// over an hour, so the heater stays off; 198 W over an hour brings it.
	for _, c := range []struct {
		stored int64
		back   bool
	}{{198*ShedTicks - 1 + 400, false}, {198*ShedTicks + 400, true}} {
		devs := []Dev{bat(1, c.stored, 1200*wh), {ID: 7, Role: Consumer, Class: ClassHeater, Want: 180, ShedUntil: 500}}
		Balance(devs, 50, 500)
		if devs[1].Powered != c.back {
			t.Errorf("stored %d: back %v, want %v", c.stored, devs[1].Powered, c.back)
		}
	}

	// The most needed come back first: the heater, then the oldest lamp,
	// while the newer lamp waits for more headroom.
	devs := []Dev{gen(1, 286), {ID: 4, Role: Consumer, Class: ClassLamp, Want: 50, ShedUntil: 900},
		{ID: 2, Role: Consumer, Class: ClassLamp, Want: 50, ShedUntil: 900},
		{ID: 9, Role: Consumer, Class: ClassHeater, Want: 180, ShedUntil: 900}}
	// (180+50)×1.1 = 253 ≤ 286 < (180+100)×1.1 = 308
	if o := Balance(devs, 50, 1000); o.Load != 230 || !o.Brownout || !byID(devs, 9).Powered || !byID(devs, 2).Powered ||
		byID(devs, 4).Powered || byID(devs, 4).ShedUntil != 900 {
		t.Fatalf("return order: %+v %+v", o, devs)
	}
}

// TestBalanceSwitchedOff: a consumer that wants nothing (switched off)
// draws nothing and is not powered; its finished shed hold is dropped, a
// running one kept.
func TestBalanceSwitchedOff(t *testing.T) {
	t.Parallel()
	devs := []Dev{gen(1, 1000), {ID: 2, Role: Consumer, Class: ClassLamp},
		{ID: 3, Role: Consumer, Class: ClassLamp, ShedUntil: 400}, {ID: 4, Role: Consumer, Class: ClassLamp, ShedUntil: 600},
		{ID: 5, Role: Transmitter, Want: 50, Powered: true}}
	if o := Balance(devs, 50, 500); o.Load != 0 || o.Brownout {
		t.Fatalf("%+v", o)
	}
	for _, d := range devs {
		if d.Powered {
			t.Errorf("%d powered", d.ID)
		}
	}
	if byID(devs, 3).ShedUntil != 0 || byID(devs, 4).ShedUntil != 600 {
		t.Errorf("holds: %+v", devs)
	}
}

// TestBalanceOrderFree: the result does not depend on the order of devs.
func TestBalanceOrderFree(t *testing.T) {
	t.Parallel()
	base := []Dev{gen(1, 420), gen(2, 130), bat(3, 2000, wh), bat(4, 900, wh), bat(5, 77, wh)}
	for i := range 30 {
		base = append(base, use(uint64(10+i), Class(1+i%4), 20+7*i))
	}
	want := slices.Clone(base)
	wantOut := Balance(want, 50, 5000)
	r := rand.New(rand.NewPCG(4, 4))
	for range 25 {
		devs := slices.Clone(base)
		r.Shuffle(len(devs), func(i, j int) { devs[i], devs[j] = devs[j], devs[i] })
		if out := Balance(devs, 50, 5000); out != wantOut {
			t.Fatalf("%+v vs %+v", out, wantOut)
		}
		for _, w := range want {
			if got := byID(devs, w.ID); got != w {
				t.Fatalf("%+v vs %+v", got, w)
			}
		}
	}
}

// TestBalanceNoFlap is the AGENTS.md no-flap rule for brownouts:
// generation ripples ±5% around the load for two in-game days with the
// batteries empty (none, or one starting empty). Every consumer's off
// span lasts at least an hour and it flips at most once per hour, plus
// one (measured against the clock, not ShedTicks, so a shorter hold
// fails).
func TestBalanceNoFlap(t *testing.T) {
	t.Parallel()
	const (
		dt     = 50
		hour   = clock.TicksPerHour
		span   = 2 * clock.TicksPerDay
		maxFlp = span/hour + 1
	)
	consumers := []Dev{use(1, ClassLamp, 50), use(2, ClassLamp, 50), use(3, ClassLamp, 50), use(4, ClassLamp, 50),
		use(5, ClassBench, 250), use(6, ClassBench, 150), use(7, ClassBench, 200),
		use(8, ClassCooler, 220), use(9, ClassCooler, 220), use(10, ClassHeater, 180), use(11, ClassHeater, 180)}
	load := 0
	for _, c := range consumers {
		load += c.Want
	}
	for _, c := range []struct {
		name string
		bats []Dev
	}{{"no battery", nil}, {"an empty battery", []Dev{bat(100, 0, 1200*wh)}}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			devs := append(append([]Dev{gen(50, 0)}, c.bats...), consumers...)
			on := map[uint64]bool{}
			offAt := map[uint64]uint64{}
			flips := map[uint64]int{}
			sheds := 0
			for i := range consumers {
				on[consumers[i].ID] = true
			}
			for now := uint64(dt); now <= span; now += dt {
				// two triangle waves, ±5% of the load together
				r := (tri(now, 620) + tri(now, 3700)) / 2
				devs[0].Out = load + load*50*r/1_000_000
				out := Balance(devs, dt, now)
				if out.Stored < 0 || out.Stored > out.Cap {
					t.Fatalf("tick %d: stored %d of %d", now, out.Stored, out.Cap)
				}
				for _, d := range devs {
					if d.Role != Consumer || d.Powered == on[d.ID] {
						continue
					}
					flips[d.ID]++
					on[d.ID] = d.Powered
					if !d.Powered {
						sheds++
						offAt[d.ID] = now
					} else if off := now - offAt[d.ID]; off < hour {
						t.Fatalf("consumer %d off for %d ticks only (tick %d)", d.ID, off, now)
					}
				}
			}
			if sheds == 0 {
				t.Fatal("the ripple never shed anything: the test proves nothing")
			}
			for id, n := range flips {
				if n > maxFlp {
					t.Errorf("consumer %d flipped %d times in %d ticks", id, n, span)
				}
			}
			t.Logf("%d sheds, flips %v", sheds, flips)
		})
	}
}

// tri is a triangle wave of the given period in [-1000, 1000].
func tri(t uint64, period uint64) int {
	p := int(t % period)
	half := int(period / 2)
	if p < half {
		return -1000 + 2000*p/half
	}
	return 1000 - 2000*(p-half)/(int(period)-half)
}

// BenchmarkBalance is one net of a large colony: 4 producers, 6
// batteries and 120 consumers, some shed and waiting.
func BenchmarkBalance(b *testing.B) {
	devs := []Dev{gen(1, 2000), gen(2, 1800), gen(3, 900), gen(4, 400)}
	for i := range 6 {
		devs = append(devs, bat(uint64(10+i), int64(i)*100*wh, 1200*wh))
	}
	for i := range 120 {
		devs = append(devs, use(uint64(100+i), Class(1+i%4), 30+i))
	}
	b.ReportAllocs()
	now := uint64(0)
	for b.Loop() {
		now += 50
		Balance(devs, 50, now)
	}
}
