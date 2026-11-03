package aqua

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
)

const hour = clock.TicksPerHour

// good is the conditions a stock thrives in.
var good = Conditions{Power: true, Air: temperature.C(18), Fed: true}

func stocked(s Species) Tank { return Tank{Species: s}.Fill() }

// run steps t for n hours under c, stopping at a death.
func run(t Tank, hours int, c Conditions) (Tank, Outcome) {
	var out Outcome
	for i := 0; i < hours; i++ {
		var o Outcome
		t, o = t.Step(hour, c)
		if o.Warn || o.Died {
			out = o
		}
		if o.Died {
			break
		}
	}
	return t, out
}

func TestDefs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		s                  Species
		label              string
		days, fish, lo, hi int
		skill              int
	}{{Tilapia, "tilapia", 6, 48, 15, 35, 0}, {Carp, "carp", 8, 72, 2, 30, 4}, {Trout, "trout", 10, 110, 0, 20, 8}} {
		d := DefOf(c.s)
		if d.Label != c.label || d.GrowTicks != c.days*clock.TicksPerDay || d.Yield != c.fish || d.MinC != c.lo || d.MaxC != c.hi || d.MinSkill != c.skill {
			t.Errorf("%s: %+v", c.label, d)
		}
		if c.s.String() != c.label {
			t.Errorf("%s: String %q", c.label, c.s.String())
		}
	}
	if DefOf(NumSpecies).Label != "" || None.Valid() || NumSpecies.Valid() || !Trout.Valid() {
		t.Fatal("an unknown stock must read empty")
	}
}

// TestGrowsToRipeThenWaits: growth takes the species' days under good
// conditions, then the ripe stock waits without passing its limit.
func TestGrowsToRipeThenWaits(t *testing.T) {
	t.Parallel()
	tk := stocked(Tilapia)
	tk, _ = run(tk, 6*clock.HoursPerDay-1, good)
	if tk.Ripe() || tk.GrowthPct() >= 100 {
		t.Fatalf("ripe an hour early: %+v", tk)
	}
	tk, _ = run(tk, 1, good)
	if !tk.Ripe() || tk.GrowthPct() != 100 {
		t.Fatalf("not ripe on time: %+v", tk)
	}
	tk, _ = run(tk, 50, good)
	if tk.Growth != DefOf(Tilapia).GrowTicks {
		t.Fatalf("growth passed the limit: %d", tk.Growth)
	}
	if n := tk.Netted(); n.Growth != 0 || !n.Stocked || n.Health != tk.Health || n.Ripe() {
		t.Fatalf("netting restocks by itself: %+v", n)
	}
}

// TestEmptyTankDoesNothing: an unstocked tank neither grows nor suffers.
func TestEmptyTankDoesNothing(t *testing.T) {
	t.Parallel()
	in := Tank{Species: Carp}
	got, out := in.Step(hour, Conditions{})
	if got != in || out != (Outcome{}) {
		t.Fatalf("%+v %+v", got, out)
	}
	if in.Ripe() || in.HealthPct() != 0 {
		t.Fatalf("empty tank reads ripe or alive: %+v", in)
	}
}

// TestTroubleOrder: power first, then the air, then feed, with the player's
// words.
func TestTroubleOrder(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		c    Conditions
		want Cause
		text string
	}{
		{good, Fine, ""},
		{Conditions{Air: temperature.C(-30)}, NoPower, "No power"},
		{Conditions{Power: true, Air: temperature.C(-1), Fed: true}, TooCold, "Too cold"},
		{Conditions{Power: true, Air: temperature.C(21), Fed: true}, TooHot, "Too hot"},
		{Conditions{Power: true, Air: temperature.C(-1)}, TooCold, "Too cold"},
		{Conditions{Power: true, Air: temperature.C(10)}, NoFeed, "No feed"},
		{Conditions{Power: true, Air: temperature.C(0), Fed: true}, Fine, ""},
		{Conditions{Power: true, Air: temperature.C(20), Fed: true}, Fine, ""},
	} {
		got := Trout.Trouble(c.c)
		if got != c.want || got.String() != c.text {
			t.Errorf("%+v: %v %q", c.c, got, got.String())
		}
	}
	if Trout.Trouble(Conditions{Power: true, Air: temperature.C(10), Fed: true}) != Fine ||
		Tilapia.Trouble(Conditions{Power: true, Air: temperature.C(10), Fed: true}) != TooCold {
		t.Fatal("each species has its own band")
	}
}

// TestDeathTimes: no power is the fastest, about a day from full to dead;
// the stock dies with its cause, and the chosen species stays chosen.
func TestDeathTimes(t *testing.T) {
	t.Parallel()
	hoursToDie := func(c Conditions) (int, Cause) {
		tk := stocked(Trout)
		for h := 1; h < 1000; h++ {
			var o Outcome
			if tk, o = tk.Step(hour, c); o.Died {
				if tk != (Tank{Species: Trout}) {
					t.Fatalf("a dead stock leaves an empty tank: %+v", tk)
				}
				return h, o.Cause
			}
		}
		t.Fatalf("%+v never killed", c)
		return 0, Fine
	}
	power, cause := hoursToDie(Conditions{Air: temperature.C(10), Fed: true})
	if power != clock.HoursPerDay || cause != NoPower {
		t.Fatalf("no power: %d hours, %v", power, cause)
	}
	cold, cause := hoursToDie(Conditions{Power: true, Air: temperature.C(-10), Fed: true})
	if cold <= power || cause != TooCold {
		t.Fatalf("cold: %d hours, %v", cold, cause)
	}
	hungry, cause := hoursToDie(Conditions{Power: true, Air: temperature.C(10)})
	if hungry <= cold || cause != NoFeed {
		t.Fatalf("no feed: %d hours, %v", hungry, cause)
	}
}

// TestGrowthPausesWhileFalling: a suffering stock doesn't grow, and a good
// spell restores it more slowly than a bad one costs.
func TestGrowthPausesWhileFalling(t *testing.T) {
	t.Parallel()
	tk := stocked(Trout)
	tk, _ = run(tk, 4, good)
	g0 := tk.Growth
	tk, _ = run(tk, 6, Conditions{Air: temperature.C(10), Fed: true})
	if tk.Growth != g0 || tk.Health != HealthFull-6*hour {
		t.Fatalf("while falling: %+v", tk)
	}
	h0 := tk.Health
	tk, _ = run(tk, 6, good)
	if tk.Growth != g0+6*hour || tk.Health <= h0 || tk.Health >= HealthFull {
		t.Fatalf("recovering: %+v", tk)
	}
}

// TestWarningHysteresis: the dying warning fires once on the way down, and a
// stock hovering around the threshold doesn't fire it again until it has
// really recovered.
func TestWarningHysteresis(t *testing.T) {
	t.Parallel()
	dark := Conditions{Air: temperature.C(10), Fed: true}
	tk := stocked(Trout)
	warns := 0
	step := func(c Conditions) {
		var o Outcome
		tk, o = tk.Step(hour, c)
		if o.Warn {
			warns++
			if o.Cause != NoPower {
				t.Fatalf("warned of %v", o.Cause)
			}
		}
		if o.Died {
			t.Fatal("died in a test of warnings")
		}
	}
	for tk.Health >= WarnBelow {
		step(dark)
	}
	if warns != 1 || !tk.Warned {
		t.Fatalf("crossing down warned %d times", warns)
	}
	// blinking power around the line: never warns again
	for i := 0; i < 200; i++ {
		c := good
		if i%2 == 0 {
			c = dark
		}
		if tk.Health < WarnBelow/2 {
			c = good
		}
		step(c)
		if tk.Health >= WarnClear {
			break
		}
	}
	if warns != 1 {
		t.Fatalf("hovering warned %d times", warns)
	}
	// recovered past the clear line, the next fall warns again
	for tk.Health < WarnClear {
		step(good)
	}
	if tk.Warned {
		t.Fatalf("still warned at %d", tk.Health)
	}
	for tk.Health >= WarnBelow {
		step(dark)
	}
	if warns != 2 {
		t.Fatalf("second fall warned %d times", warns)
	}
	if WarnClear <= WarnBelow {
		t.Fatal("the thresholds must differ")
	}
}

// TestStepIsPerElapsed: one long step equals the same time in short ones
// (no per-call rounding drift for the rates the game uses).
func TestStepIsPerElapsed(t *testing.T) {
	t.Parallel()
	a := stocked(Carp)
	b := a
	a, _ = a.Step(10*hour, Conditions{Air: temperature.C(10), Fed: true})
	for i := 0; i < 10; i++ {
		b, _ = b.Step(hour, Conditions{Air: temperature.C(10), Fed: true})
	}
	if a != b {
		t.Fatalf("%+v != %+v", a, b)
	}
}

// TestNetYield: skill scales the catch, with the unskilled fumbling part of
// it and a master losing none.
func TestNetYield(t *testing.T) {
	t.Parallel()
	if got := NetYield(Tilapia, 0); got <= 0 || got >= DefOf(Tilapia).Yield {
		t.Fatalf("unskilled: %d", got)
	}
	if got := NetYield(Trout, 20); got != DefOf(Trout).Yield {
		t.Fatalf("master: %d", got)
	}
	last := 0
	for lvl := 0; lvl <= 20; lvl++ {
		got := NetYield(Carp, lvl)
		if got < last {
			t.Fatalf("level %d catches less: %d < %d", lvl, got, last)
		}
		last = got
	}
	if NetYield(None, 10) != 0 {
		t.Fatal("no stock, no fish")
	}
}

// TestPercents: the read model's numbers.
func TestPercents(t *testing.T) {
	t.Parallel()
	tk := stocked(Tilapia)
	if tk.HealthPct() != 100 || tk.GrowthPct() != 0 {
		t.Fatalf("%+v", tk)
	}
	tk.Health, tk.Growth = HealthFull/2, DefOf(Tilapia).GrowTicks/4
	if tk.HealthPct() != 50 || tk.GrowthPct() != 25 {
		t.Fatalf("%d %d", tk.HealthPct(), tk.GrowthPct())
	}
	tk.Health = 1
	if tk.HealthPct() != 1 {
		t.Fatalf("a living stock reads above 0: %d", tk.HealthPct())
	}
}

// TestFeedTable: refuel asks at a quarter and fills to full, far apart.
func TestFeedTable(t *testing.T) {
	t.Parallel()
	if FeedFull != FeedItems*FeedItemTicks || FeedBelow != FeedFull/4 || FeedBelow <= 0 {
		t.Fatalf("%d %d %d", FeedFull, FeedBelow, FeedItemTicks)
	}
}
