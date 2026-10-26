package body

import (
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
)

func TestNoChronicBeforeOnsetAge(t *testing.T) {
	onset := map[CondKind]int{CondBadBack: 41, CondCataract: 49, CondFrail: 51, CondDementia: 69,
		CondHearingLoss: 49, CondArteryBlockage: 21, CondCarcinoma: 23} // wiki: Ailments
	if len(Chronics()) != len(onset) {
		t.Fatalf("chronic kinds %d, want %d", len(Chronics()), len(onset))
	}
	for _, c := range Chronics() {
		if c.OnsetAge != onset[c.Kind] {
			t.Fatalf("%s onset %d, want %d", DefOfCond(c.Kind).Label, c.OnsetAge, onset[c.Kind])
		}
		for age := 0; age < c.OnsetAge; age++ {
			if OnsetPerYear(c.Kind, age) != 0 || OnsetPerDay(c.Kind, age) != 0 {
				t.Fatalf("%s at %d before onset %d", DefOfCond(c.Kind).Label, age, c.OnsetAge)
			}
		}
		if OnsetPerDay(c.Kind, c.OnsetAge) <= 0 {
			t.Fatalf("%s can start at its onset age", DefOfCond(c.Kind).Label)
		}
	}
	if OnsetPerYear(CondHeartAttack, 90) != 0 {
		t.Fatal("a heart attack comes from an artery blockage, not from age")
	}
}

func TestOnsetChanceRisesWithAge(t *testing.T) {
	for _, c := range Chronics() {
		prev := int32(0)
		for age := c.OnsetAge; age <= 120; age++ {
			n := OnsetPerYear(c.Kind, age)
			if n < prev || n > c.Cap {
				t.Fatalf("%s at %d: %d after %d (cap %d)", DefOfCond(c.Kind).Label, age, n, prev, c.Cap)
			}
			prev = n
		}
		if prev != c.Cap || OnsetPerYear(c.Kind, c.OnsetAge+1) <= OnsetPerYear(c.Kind, c.OnsetAge) {
			t.Fatalf("%s rises linearly to its cap: %d", DefOfCond(c.Kind).Label, prev)
		}
	}
	// a 20 ‰ a year chance is 20 000 / 60 a million a day
	if got := OnsetPerDay(CondBadBack, 41); got != 20*1000/clock.DaysPerYear {
		t.Fatalf("bad back at 41: %d a million a day", got)
	}
}

func TestAgingDefsFollowWiki(t *testing.T) {
	var b Body
	b.Develop(CondBadBack, Spine, 0)
	if b.Capacity(Moving) != 700 || b.Capacity(Manipulation) != 900 {
		t.Fatalf("bad back: moving %d, manipulation %d", b.Capacity(Moving), b.Capacity(Manipulation))
	}
	b = Body{}
	b.Develop(CondCataract, EyeL, 0)
	if b.Capacity(Sight) != 750 {
		t.Fatalf("one cataract: sight %d", b.Capacity(Sight))
	}
	b.Develop(CondCataract, EyeR, 0)
	if b.Capacity(Sight) != 500 {
		t.Fatalf("both eyes: sight %d (wiki: 50 %%)", b.Capacity(Sight))
	}
	b = Body{}
	b.Develop(CondDementia, Brain, 0)
	if b.Capacity(Consciousness) != 850 || b.Capacity(Talking) != 600 || b.Capacity(Hearing) != 750 {
		t.Fatalf("dementia: consciousness %d, talking %d, hearing %d",
			b.Capacity(Consciousness), b.Capacity(Talking), b.Capacity(Hearing))
	}
	b = Body{}
	b.Develop(CondHeartAttack, Heart, 0)
	c, _ := b.Cond(CondHeartAttack)
	if c.Sev != 400 || b.Pain() != 400 || c.Stage != 0 {
		t.Fatalf("heart attack starts at 400, painful: %+v pain %d", *c, b.Pain())
	}
	c.Sev, c.Stage = 600, 1
	if b.Capacity(Consciousness) > 100 {
		t.Fatalf("debilitating: consciousness %d", b.Capacity(Consciousness))
	}
}

func TestArteryBlockageProgresses(t *testing.T) {
	var b Body
	b.Develop(CondArteryBlockage, Heart, 0)
	salt := uint64(7)
	c, _ := b.Cond(CondArteryBlockage)
	rate := AgingRate(*c, salt)
	if rate < 350 || rate > 2100 {
		t.Fatalf("rate %d millionths a day, want 700 × 0.5..3", rate)
	}
	day := uint64(clock.TicksPerDay)
	prevSev, prevStage := c.Sev, c.Stage
	var mtb []int32
	for d := uint64(1); ; d++ {
		st := b.StepAging(d*day, salt, nil)
		c, ok := b.Cond(CondArteryBlockage)
		if st.Died == CondArteryBlockage {
			if c.Sev != AgingDeath {
				t.Fatalf("died at %d", c.Sev)
			}
			break
		}
		if !ok || c.Sev < prevSev || c.Stage < prevStage {
			t.Fatalf("day %d: severity only grows: %d → %+v", d, prevSev, c)
		}
		if c.Stage != prevStage || d == 1 {
			mtb = append(mtb, b.HeartAttackMTB())
		}
		prevSev, prevStage = c.Sev, c.Stage
		if d > 20*clock.DaysPerYear*60 {
			t.Fatal("never ends")
		}
	}
	want := []int32{300, 200, 100, 60, 30} // wiki: Ailments
	if len(mtb) != len(want) {
		t.Fatalf("stages passed %v", mtb)
	}
	for i := range want {
		if mtb[i] != want[i] {
			t.Fatalf("heart attack MTB by stage %v, want %v", mtb, want)
		}
	}
	// the pumping falls with the stages
	b = Body{}
	b.AddCond(Condition{Kind: CondArteryBlockage, Part: Heart, Sev: 950, Stage: 4})
	if b.Capacity(BloodPumping) != 400 {
		t.Fatalf("extreme blockage: pumping %d", b.Capacity(BloodPumping))
	}
}

func TestCarcinomaGrowthModes(t *testing.T) {
	seen := map[Growth]int{}
	for s := uint64(0); s < 400; s++ {
		c := Condition{Kind: CondCarcinoma, Part: Stomach, Sev: CarcinomaOnset, Since: s * 31}
		g, r := CarcinomaGrowth(c, s), AgingRate(c, s)
		seen[g]++
		switch g {
		case Growing:
			if r < 1350 || r > 4950 {
				t.Fatalf("growing rate %d", r)
			}
		case Stable:
			if r != 0 {
				t.Fatalf("stable rate %d", r)
			}
		case Remission:
			if r > -1400 || r < -3000 {
				t.Fatalf("remission rate %d", r)
			}
		}
		if AgingRate(c, s) != r || CarcinomaGrowth(c, s) != g {
			t.Fatal("the rate is a pure function of the condition and salt")
		}
	}
	if seen[Growing] < 150 || seen[Stable] < 60 || seen[Remission] < 60 {
		t.Fatalf("modes %v", seen)
	}
}

func TestCarcinomaDestroysPartOrKills(t *testing.T) {
	day := uint64(clock.TicksPerDay)
	growing := func(p Part) (Body, uint64) {
		for s := uint64(0); ; s++ {
			c := Condition{Kind: CondCarcinoma, Part: p}
			if CarcinomaGrowth(c, s) == Growing {
				var b Body
				b.Develop(CondCarcinoma, p, 0)
				return b, s
			}
		}
	}
	b, salt := growing(ArmL)
	st := b.StepAging(1000*day, salt, nil)
	if len(st.Lost) != 1 || st.Lost[0] != ArmL || !b.isMissing(HandL) || b.condOn(CondCarcinoma, ArmL) || st.Died != CondNone {
		t.Fatalf("a full carcinoma destroys its limb: %+v", st)
	}
	b, salt = growing(Brain)
	if st := b.StepAging(1000*day, salt, nil); st.Died != CondCarcinoma || st.DiedOn != Brain {
		t.Fatalf("a full carcinoma in the brain kills: %+v", st)
	}
	// in remission it fades
	for s := uint64(0); ; s++ {
		c := Condition{Kind: CondCarcinoma, Part: Liver}
		if CarcinomaGrowth(c, s) != Remission {
			continue
		}
		var b Body
		b.Develop(CondCarcinoma, Liver, 0)
		st := b.StepAging(1000*day, s, nil)
		if len(st.Ended) != 1 || b.Conds != nil {
			t.Fatalf("remission fades: %+v %+v", st, b.Conds)
		}
		break
	}
}

func TestHeartAttackDeterministic(t *testing.T) {
	run := func(seed uint64, tendQ int32) (hours int, dead bool) {
		r := rand.New(rand.NewPCG(seed, 1))
		var b Body
		b.Develop(CondHeartAttack, Heart, 0)
		for h := 1; h < 200; h++ {
			if c, ok := b.Cond(CondHeartAttack); ok && tendQ > 0 {
				c.TendQ, c.TendSum = tendQ, tendQ
			}
			dead, cured, _ := b.StepHeartAttack(r.IntN)
			if dead || cured {
				return h, dead
			}
		}
		t.Fatal("a heart attack ends")
		return 0, false
	}
	deaths, tendedDeaths := 0, 0
	for s := uint64(0); s < 300; s++ {
		h1, d1 := run(s, 0)
		h2, d2 := run(s, 0)
		if h1 != h2 || d1 != d2 {
			t.Fatalf("seed %d: %d/%v vs %d/%v", s, h1, d1, h2, d2)
		}
		if d1 {
			deaths++
		}
		if _, d := run(s, 1000); d {
			tendedDeaths++
		}
	}
	// wiki: Ailments (untreated, about 76 % die); tending every hour at
	// full quality saves most
	if deaths < 180 || deaths > 270 || tendedDeaths >= deaths/2 {
		t.Fatalf("untreated deaths %d/300, tended %d/300", deaths, tendedDeaths)
	}
}

func TestPumpHeartPreventsHeartAttack(t *testing.T) {
	var b Body
	b.Develop(CondArteryBlockage, Heart, 0)
	if b.HeartAttackMTB() != 300 {
		t.Fatalf("minor blockage MTB %d", b.HeartAttackMTB())
	}
	for _, k := range []item.Kind{item.PumpHeart, item.ValveHeart} {
		h := Body{Conds: append([]Condition(nil), b.Conds...)}
		if !h.Apply(Operation{Kind: InstallImplant, Part: Heart, Item: k}, Success, nil, 0) {
			t.Fatalf("fit %s", k)
		}
		if _, ok := h.Cond(CondArteryBlockage); ok || h.HeartAttackMTB() != 0 ||
			h.CanDevelop(CondArteryBlockage, Heart) || h.Develop(CondHeartAttack, Heart, 0) {
			t.Fatalf("an artificial heart (%s) ends the blockage and prevents heart attacks: %+v", k, h.Conds)
		}
	}
	// a natural heart transplant takes the old heart's blockage away too
	h := Body{Conds: append([]Condition(nil), b.Conds...)}
	h.Wounds = []Wound{{Part: Heart, Dmg: 1}}
	if !h.Apply(Operation{Kind: InstallOrgan, Part: Heart, Item: item.Heart}, Success, nil, 0) || h.Conds != nil {
		t.Fatalf("a new heart: %+v", h.Conds)
	}
}

func TestExciseCarcinoma(t *testing.T) {
	var b Body
	op := Operation{Kind: ExciseCarcinoma, Part: Stomach}
	if b.CanOperate(op) {
		t.Fatal("nothing to excise")
	}
	b.Develop(CondCarcinoma, Stomach, 0)
	b.Develop(CondCataract, EyeL, 0)
	if !b.CanOperate(op) || b.CanOperate(Operation{Kind: ExciseCarcinoma, Part: Liver}) {
		t.Fatal("excision is offered on the carcinoma's part only")
	}
	if MinSkill(ExciseCarcinoma) != 10 || FailDeathChance(op) != 250 {
		t.Fatal("wiki: Medicine 10, 25 % death on failure")
	}
	if q, least := MedicineNeed(ExciseCarcinoma); q != 4 || least != item.IndustrialMedicine {
		t.Fatal("wiki: 4 industrial medicine")
	}
	if !b.Apply(op, Success, nil, 0) || b.condOn(CondCarcinoma, Stomach) || !b.condOn(CondCataract, EyeL) {
		t.Fatalf("a successful excision removes the carcinoma only: %+v", b.Conds)
	}
}

func TestCataractCuredByEyeReplacement(t *testing.T) {
	var b Body
	b.Develop(CondCataract, EyeL, 0)
	b.Develop(CondCataract, EyeR, 0)
	b.Develop(CondHearingLoss, EarR, 0)
	if !b.Apply(Operation{Kind: InstallImplant, Part: EyeL, Item: item.OpticImplant}, Success, nil, 0) {
		t.Fatal("fit the optic implant")
	}
	if b.condOn(CondCataract, EyeL) || !b.condOn(CondCataract, EyeR) {
		t.Fatalf("the replaced eye's cataract goes, the other stays: %+v", b.Conds)
	}
	if b.Capacity(Sight) != (1250+500)/2 {
		t.Fatalf("sight %d", b.Capacity(Sight))
	}
	if !b.Apply(Operation{Kind: InstallImplant, Part: EarR, Item: item.CochlearImplant}, Success, nil, 0) || b.condOn(CondHearingLoss, EarR) {
		t.Fatal("a cochlear implant ends that ear's hearing loss")
	}
	// an eye lost to a wound takes its cataract along at the next step
	b.destroy(EyeR)
	if !b.DropLostAging() || b.condOn(CondCataract, EyeR) {
		t.Fatal("a lost eye has no cataract")
	}
	if b.CanDevelop(CondCataract, EyeR) || b.CanDevelop(CondCataract, EyeL) {
		t.Fatal("no cataract on a missing or artificial eye")
	}
}

func TestBadBackCuredBySpinalBrace(t *testing.T) {
	var b Body
	b.Develop(CondBadBack, Spine, 0)
	if !b.Apply(Operation{Kind: InstallImplant, Part: Spine, Item: item.SpinalBrace}, Success, nil, 0) {
		t.Fatal("fit the brace")
	}
	if _, ok := b.Cond(CondBadBack); ok || !b.SpineBraced() || b.CanDevelop(CondBadBack, Spine) {
		t.Fatalf("a spinal brace ends the bad back for good: %+v", b.Conds)
	}
	if b.Capacity(Moving) != 1000 {
		t.Fatalf("moving %d", b.Capacity(Moving))
	}
}

func TestPermanentChronicNeverChanges(t *testing.T) {
	var b Body
	for _, c := range Chronics() {
		if Permanent(c.Kind) {
			b.Develop(c.Kind, c.Parts[0], 0)
		}
	}
	before := append([]Condition(nil), b.Conds...)
	for d := uint64(1); d < 2000; d += 97 {
		if st := b.StepAging(d*clock.TicksPerDay, 3, nil); st.Changed || st.Died != CondNone {
			t.Fatalf("day %d: %+v", d, st)
		}
	}
	if len(b.Conds) != len(before) {
		t.Fatalf("%+v", b.Conds)
	}
	for i := range before {
		if b.Conds[i] != before[i] {
			t.Fatalf("%+v vs %+v", b.Conds[i], before[i])
		}
	}
}
