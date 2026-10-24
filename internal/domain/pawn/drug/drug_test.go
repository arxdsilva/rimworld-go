package drug

import (
	"bytes"
	"encoding/gob"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/clock"
)

const hr = uint64(clock.TicksPerHour)

func never(int) int  { return 999 }
func always(int) int { return 0 }

func TestDefs(t *testing.T) {
	for _, k := range []item.Kind{item.Beer, item.AshleafTea, item.SparkTablet} {
		if _, ok := DefOf(k); !ok {
			t.Fatal(k)
		}
	}
	if _, ok := DefOf(item.Wood); ok {
		t.Fatal("wood is not a drug")
	}
	if len(All()) != 3 {
		t.Fatal("All")
	}
	var s State
	s.Ingest(item.Wood, always, 1) // ignored
	if s.Alcohol != 0 {
		t.Fatal("ignored")
	}
}

func TestBands(t *testing.T) {
	cases := []struct {
		a, doses int32
		b        Band
		f        int32
	}{{0, 0, Sober, 1000}, {100, 1, Warm, 1500}, {249, 1, Warm, 1500}, {250, 1, Tipsy, 2500}, {399, 1, Tipsy, 2500},
		{400, 1, Drunk, 4000}, {699, 2, Drunk, 4000}, {700, 1, Hammered, 5000}, {800, 0, Drunk, 4000}}
	for _, c := range cases {
		s := State{Alcohol: c.a, Doses: c.doses}
		if s.Band() != c.b || s.FightMultiplier() != c.f {
			t.Errorf("%d: band %v fight %d", c.a, s.Band(), s.FightMultiplier())
		}
	}
	if Tipsy.Label() != "tipsy" {
		t.Fatal("label")
	}
}

func TestAlcoholDecayAndHangover(t *testing.T) {
	var s State
	for i := 0; i < 3; i++ {
		s.Ingest(item.Beer, never, 0)
	}
	if s.Alcohol != 450 || s.Band() != Drunk {
		t.Fatalf("alcohol %d", s.Alcohol)
	}
	s.Tick(2, 2*hr)
	if s.Alcohol != 300 || s.Band() != Tipsy {
		t.Fatalf("after 2h %d", s.Alcohol)
	}
	s.Tick(4, 6*hr)
	if s.Alcohol != 0 || s.Doses != 0 {
		t.Fatalf("sober %d", s.Alcohol)
	}
	if !s.ConsumeHangover() || s.ConsumeHangover() {
		t.Fatal("hangover once")
	}
	if !s.Hungover(6*hr+1) || s.Hungover(6*hr+uint64(hangoverTicks)) {
		t.Fatal("hangover window")
	}
}

func TestNoHangoverWhenOnlyTipsy(t *testing.T) {
	var s State
	s.Ingest(item.Beer, never, 0)
	s.Tick(5, 5*hr)
	if s.ConsumeHangover() || s.Hungover(5*hr) {
		t.Fatal("light drinking has no hangover")
	}
}

func TestHighsAndThoughts(t *testing.T) {
	var s State
	s.Ingest(item.AshleafTea, never, 0)
	s.Ingest(item.AshleafTea, never, hr) // refresh, not duplicate
	if len(s.Highs) != 1 {
		t.Fatal("one high")
	}
	if th := s.Thoughts(); len(th) != 1 || th[0] != mood.HighOnDrug {
		t.Fatal(th)
	}
	s.Tick(13, 14*hr)
	if len(s.Highs) != 0 || len(s.Thoughts()) != 0 {
		t.Fatal("high expired")
	}
	s.Ingest(item.Beer, never, 0)
	s.Ingest(item.Beer, never, 0)
	if th := s.Thoughts(); len(th) != 1 || th[0] != mood.Drunk {
		t.Fatal(th)
	}
}

func TestAddictionRoll(t *testing.T) {
	var s State
	s.Ingest(item.SparkTablet, never, 5)
	if s.Addicted(item.SparkTablet) {
		t.Fatal("no roll")
	}
	s.Ingest(item.SparkTablet, always, 5)
	if !s.Addicted(item.SparkTablet) {
		t.Fatal("addicted")
	}
	// threshold: tolerance scales chance; 60 base, tolerance 200 -> 72
	s2 := State{}
	s2.Tolerance[2] = 1000
	s2.Ingest(item.SparkTablet, func(int) int { return 100 }, 0)
	if !s2.Addicted(item.SparkTablet) {
		t.Fatal("tolerance boosts chance to 120")
	}
	s3 := State{}
	s3.Ingest(item.SparkTablet, func(int) int { return 100 }, 0)
	if s3.Addicted(item.SparkTablet) {
		t.Fatal("no tolerance: 60")
	}
	// nil roll never addicts
	s4 := State{}
	s4.Ingest(item.AshleafTea, nil, 0)
	if s4.Addicted(item.AshleafTea) {
		t.Fatal("nil roll")
	}
}

func TestToleranceBuildsAndDecays(t *testing.T) {
	var s State
	for i := 0; i < 20; i++ {
		s.Ingest(item.Beer, never, 0)
	}
	if s.Tolerance[0] != 1000 {
		t.Fatal(s.Tolerance[0])
	}
	s.Tick(10, 10*hr)
	if s.Tolerance[0] != 900 {
		t.Fatal(s.Tolerance[0])
	}
	s.Tick(200, 210*hr)
	if s.Tolerance[0] != 0 {
		t.Fatal("floor 0")
	}
}

func TestWithdrawalAndMults(t *testing.T) {
	var s State
	s.Ingest(item.AshleafTea, always, 0)
	s.Ingest(item.SparkTablet, always, 0)
	s.Tick(10, 10*hr)
	if s.FightMultiplier() != 1000 || len(s.Thoughts()) != 1 { // only the high
		t.Fatal("not yet in withdrawal")
	}
	s.Tick(50, 60*hr)
	if th := s.Thoughts(); th[len(th)-1] != mood.Withdrawal {
		t.Fatal(th)
	}
	if s.FightMultiplier() != 3000 {
		t.Fatal(s.FightMultiplier())
	}
	s.Alcohol, s.Doses = 300, 1 // tipsy 2500 x 3000
	if s.FightMultiplier() != 7500 {
		t.Fatal(s.FightMultiplier())
	}
	// only spark: 2000
	var u State
	u.Ingest(item.SparkTablet, always, 0)
	u.Tick(49, 49*hr)
	if u.FightMultiplier() != 2000 {
		t.Fatal(u.FightMultiplier())
	}
}

func TestCravingHysteresis(t *testing.T) {
	s := State{Addictions: []Addiction{{Drug: item.AshleafTea, Need: 1000}}}
	if _, ok := s.Craving(); ok {
		t.Fatal("fresh")
	}
	flips, last := 0, false
	// Drive need directly through Tick-equivalent: oscillate around 500.
	for i := 0; i < 20; i++ {
		s.Addictions[0].Need = 480 + int32(i%2)*40 // 480/520
		s.Tick(1, 1)                               // decays a little more each call
		s.Addictions[0].Need += needDecayHour
		_, on := s.Craving()
		if on != last {
			flips++
			last = on
		}
	}
	if flips != 0 {
		t.Fatalf("flapped %d times around 500", flips)
	}
	s.Addictions[0].Need = 310
	s.Tick(1, 1)
	if _, on := s.Craving(); !on {
		t.Fatal("below 300 starts seeking")
	}
	s.Addictions[0].Need = 600 // between thresholds: stays on
	s.Tick(0, 1)
	s.Addictions[0].Need = 600 + needDecayHour
	s.Tick(1, 1)
	if _, on := s.Craving(); !on {
		t.Fatal("stays seeking at 600")
	}
	s.Addictions[0].Need = 800
	s.Tick(1, 1)
	if _, on := s.Craving(); on {
		t.Fatal("stops above 700")
	}
}

func TestCravingNaturalCycle(t *testing.T) {
	var s State
	s.Ingest(item.AshleafTea, always, 0)
	var k item.Kind
	started := -1
	for h := 1; h <= 48; h++ {
		s.Tick(1, uint64(h)*hr)
		if c, ok := s.Craving(); ok && started < 0 {
			started, k = h, c
		}
	}
	if started < 30 || started > 40 || k != item.AshleafTea {
		t.Fatalf("craving started at hour %d", started)
	}
	s.Ingest(item.AshleafTea, always, 48*hr)
	s.Tick(1, 49*hr)
	if _, ok := s.Craving(); ok || s.Addictions[0].Withdrawal {
		t.Fatal("dose clears craving and withdrawal")
	}
}

func TestRecovery(t *testing.T) {
	var s State
	s.Ingest(item.SparkTablet, always, 0)
	s.Tick(1, 9*uint64(clock.TicksPerDay))
	if !s.Addicted(item.SparkTablet) {
		t.Fatal("9 days: still addicted")
	}
	s.Tick(1, 10*uint64(clock.TicksPerDay))
	if s.Addicted(item.SparkTablet) {
		t.Fatal("10 days clean: recovered")
	}
	if s.FightMultiplier() != 1000 || len(s.Thoughts()) != 0 {
		t.Fatal("no residual withdrawal")
	}
}

func TestLines(t *testing.T) {
	var s State
	if len(s.Lines()) != 0 {
		t.Fatal("empty")
	}
	s.Ingest(item.Beer, never, 0)
	s.Ingest(item.AshleafTea, always, 0)
	s.Addictions = append(s.Addictions,
		Addiction{Drug: item.SparkTablet, Need: 200, Seeking: true},
		Addiction{Drug: item.Beer, Need: 0, Withdrawal: true})
	got := s.Lines()
	want := []string{"warm", "high on ashleaf", "hooked on ashleaf", "craving spark", "withdrawal from beer"}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i, l := range got {
		if l.Label != want[i] {
			t.Errorf("%d: %q want %q", i, l.Label, want[i])
		}
	}
	if got[0].Percent != 15 || got[3].Percent != 20 {
		t.Fatal(got)
	}
}

func TestPolicyMatrix(t *testing.T) {
	var fresh State
	now := 100 * hr
	type tc struct {
		name   string
		p      Preset
		k      item.Kind
		joyLow bool
		tr     []trait.ID
		want   bool
	}
	cases := []tc{
		{"nodrugs", NoDrugs, item.Beer, true, nil, false},
		{"social beer low joy", SocialDrinking, item.Beer, true, nil, true},
		{"social beer ok joy", SocialDrinking, item.Beer, false, nil, false},
		{"social tea", SocialDrinking, item.AshleafTea, true, nil, false},
		{"unrestricted tea", Unrestricted, item.AshleafTea, true, nil, true},
		{"unrestricted spark ok joy", Unrestricted, item.SparkTablet, false, nil, false},
		{"abstainer", Unrestricted, item.Beer, true, []trait.ID{trait.Abstainer}, false},
		{"hedonist ok joy", SocialDrinking, item.Beer, false, []trait.ID{trait.Hedonist}, true},
		{"hedonist no tea on social", SocialDrinking, item.AshleafTea, false, []trait.ID{trait.Hedonist}, false},
		{"hedonist nodrugs", NoDrugs, item.Beer, false, []trait.ID{trait.Hedonist}, false},
		{"non drug", Unrestricted, item.Wood, true, nil, false},
	}
	for _, c := range cases {
		if got := c.p.Allows(c.k, c.joyLow, &fresh, c.tr, now); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
	if SocialDrinking.Allows(item.Beer, true, nil, nil, now) {
		t.Fatal("nil state")
	}
}

func TestPolicyIntervals(t *testing.T) {
	now := 100 * hr
	mk := func() *State { s := &State{}; s.LastDose[0] = now - 5*hr; return s }
	if SocialDrinking.Allows(item.Beer, true, mk(), nil, now) {
		t.Fatal("5h < 12h")
	}
	if SocialDrinking.Allows(item.Beer, true, mk(), []trait.ID{trait.Indulgent}, now) {
		t.Fatal("indulgent 6h: 5h too soon")
	}
	s := mk()
	s.LastDose[0] = now - 6*hr
	if !SocialDrinking.Allows(item.Beer, true, s, []trait.ID{trait.Indulgent}, now) {
		t.Fatal("indulgent 6h ok")
	}
	if !SocialDrinking.Allows(item.Beer, false, mk(), []trait.ID{trait.Hedonist}, now) {
		t.Fatal("hedonist 3h ok after 5h")
	}
	s.LastDose[0] = now - 2*hr
	if SocialDrinking.Allows(item.Beer, false, s, []trait.ID{trait.Hedonist}, now) {
		t.Fatal("hedonist 2h too soon")
	}
	s.LastDose[0] = now - 13*hr
	if !SocialDrinking.Allows(item.Beer, true, s, nil, now) {
		t.Fatal("12h ok")
	}
}

func TestPolicyNoBingeAndCraving(t *testing.T) {
	now := 100 * hr
	s := &State{Alcohol: 500, Doses: 3}
	if Unrestricted.Allows(item.Beer, true, s, nil, now) {
		t.Fatal("too drunk")
	}
	s = &State{Addictions: []Addiction{{Drug: item.AshleafTea, Need: 100, Seeking: true}}}
	s.LastDose[1] = now - 1
	ab := []trait.ID{trait.Abstainer}
	if !SocialDrinking.Allows(item.AshleafTea, false, s, ab, now) {
		t.Fatal("craving overrides abstainer, interval and joy")
	}
	if NoDrugs.Allows(item.AshleafTea, true, s, nil, now) {
		t.Fatal("nodrugs holds")
	}
	if Unrestricted.Allows(item.SparkTablet, true, s, ab, now) {
		t.Fatal("craving is drug-specific")
	}
	for p, l := range map[Preset]string{NoDrugs: "no drugs", SocialDrinking: "social drinking", Unrestricted: "unrestricted"} {
		if p.Label() != l {
			t.Fatal(l)
		}
	}
}

func TestGob(t *testing.T) {
	var s State
	s.Ingest(item.AshleafTea, always, 3)
	s.Ingest(item.Beer, never, 3)
	s.Tick(3, 3+3*hr)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&s); err != nil {
		t.Fatal(err)
	}
	var out State
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Alcohol != s.Alcohol || len(out.Addictions) != 1 || out.Tolerance != s.Tolerance {
		t.Fatal("round trip")
	}
}

// The policy is saved with the state and survives doses and hours.
func TestPolicyKeptAndSaved(t *testing.T) {
	s := State{Policy: Unrestricted, Retry: 77}
	s.Ingest(item.Beer, always, 3)
	s.Tick(30, 3+30*hr)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&s); err != nil {
		t.Fatal(err)
	}
	var out State
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Policy != Unrestricted || out.Retry != 77 {
		t.Fatalf("policy %v retry %d", out.Policy, out.Retry)
	}
}
