package storyteller

import (
	"strings"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/pkg/randx"
)

const tpd = clock.TicksPerDay

func mustProfile(t *testing.T, key string) Profile {
	t.Helper()
	p, ok := ByKey(key)
	if !ok {
		t.Fatalf("no storyteller %q", key)
	}
	return p
}

func TestProfilesTable(t *testing.T) {
	t.Parallel()
	vis := Profiles()
	if len(vis) != 3 {
		t.Fatalf("%d visible storytellers, want 3", len(vis))
	}
	want := []struct {
		key, name, title        string
		mode                    Mode
		first, gmin, gmax, bias int
	}{
		{"steady", "Orla Vane", "the Steady Hand", Cycle, 6, 3, 5, 20},
		{"hearth", "Bram Holloway", "the Hearthkeeper", Calm, 10, 9, 15, 45},
		{"gambler", "Kestrel Quay", "the Gambler", Dice, 4, 1, 0, 30},
	}
	for i, w := range want {
		p := vis[i]
		if p.Key != w.key || p.Name != w.name || p.Title != w.title || p.Mode != w.mode ||
			p.FirstThreatDay != w.first || p.GapMin != w.gmin || p.GapMax != w.gmax || p.PositiveBias != w.bias {
			t.Errorf("%s: %+v", w.key, p)
		}
		paras := strings.Split(p.Bio, "\n\n")
		if len(paras) < 2 || len(paras) > 3 {
			t.Errorf("%s bio has %d paragraphs, want 2-3", p.Key, len(paras))
		}
		if p.Portrait != "ui/portrait/"+p.Key {
			t.Errorf("%s portrait %q", p.Key, p.Portrait)
		}
		if p.Hidden {
			t.Errorf("%s should be visible", p.Key)
		}
	}
	if g := mustProfile(t, "steady"); g.CycleOn != 8 || g.CycleOff != 5 || g.RampPerCycle != 4 || g.RampCap != 40 {
		t.Errorf("steady cycle numbers: %+v", g)
	}
	if h := mustProfile(t, "hearth"); h.LossEaseDays != 6 || h.LossEasePct != 70 {
		t.Errorf("hearth loss ease: %+v", h)
	}
	if g := mustProfile(t, "gambler"); g.DiceP0 != 125 || g.DiceStep != 25 || g.PointsMin != 60 || g.PointsMax != 150 {
		t.Errorf("gambler dice: %+v", g)
	}
	l := mustProfile(t, "legacy")
	if !l.Hidden || l.Mode != Legacy || l.FirstThreatDay != 5 || l.GapMin != 5 || l.GapMax != 9 {
		t.Errorf("legacy: %+v", l)
	}
	for _, p := range vis {
		if p.Key == "legacy" {
			t.Error("legacy is hidden from Profiles")
		}
	}
	if _, ok := ByKey("nobody"); ok {
		t.Error("unknown key")
	}
	for _, m := range []Mode{Legacy, Cycle, Calm, Dice} {
		if m.String() == "" {
			t.Error("mode string")
		}
	}
}

// TestLegacyPlanMatchesConstants reimplements today's scheduleThreats and
// fireThreat timing (raids.go: firstRaidDay 5, raidGapMin 5, raidGapSpread
// 5) and checks the legacy profile reproduces it for 1,000 days, with the
// other random draws a fired threat makes in between.
func TestLegacyPlanMatchesConstants(t *testing.T) {
	t.Parallel()
	const (
		firstRaidDay  = 5
		raidGapMin    = 5
		raidGapSpread = 5
	)
	legacy := mustProfile(t, "legacy")
	for _, seed := range []uint64{1, 7, 42, 99, 12345} {
		ref, got := randx.New(seed), randx.New(seed)
		// scheduleThreats
		next := uint64(firstRaidDay*clock.TicksPerDay + ref.IntN(clock.TicksPerDay))
		var st State
		d := Plan(legacy, &st, 0, got)
		if d.AtTick != next || d.Category != Raid || d.PointsPct != 100 || d.Second != None {
			t.Fatalf("seed %d start: got %+v, want tick %d", seed, d, next)
		}
		n := 0
		for next/clock.TicksPerDay < 1000 {
			now := next
			// fireThreat's own draws between the firing and the next plan.
			extra := 1 + n%5
			for i := 0; i < extra; i++ {
				if a, b := ref.IntN(8), got.IntN(8); a != b {
					t.Fatalf("seed %d: streams diverged", seed)
				}
			}
			next = now + uint64((raidGapMin+ref.IntN(raidGapSpread))*clock.TicksPerDay+ref.IntN(clock.TicksPerDay))
			d = Plan(legacy, &st, now, got)
			if d.AtTick != next {
				t.Fatalf("seed %d threat %d: got tick %d, want %d", seed, n, d.AtTick, next)
			}
			if d.PointsPct != 100 {
				t.Fatalf("legacy points are unscaled: %d", d.PointsPct)
			}
			n++
		}
		if n < 115 { // about 1000 days over a 7.5-day mean gap
			t.Fatalf("seed %d: only %d threats in 1000 days", seed, n)
		}
		// Both streams are in the same state at the end.
		if ref.IntN(1<<30) != got.IntN(1<<30) {
			t.Fatalf("seed %d: Plan drew a different number of values", seed)
		}
	}
}

func chain(p Profile, seed uint64, days int) []Decision {
	rng := randx.New(seed)
	var st State
	var out []Decision
	d := Plan(p, &st, 0, rng)
	for int(d.AtTick/tpd) < days {
		out = append(out, d)
		d = Plan(p, &st, d.AtTick, rng)
	}
	return out
}

func TestCycleRestHasNoBigThreats(t *testing.T) {
	t.Parallel()
	p := mustProfile(t, "steady")
	length := p.CycleOn + p.CycleOff
	for seed := uint64(1); seed <= 30; seed++ {
		ds := chain(p, seed, 800)
		if int(ds[0].AtTick/tpd) != p.FirstThreatDay {
			t.Fatalf("seed %d: first threat on day %d, want %d", seed, ds[0].AtTick/tpd, p.FirstThreatDay)
		}
		perCycle := map[int]int{}
		for i, d := range ds {
			day := int(d.AtTick / tpd)
			if day%length >= p.CycleOn {
				t.Fatalf("seed %d: threat %d on day %d falls in the rest phase", seed, i, day)
			}
			if d.Category != Raid {
				t.Fatalf("category %v", d.Category)
			}
			perCycle[day/length]++
			wantPct := 100 + min(40, 4*(day/length))
			if d.PointsPct != wantPct {
				t.Fatalf("seed %d day %d: points %d%%, want %d%%", seed, day, d.PointsPct, wantPct)
			}
			if i > 0 {
				gap := d.AtTick - ds[i-1].AtTick
				if gap < uint64(p.GapMin*tpd) {
					t.Fatalf("seed %d: gap %d ticks is under the minimum", seed, gap)
				}
			}
		}
		for c, n := range perCycle {
			if n > 3 {
				t.Errorf("seed %d cycle %d: %d threats in one pressure phase", seed, c, n)
			}
		}
		if len(perCycle) < 55 { // 800 days = 61 cycles; nearly every one has a threat
			t.Errorf("seed %d: threats in only %d cycles", seed, len(perCycle))
		}
	}
	// Phase and ramp helpers.
	if on, c := Phase(p, 7); !on || c != 0 {
		t.Error("day 7 is pressure")
	}
	if on, _ := Phase(p, 8); on {
		t.Error("day 8 is rest")
	}
	if on, c := Phase(p, 13); !on || c != 1 {
		t.Error("day 13 starts cycle 1")
	}
	if Ramp(p, 13*20) != 40 || Ramp(p, 0) != 0 || Ramp(p, 13*3) != 12 {
		t.Error("ramp is 4% per cycle, capped at 40%")
	}
	if on, c := Phase(mustProfile(t, "hearth"), 1234); !on || c != 0 {
		t.Error("only Cycle storytellers have a rest phase")
	}
	if Ramp(mustProfile(t, "hearth"), 999) != 0 {
		t.Error("only Cycle storytellers ramp")
	}
}

func TestCalmGapsAndLossEase(t *testing.T) {
	t.Parallel()
	p := mustProfile(t, "hearth")
	for seed := uint64(1); seed <= 20; seed++ {
		ds := chain(p, seed, 1500)
		if d := int(ds[0].AtTick / tpd); d != 10 {
			t.Fatalf("first threat day %d, want 10", d)
		}
		for i := 1; i < len(ds); i++ {
			gap := ds[i].AtTick - ds[i-1].AtTick
			if gap < 9*tpd || gap >= 16*tpd {
				t.Fatalf("seed %d: gap %.2f days outside 9..15", seed, float64(gap)/tpd)
			}
			if ds[i].PointsPct != 100 {
				t.Fatalf("calm threats run at full points: %d", ds[i].PointsPct)
			}
		}
	}
	// Loss ease: +6 days, 70% points, once per pending threat.
	rng := randx.New(5)
	var st State
	d := Plan(p, &st, 0, rng)
	before := d
	if !OnLoss(p, &st, &d, 3) {
		t.Fatal("a death eases the pending threat")
	}
	if d.AtTick != before.AtTick+6*tpd || d.PointsPct != 70 {
		t.Errorf("eased: %+v from %+v", d, before)
	}
	if st.LastLossDay != 3 {
		t.Errorf("loss day recorded: %d", st.LastLossDay)
	}
	again := d
	if OnLoss(p, &st, &d, 4) || d != again {
		t.Error("a second death does not stack")
	}
	if st.LastLossDay != 4 {
		t.Error("but the loss day updates")
	}
	d = Plan(p, &st, d.AtTick, rng)
	if st.Eased {
		t.Error("planning the next threat clears the easing")
	}
	if !OnLoss(p, &st, &d, 40) {
		t.Error("the next threat can be eased again")
	}
	// Profiles without loss ease and bad calls do nothing.
	steady := mustProfile(t, "steady")
	var st2 State
	d2 := Plan(steady, &st2, 0, randx.New(1))
	if OnLoss(steady, &st2, &d2, 1) {
		t.Error("steady has no loss ease")
	}
	if OnLoss(p, &State{}, nil, 1) {
		t.Error("nil decision")
	}
	none := Decision{}
	if OnLoss(p, &State{}, &none, 1) {
		t.Error("no pending threat to ease")
	}
	odd := p
	odd.LossEasePct = 0
	d3 := Decision{AtTick: 5, Category: Raid, PointsPct: 90}
	if !OnLoss(odd, &State{}, &d3, 1) || d3.PointsPct != 90 {
		t.Error("a zero percent means unchanged points")
	}
}

func TestDiceProbabilityRisesWhenQuiet(t *testing.T) {
	t.Parallel()
	p := mustProfile(t, "gambler")
	const trials = 40000
	counts := make([]int, 60)
	doubles, lo, hi := 0, 1000, 0
	for i := 0; i < trials; i++ {
		rng := randx.New(uint64(i) + 1)
		var st State
		// Start plan, then a plan after a threat at day 20 00:00.
		first := Plan(p, &st, 0, rng)
		if d := int(first.AtTick / tpd); d < p.FirstThreatDay {
			t.Fatalf("first threat before day %d: %d", p.FirstThreatDay, d)
		}
		now := uint64(20 * tpd)
		d := Plan(p, &st, now, rng)
		counts[st.QuietDays]++
		fireDay := int(d.AtTick / tpd)
		if fireDay != 21+st.QuietDays {
			t.Fatalf("fire day %d with %d quiet days", fireDay, st.QuietDays)
		}
		if d.AtTick%tpd != 7*clock.TicksPerHour {
			t.Fatalf("the roll happens at 07:00, got offset %d", d.AtTick%tpd)
		}
		if d.PointsPct < 60 || d.PointsPct > 150 {
			t.Fatalf("points %d%% outside 60..150", d.PointsPct)
		}
		lo, hi = min(lo, d.PointsPct), max(hi, d.PointsPct)
		if d.Second != None {
			doubles++
		}
	}
	if lo > 62 || hi < 148 {
		t.Errorf("points should cover the 60..150 range, saw %d..%d", lo, hi)
	}
	if share := float64(doubles) / trials; share < 0.04 || share > 0.06 {
		t.Errorf("double threats %.3f of the time, want about 5%%", share)
	}
	// Hazard on quiet day k is (P0 + k*step)/1000.
	remaining := trials
	for k := 0; k <= 8; k++ {
		hazard := float64(counts[k]) / float64(remaining)
		want := float64(125+25*k) / 1000
		if hazard < want-0.03 || hazard > want+0.03 {
			t.Errorf("day %d: hazard %.3f, want about %.3f", k, hazard, want)
		}
		remaining -= counts[k]
	}
	// Certain by the 36th quiet day.
	for k := 36; k < len(counts); k++ {
		if counts[k] != 0 {
			t.Errorf("quiet streak of %d days should be impossible", k)
		}
	}
}

func TestPlanIsDeterministic(t *testing.T) {
	t.Parallel()
	for _, p := range append(Profiles(), mustProfile(t, "legacy")) {
		a, b := chain(p, 77, 300), chain(p, 77, 300)
		if len(a) == 0 || len(a) != len(b) {
			t.Fatalf("%s: chains differ in length %d vs %d", p.Key, len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("%s: threat %d differs", p.Key, i)
			}
		}
	}
}

func TestSpark(t *testing.T) {
	t.Parallel()
	if Spark(mustProfile(t, "steady"), 0, 1) != nil {
		t.Error("no days, no spark")
	}
	for _, p := range Profiles() {
		s := Spark(p, 120, 9)
		if len(s) != 120 {
			t.Fatalf("%s: %d days", p.Key, len(s))
		}
		total, peak := 0, 0
		for day, v := range s {
			if v < 0 {
				t.Fatalf("negative strength")
			}
			if v > 0 && day < p.FirstThreatDay {
				t.Errorf("%s: threat on day %d before day %d", p.Key, day, p.FirstThreatDay)
			}
			total += v
			peak = max(peak, v)
		}
		if total == 0 {
			t.Errorf("%s: spark is empty", p.Key)
		}
		again := Spark(p, 120, 9)
		for i := range s {
			if s[i] != again[i] {
				t.Fatalf("%s: spark not deterministic", p.Key)
			}
		}
		if p.Mode == Cycle {
			for day, v := range s {
				if v > 0 && day%(p.CycleOn+p.CycleOff) >= p.CycleOn {
					t.Errorf("rest day %d has a threat", day)
				}
			}
		}
	}
	// Calm storytellers send fewer threats than dice in a long run.
	count := func(key string) int {
		n := 0
		for _, v := range Spark(mustProfile(t, key), 600, 3) {
			if v > 0 {
				n++
			}
		}
		return n
	}
	if count("hearth") >= count("steady") {
		t.Error("the hearthkeeper is gentler than the steady hand")
	}
}

// TestNamesAvoidBlocklist is the originality lint over names, titles and
// bios (NG-1 §0.4).
func TestNamesAvoidBlocklist(t *testing.T) {
	t.Parallel()
	for _, p := range Profiles() {
		for _, text := range []string{p.Name, p.Title, p.Bio} {
			if phrase, bad := faction.Blocked(text); bad {
				t.Errorf("%s: %q contains blocklisted %q", p.Key, text, phrase)
			}
		}
	}
}
