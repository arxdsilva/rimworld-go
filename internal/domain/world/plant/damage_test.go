package plant

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// step is one climate window of the staggered plant pass (250 ticks).
const step = 250

// exposeFor runs p through windows of air at temp until it dies or
// limit ticks pass, from tick start; it returns the plant and the ticks
// it took.
func exposeFor(p Plant, temp int, start, limit uint64) (Plant, uint64) {
	t := start
	for t-start < limit && !p.Dead() {
		p, _ = p.Expose(temp, t, t+step)
		t += step
	}
	return p, t - start
}

// cornAt0 is a corn plant at (0, 0) with its own frost limit (FOOD-2),
// written into d.FrostKill so the numbers below read as before.
func cornAt0() Def {
	d := DefOf(Corn)
	d.FrostKill, _ = Plant{Species: Corn}.ColdLimit()
	return d
}

func TestColdDamageKillsOverHours(t *testing.T) {
	d := cornAt0()
	hour := uint64(clock.TicksPerHour)
	// just below the frost limit: about four hours to die
	p, took := exposeFor(Plant{Species: Corn}, d.FrostKill-1, 0, 10*hour)
	if !p.Dead() || took < 37*hour/10 || took > 4*hour {
		t.Fatalf("at the threshold: dead %v after %d ticks, want about 4 h", p.Dead(), took)
	}
	if p.DamageHeat || !p.Damaging {
		t.Fatalf("cold damage flags: %+v", p)
	}
	// deeper is faster; a hard freeze kills in under two hours (the cap)
	_, mid := exposeFor(Plant{Species: Corn}, d.FrostKill-30, 0, 10*hour)
	_, hard := exposeFor(Plant{Species: Corn}, d.FrostKill-70, 0, 10*hour)
	_, deep := exposeFor(Plant{Species: Corn}, d.FrostKill-300, 0, 10*hour)
	if mid >= took || hard >= mid || hard >= 2*hour || deep != hard {
		t.Fatalf("death in %d (-0.1°), %d (-3°), %d (-7°), %d (-30°) ticks", took, mid, hard, deep)
	}
	// rates are a percent an hour: 25 at the threshold, +5 a degree, 60 at most
	for _, c := range []struct{ excess, rate int }{{1, 255}, {10, 300}, {70, 600}, {500, 600}} {
		if got := damageRate(c.excess); got != c.rate {
			t.Fatalf("damageRate(%d) = %d, want %d", c.excess, got, c.rate)
		}
	}
}

func TestHeatDamageScorches(t *testing.T) {
	d := DefOf(Pine)
	hour := uint64(clock.TicksPerHour)
	p, took := exposeFor(Plant{Species: Pine}, d.HeatKill+1, 0, 10*hour)
	if !p.Dead() || !p.DamageHeat || took > 4*hour || took < 37*hour/10 {
		t.Fatalf("pine at its heat limit: %+v after %d ticks", p, took)
	}
	// the pine shrugs off any cold; a jungle tree dies of it
	if q, _ := exposeFor(Plant{Species: Pine}, -600, 0, 10*hour); q.Damage != 0 {
		t.Fatalf("pine took cold damage: %+v", q)
	}
	if q, _ := exposeFor(Plant{Species: JungleTree}, DefOf(JungleTree).ColdKill-1, 0, 10*hour); !q.Dead() || q.DamageHeat {
		t.Fatalf("jungle tree in the cold: %+v", q)
	}
}

func TestDormantPlantsTakeNoDamage(t *testing.T) {
	for _, s := range []Species{Tree, BerryBush, FireLily} {
		for _, temp := range []int{-900, 900} {
			if s == FireLily && temp > 0 {
				continue // fire lily has a heat limit
			}
			p, _ := exposeFor(Plant{Species: s}, temp, 0, 20*clock.TicksPerHour)
			if p.Damage != 0 || p.Damaging {
				t.Fatalf("%v at %d: %+v", s, temp, p)
			}
		}
	}
}

func TestDamageHealsOnlyBelowDying(t *testing.T) {
	d := DefOf(Corn)
	hour := uint64(clock.TicksPerHour)
	warm := d.FrostKill + DamageStopMargin + 50
	// 40% damage, then safe air: heals 10% an hour
	p := Plant{Species: Corn, Damage: 40, Damaging: true}
	p, _ = p.Expose(warm, 0, hour)
	if p.Damaging || p.Damage != 30 {
		t.Fatalf("an hour of safe air from 40%%: %+v, want 30%% and not damaging", p)
	}
	p, _ = exposeFor(p, warm, hour, 5*hour)
	if p.Damage != 0 || p.DamageHeat || p.Dead() {
		t.Fatalf("healed fully: %+v", p)
	}
	// from 60% on it keeps dying, warm or not
	q := Plant{Species: Corn, Damage: DamageDying, Damaging: true}
	q, took := exposeFor(q, warm, 0, 10*hour)
	if !q.Dead() || took > 2*hour {
		t.Fatalf("a dying plant in warm air: %+v after %d ticks", q, took)
	}
	// just under it, it heals
	r, _ := Plant{Species: Corn, Damage: DamageDying - 1}.Expose(warm, 0, hour)
	if r.Damage >= DamageDying-1 {
		t.Fatalf("59%% in safe air did not heal: %+v", r)
	}
}

// TestDamageDoesNotFlap: air hovering at the kill temperature keeps the
// plant damaging (it never starts and stops by turns), and air hovering
// at the stop line keeps a recovering plant healing.
func TestDamageDoesNotFlap(t *testing.T) {
	d := cornAt0()
	p := Plant{Species: Corn}
	flips, starts, was := 0, 0, false
	at := uint64(0)
	for i := 0; i < 60 && !p.Dead(); i++ {
		temp := d.FrostKill - 1 // just past the kill line, then just short of the stop line
		if i%2 == 1 {
			temp = d.FrostKill + DamageStopMargin - 1
		}
		var started bool
		p, started = p.Expose(temp, at, at+step)
		at += step
		if started {
			starts++
		}
		if p.Damaging != was {
			flips++
			was = p.Damaging
		}
	}
	if flips != 1 || starts != 1 || !p.Dead() {
		t.Fatalf("hovering at the threshold: %d flips, %d starts, %+v", flips, starts, p)
	}
	// stop line: once stopped (2 °C back), air just under it doesn't
	// restart the damage; only the kill temperature does.
	q := Plant{Species: Corn, Damage: 30, Damaging: true}
	q, _ = q.Expose(d.FrostKill+DamageStopMargin, 0, step)
	if q.Damaging {
		t.Fatalf("damage did not stop at the stop line: %+v", q)
	}
	for i := 1; i < 20; i++ { // 19 more windows, alternating either side of the stop line
		temp := d.FrostKill + DamageStopMargin - 1 + 2*(i%2)
		before := q.Damage
		var started bool
		q, started = q.Expose(temp, uint64(i)*step, uint64(i+1)*step)
		if started || q.Damaging || (before > 0 && q.Damage > before) {
			t.Fatalf("restarted near the stop line at %d: %+v", temp, q)
		}
	}
	if q.Damage != 10 {
		t.Fatalf("healing 10%%/h over 5000 ticks from 30%%: %d, want 10", q.Damage)
	}
}

func TestExposeStartsOnce(t *testing.T) {
	p := Plant{Species: Corn}
	cold := cornAt0().FrostKill - 5
	p, started := p.Expose(cold, 0, step)
	if !started || p.Damage == 0 {
		t.Fatalf("first frosty window: started %v, %+v", started, p)
	}
	if _, again := p.Expose(cold, step, 2*step); again {
		t.Fatal("started again while damaging")
	}
	// windows add up to the whole span (the rate telescopes)
	a := Plant{Species: Corn}
	for at := uint64(0); at < 4000; at += step {
		a, _ = a.Expose(cold, at, at+step)
	}
	b, _ := Plant{Species: Corn}.Expose(cold, 0, 4000)
	if a.Damage != b.Damage {
		t.Fatalf("staggered windows %d vs one span %d", a.Damage, b.Damage)
	}
}

func TestDamagedYield(t *testing.T) {
	for _, c := range []struct {
		dmg     uint8
		qty, at int
	}{{0, 7, 7}, {30, 7, 4}, {50, 8, 4}, {99, 12, 0}, {100, 5, 0}, {10, 0, 0}} {
		if got := (Plant{Damage: c.dmg}).DamagedYield(c.qty); got != c.at {
			t.Fatalf("DamagedYield(%d) at %d%% = %d, want %d", c.qty, c.dmg, got, c.at)
		}
	}
}

// TestColdLimitPerPlant (FOOD-2, wiki: Plants): each crop dies of cold
// below its own limit between -18 and -10 °C; healroot only below -29;
// wild plants keep their biome limit.
func TestColdLimitPerPlant(t *testing.T) {
	t.Parallel()
	seen := map[int]bool{}
	for x := 0; x < 40; x++ {
		lim, ok := Plant{Species: Rice, X: x, Y: 3}.ColdLimit()
		if !ok || lim > -100 || lim < -180 {
			t.Fatalf("rice at %d: limit %d", x, lim)
		}
		seen[lim] = true
	}
	if len(seen) < 10 {
		t.Errorf("only %d different limits over 40 plants", len(seen))
	}
	if lim, _ := (Plant{Species: Healroot, X: 5}).ColdLimit(); lim != -290 {
		t.Errorf("healroot limit %d, want -290", lim)
	}
	if lim, ok := (Plant{Species: Pine}).ColdLimit(); ok || lim != 0 {
		t.Errorf("pine limit %d %v", lim, ok)
	}
}
