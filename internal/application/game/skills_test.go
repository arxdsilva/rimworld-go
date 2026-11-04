package game

import (
	"rimworld-go/internal/application/game/dossierview"
	rlab "rimworld-go/internal/application/game/research"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
	"rimworld-go/pkg/randx"
)

// genPawn is a hand-made generated colonist: a legacy body with the
// given skills/traits/backstories applied.
func genPawn(name string, edit func(p *pawngen.Pawn)) pawngen.Pawn {
	p := pawngen.Legacy(99, name, look.SkinCount, look.HairCount, look.ClothCount)
	p.Last = "Test"
	if edit != nil {
		edit(&p)
	}
	return p
}

func spawnGen(t *testing.T, g *Game, x, y int, p pawngen.Pawn) entity.EntityID {
	t.Helper()
	id, err := g.spawnGenerated(p, x, y)
	if err != nil {
		t.Fatalf("spawn generated: %v", err)
	}
	return id
}

// backstoryWith finds an adulthood/childhood backstory incapable of b.
func backstoryWith(t *testing.T, b backstory.Bits) backstory.Def {
	t.Helper()
	for _, d := range backstory.All() {
		if d.Incapable&b != 0 {
			return d
		}
	}
	t.Fatalf("no backstory incapable of %v", b)
	return backstory.Def{}
}

// TestNeutralPawnTimingsUnchanged: a neutral pawn (SpawnNamedPawn) works
// exactly as the old boolean workTick did — 1 unit on every tick it
// allowed, 0 on the low-mood ticks it skipped — skilled or not, and every
// other skill/trait hook is the identity for it. Plant work has its own speed
// curve (HRV-009, TestPlantWorkProgress).
func TestNeutralPawnTimingsUnchanged(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 12, Height: 8})
	happy, _ := g.SpawnNamedPawn(2, 2, "Ada")
	sad, _ := g.SpawnNamedPawn(5, 2, "Bo")
	g.SetNeeds(sad, needs.CriticalHunger-1, needs.CriticalRest-1)
	g.joy[sad] = 0
	if g.moodOf(sad) >= mood.LowMood {
		t.Fatalf("fixture: sad mood %d", g.moodOf(sad))
	}
	skipped := 0
	for i := 0; i < 4000; i++ {
		for _, id := range []entity.EntityID{happy, sad} {
			want := 0
			if g.workTick(id) {
				want = 1
			}
			sk := skill.ID(i % int(skill.NumSkills))
			if sk == skill.Plants {
				sk = skill.Mining
			}
			if got := g.workProgress(id, sk, i%3 != 0); got != want {
				t.Fatalf("tick %d pawn %d: progress %d, workTick %d", i, id, got, want)
			}
			if want == 0 {
				skipped++
			}
		}
		g.clock.Advance(1)
	}
	if skipped == 0 {
		t.Fatal("the low-mood skip never happened")
	}
	n := needs.Pair{Hunger: 5000, Rest: 5000}
	for now := uint64(0); now < 200; now++ {
		if a, b := g.decayNeeds(happy, n, now), needs.Decay(n, now); a != b {
			t.Fatalf("need decay %v != %v", a, b)
		}
	}
	if m := g.meleeOf(happy); m.Offset != 0 || m.Sight != 1000 || m.Manip != 1000 || m.Moving != 1000 || !m.Skilled || m.Down {
		t.Errorf("melee numbers moved for a neutral pawn: %+v", m)
	}
	if _, off := (combatSkills{g}).ShotAim(happy); off != 0 {
		t.Error("shooting offset moved for a neutral pawn")
	}
	if g.paceStep(happy, 17) != 17 || g.aimTicksFor(happy, 90) != 90 {
		t.Error("step or aim time moved for a neutral pawn")
	}
	if g.bandsOf(happy) != g.apparel[happy].Bands() { // TEMP-4: its everyday clothes are garments now, no shift on top
		t.Errorf("bands %v", g.bandsOf(happy))
	}
	if g.skillYield(happy, skill.Plants, 7) != 7 {
		t.Error("yields moved for a neutral pawn")
	}
	// FOOD-3: butchering is the wiki's efficiency, 92.5 % at Cooking 7
	if g.butcherYield(happy, 40) != 37 {
		t.Errorf("butcher yield %d of 40 at Cooking 7", g.butcherYield(happy, 40))
	}
	if (researchHost{g}).Speed(happy) != (rlab.Pawn{Intellectual: g.skillOf(happy, skIntellectual).Level, GlobalWork: 1000,
		Manipulation: 1000, Sight: 1000, TraitPct: 100}) { // game/research (RSCH-1)
		t.Error("research inputs moved for a neutral pawn")
	}
	if g.breakOffset(happy) != 0 || g.painShockAt(happy) != health.PainShock {
		t.Error("break or downed threshold moved")
	}
	if g.temperamentOf(happy).String() != legacyTemperament("Ada").String() {
		t.Error("temperament moved")
	}
}

// legacyTemperament is the historic name hash.
func legacyTemperament(n string) Temperament {
	return Temperament(pawngen.TemperamentOfName(n))
}

// TestSkillSpeedRatio: a level 20 builder outworks a level 0 one by
// 1780/580.
func TestSkillSpeedRatio(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	lv := func(l uint8) func(*pawngen.Pawn) {
		return func(p *pawngen.Pawn) { p.Skills[skill.Construction].Level = l }
	}
	master := spawnGen(t, g, 2, 2, genPawn("Master", lv(20)))
	novice := spawnGen(t, g, 5, 2, genPawn("Novice", lv(0)))
	um, un := 0, 0
	const ticks = 10000
	for i := 0; i < ticks; i++ {
		um += g.workProgress(master, skill.Construction, true)
		un += g.workProgress(novice, skill.Construction, true)
		g.clock.Advance(1)
	}
	if mathx.Abs(um-1780*ticks/1000) > 1 || mathx.Abs(un-580*ticks/1000) > 1 {
		t.Fatalf("units: master %d (want %d), novice %d (want %d)", um, 1780*ticks/1000, un, 580*ticks/1000)
	}
}

// TestLearningSaturation: one skill learns at most learnCapDay a day; the
// next day it learns again. Incapable skills never learn.
func TestLearningSaturation(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Kit", func(p *pawngen.Pawn) {
		p.Skills[skill.Mining].Passion = skill.Major
		p.Traits = []trait.ID{trait.Gentle}
	}))
	before := totalXP(g.skillOf(id, skill.Mining))
	for i := 0; i < 20000; i++ {
		g.learn(id, skill.Mining, 1)
	}
	day1 := totalXP(g.skillOf(id, skill.Mining)) - before
	if day1 > learnCapDay || day1 < learnCapDay-1 {
		t.Fatalf("learned %d in a day, cap %d", day1, learnCapDay)
	}
	g.clock.Advance(24 * 2500)
	g.learn(id, skill.Mining, 100)
	if got := totalXP(g.skillOf(id, skill.Mining)) - before - day1; got < 150 || got > 151 {
		t.Fatalf("next day learned %d, want 150 (major passion)", got)
	}
	if c, _ := g.Dossier(id); int64(c.LearnedWeek[skill.Mining]) != totalXP(g.skillOf(id, skill.Mining))-before {
		t.Errorf("this week %d", c.LearnedWeek[skill.Mining])
	}
	shoot := g.skillOf(id, skill.Shooting)
	g.learn(id, skill.Shooting, 500)
	g.combatLearn(id, true, 120)
	if g.skillOf(id, skill.Shooting) != shoot {
		t.Error("an incapable skill learned")
	}
}

// TestDecayNoFlapInPlay: a level 12 colonist alternating a day of
// neglect with a day of practice changes level at most once, and its
// daily rollover runs exactly once a day at its hour bucket.
func TestDecayNoFlapInPlay(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Ox", func(p *pawngen.Pawn) {
		p.Skills[skill.Mining] = skill.Skill{Level: 12, Passion: skill.Minor}
	}))
	changes, last := 0, uint8(12)
	for day := 0; day < 60; day++ {
		g.dayPasses(id)
		if day%2 == 1 {
			g.learn(id, skill.Mining, 3000)
		}
		g.clock.Advance(24 * 2500)
		if l := g.skillOf(id, skill.Mining).Level; l != last {
			changes, last = changes+1, l
		}
	}
	if changes > 1 {
		t.Fatalf("level flapped %d times", changes)
	}
	// the daily hook fires once per day, at the pawn's own hour
	p0 := g.sk.bios[id].AgeDays
	start := uint64(g.clock.Now())
	for now := start; now < start+uint64(24*2500); now++ {
		g.skillsDaily(id, now)
	}
	if got := g.sk.bios[id].AgeDays - p0; got != 1 {
		t.Fatalf("daily hook ran %d times in a day", got)
	}
}

// TestIncapablePriorityIgnoredAndOrderRefused: incapable work starts at
// 0, can't be switched on (a toast says why), is never assigned, and a
// forced order is refused with the reason.
func TestIncapablePriorityIgnoredAndOrderRefused(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	bs := backstoryWith(t, backstory.Mining)
	id := spawnGen(t, g, 2, 2, genPawn("Pip", func(p *pawngen.Pawn) {
		p.Adulthood = bs.ID
		p.Traits = []trait.ID{trait.Gentle}
	}))
	if g.prioOf(id)[work.Mine] != 0 || g.prioOf(id)[work.Hunt] != 0 {
		t.Fatalf("incapable work has priority: %v", g.prioOf(id))
	}
	g.Apply(command.Command{Kind: command.SetWorkPriority, Pawn: uint64(id), Work: command.WorkMine, Count: 1})
	g.Apply(command.Command{Kind: command.SetWorkEnabled, Pawn: uint64(id), Work: command.WorkHunt, Enabled: true})
	g.Step(1)
	if g.prioOf(id)[work.Mine] != 0 || g.prioOf(id)[work.Hunt] != 0 {
		t.Fatalf("incapable priority set: %v", g.prioOf(id))
	}
	toast := 0
	for _, m := range g.messages {
		if strings.HasPrefix(m.Text, "Incapable: Mining (backstory: "+bs.Title) ||
			strings.HasPrefix(m.Text, "Incapable: Violence (trait: Gentle)") {
			toast++
		}
	}
	if toast != 2 {
		t.Fatalf("toasts: %v", g.messages)
	}
	g.SetWork(id, work.Mine, true) // fixture bypass: still never assigned
	if g.assignWorkType(id, work.Mine) {
		t.Error("incapable work assigned")
	}
	if _, why := g.forcedJob(id, command.OrderMine, 6, 6, construction.LayerObject); !strings.HasPrefix(why, ReasonIncapableOf+"Mining") {
		t.Errorf("mine order: %q", why)
	}
	if _, why := g.forcedJob(id, command.OrderAttack, 6, 6, construction.LayerObject); !strings.HasPrefix(why, ReasonIncapableOf+"Violence") {
		t.Errorf("attack order: %q", why)
	}
	if g.IncapableReason(id, work.Hunt) == "" || g.IncapableReason(id, work.Construct) != "" {
		t.Error("work tab reasons")
	}
	// a neutral colonist is unaffected
	ada, _ := g.SpawnNamedPawn(4, 4, "Ada")
	if _, why := g.forcedJob(ada, command.OrderMine, 6, 6, construction.LayerObject); strings.HasPrefix(why, ReasonIncapableOf) {
		t.Errorf("neutral refused: %q", why)
	}
}

// TestViolenceIncapableNeverStrikes: a pacifist told to attack never
// swings, never auto-engages, and runs from danger even when armed.
func TestViolenceIncapableNeverStrikes(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Dove", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Gentle} }))
	aid, err := g.SpawnAnimal("wild_boar", 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	g.setDraft(id, true)
	g.startJob(id, job.Record{Kind: job.Attack, Target: aid, TX: 3, TY: 2, WorkLeft: 1})
	for i := 0; i < 300; i++ {
		g.Step(1)
		if _, struck := g.lastStrike[id]; struck {
			t.Fatal("a pacifist struck")
		}
	}
	g.combat.Strikes.AutoEngage(id)
	if j, _ := g.jobs.Get(id); j.Kind == job.Attack && j.Active() {
		t.Fatal("a pacifist engaged")
	}
	// control: a neutral colonist in the same spot does strike
	h := New(Config{Seed: 1, Width: 16, Height: 8})
	ctl, _ := h.SpawnNamedPawn(2, 2, "Hawk")
	bid, _ := h.SpawnAnimal("wild_boar", 3, 2)
	h.setDraft(ctl, true)
	h.startJob(ctl, job.Record{Kind: job.Attack, Target: bid, TX: 3, TY: 2, WorkLeft: 1})
	h.Step(300)
	if _, struck := h.lastStrike[ctl]; !struck {
		t.Fatal("control: a neutral colonist never struck")
	}
	if !g.canViolence(entity.EntityID(0)) {
		t.Error("an unknown attacker must default to capable")
	}
}

// TestAccuracyRisesWithSkill: 400 shots with one seed hit more often the
// better the shooter; Deliberate aim adds and Quick draw costs accuracy.
func TestAccuracyRisesWithSkill(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	shooter := func(name string, l uint8, tr ...trait.ID) entity.EntityID {
		return spawnGen(t, g, 1+len(g.pawnIDs), 2, genPawn(name, func(p *pawngen.Pawn) {
			p.Skills[skill.Shooting].Level = l
			p.Traits = tr
		}))
	}
	hits := func(id entity.EntityID) int {
		r := randx.New(42)
		n := 0
		pt, off := (combatSkills{g}).ShotAim(id) // CBT-2: the banded aim at 10 tiles
		c := attack.Report{Band: 550, Shooter: attack.ShooterFactor(pt, 10), Offset: off, Size: 1000, Weather: 1000, Light: 1000}.Aim()
		for range 400 {
			if r.IntN(1000) < c {
				n++
			}
		}
		return n
	}
	h0, h7, h20 := hits(shooter("A", 0)), hits(shooter("B", 7)), hits(shooter("C", 20))
	if !(h0 < h7 && h7 < h20) {
		t.Fatalf("hits 0/7/20: %d %d %d", h0, h7, h20)
	}
	if hits(shooter("D", 7, trait.DeliberateAim)) <= h7 || hits(shooter("E", 7, trait.QuickDraw)) >= h7 {
		t.Error("trigger traits")
	}
	q := shooter("F", 7, trait.QuickDraw)
	if g.aimTicksFor(q, 100) != 75 {
		t.Errorf("quick draw aim %d", g.aimTicksFor(q, 100))
	}
}

// TestTraitMoodAndBreakOffsets: Sunny/Brooding shift mood, Fussy doubles
// a hardship thought, Steady/Brittle move the break threshold.
func TestTraitMoodAndBreakOffsets(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	base, _ := g.SpawnNamedPawn(2, 2, "Ada")
	sunny := spawnGen(t, g, 3, 2, genPawn("Sun", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Sunny, trait.Brittle} }))
	gloom := spawnGen(t, g, 4, 2, genPawn("Rain", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Brooding, trait.Steady} }))
	fussy := spawnGen(t, g, 5, 2, genPawn("Prim", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Fussy} }))
	m := g.moodOf(base)
	if g.moodOf(sunny) != m+8 || g.moodOf(gloom) != m-8 {
		t.Fatalf("mood base %d sunny %d brooding %d", m, g.moodOf(sunny), g.moodOf(gloom))
	}
	for _, id := range []entity.EntityID{base, fussy} {
		g.remember(id, mood.SleptOutside)
	}
	if d := g.moodOf(fussy) - m; d != 2*mood.DefOf(mood.SleptOutside).Value {
		t.Errorf("fussy slept-outside %d", d)
	}
	if g.breakOffset(sunny) != 6 || g.breakOffset(gloom) != -6 {
		t.Fatalf("break offsets %d %d", g.breakOffset(sunny), g.breakOffset(gloom))
	}
	// at mood 17: brittle (threshold 26) risks more than steady (14: none)
	if mentalbreak.RiskPerHour(17-g.breakOffset(sunny)) <= mentalbreak.RiskPerHour(17) ||
		mentalbreak.RiskPerHour(17-g.breakOffset(gloom)) != 0 {
		t.Error("break risk")
	}
}

// TestBirthdayRebuildsStats: turning 50 slows work and walking.
func TestBirthdayRebuildsStats(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Old", func(p *pawngen.Pawn) { p.AgeDays = 50*pawngen.DaysPerYear - 1 }))
	if g.statOf(id, stat.WorkSpeed) != 1000 {
		t.Fatalf("49: work %d", g.statOf(id, stat.WorkSpeed))
	}
	g.dayPasses(id)
	if g.sk.bios[id].AgeYears() != 50 || g.statOf(id, stat.WorkSpeed) != 950 || g.statOf(id, stat.MoveSpeed) != 950 {
		t.Fatalf("50: age %d work %d move %d", g.sk.bios[id].AgeYears(), g.statOf(id, stat.WorkSpeed), g.statOf(id, stat.MoveSpeed))
	}
	if g.paceStep(id, 19) != 20 {
		t.Errorf("step %d", g.paceStep(id, 19))
	}
	if v := g.Snapshot()[0]; v.AgeYears != 50 {
		t.Errorf("view age %d", v.AgeYears)
	}
}

// TestLegacyLookUnchanged: a SpawnNamedPawn colonist's view look is the
// renderer's historic hash look (palette indices).
func TestLegacyLookUnchanged(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	for i := 0; i < 6; i++ {
		g.SpawnNamedPawn(1+i, 2, "")
	}
	for _, v := range g.Snapshot() {
		id := uint64(v.ID)
		pick := func(salt uint64, n int) uint8 { return uint8(rendererMix(id*0x9E3779B97F4A7C15+salt) % uint64(n)) }
		l := v.Look
		if l.Sex != look.Male || l.Build != look.Average || l.Skin != pick(1, look.SkinCount) ||
			l.Hair != pick(2, look.HairCount) || l.HairStyle != pick(3, 3) || l.Shirt != pick(4, look.ClothCount) {
			t.Fatalf("pawn %d look %+v", id, l)
		}
		if l != look.Legacy(id, look.SkinCount, look.HairCount, look.ClothCount) {
			t.Fatalf("pawn %d: view look differs from look.Legacy", id)
		}
		if v.Sex != "Male" || v.Build != "Average" || v.AgeYears != 30 || v.Incapable != 0 || v.SkillLv[skill.Mining] != skill.Neutral {
			t.Fatalf("legacy identity %+v", v)
		}
	}
}

// rendererMix is ebiten/art.go's historic splitmix64 finaliser. It stays a
// separate copy here: this test pins legacy looks independently of randx.
func rendererMix(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return x
}

// TestBioViewInGame: the Bio tab shows the generated identity with live
// skills; the pawn view carries sex, age, title, levels and passions.
func TestBioViewInGame(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	bs := backstoryWith(t, backstory.Violence)
	id := spawnGen(t, g, 2, 2, genPawn("Wren", func(p *pawngen.Pawn) {
		p.Look.Sex = look.Female
		p.Adulthood = bs.ID
		p.AgeDays = 41 * pawngen.DaysPerYear
		p.Skills[skill.Plants] = skill.Skill{Level: 9, Passion: skill.Major}
	}))
	g.learn(id, skill.Plants, 2000)
	b, ok := g.Bio(id)
	if !ok || b.Nick != "Wren" || b.Sex != "Female" || b.AgeYears != 41 || b.Adulthood != bs.Title {
		t.Fatalf("bio %+v", b)
	}
	if s := b.Skills[skill.Plants]; s.Level != 9 || s.Passion != 2 || s.XPPct == 0 {
		t.Fatalf("plants %+v", s)
	}
	if !b.Skills[skill.Shooting].Incapable || len(b.Incapable) == 0 {
		t.Fatalf("incapable %+v", b.Incapable)
	}
	v := g.Snapshot()[0]
	if v.Sex != "Female" || v.AgeYears != 41 || v.Title != bs.Title || v.SkillLv[skill.Plants] != 9 ||
		v.Passion[skill.Plants] != uint8(skill.Major) || !v.Incapable.Enabled(work.Hunt) {
		t.Fatalf("view %+v", v)
	}
	if _, ok := g.Bio(entity.EntityID(999)); ok {
		t.Error("bio of nobody")
	}
}

// TestImportBiosMigratesLegacy: a save's bios round-trip; colonists of an
// older save without any get their legacy identity.
func TestImportBiosMigratesLegacy(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Sol", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Tireless} }))
	ada, _ := g.SpawnNamedPawn(3, 2, "Ada")
	g.learn(id, skill.Mining, 900)
	bios, skills, carry := g.exportBios()

	h := New(Config{Seed: 1, Width: 12, Height: 8})
	h.SpawnNamedPawn(2, 2, "Sol")
	h.SpawnNamedPawn(3, 2, "Ada")
	h.sk = skillState{}
	h.importBios(bios, skills, carry)
	if h.skillOf(id, skill.Mining) != g.skillOf(id, skill.Mining) || h.statOf(id, stat.WorkSpeed) != 1200 {
		t.Fatal("bios did not round-trip")
	}
	old := New(Config{Seed: 1, Width: 12, Height: 8})
	old.SpawnNamedPawn(2, 2, "Sol")
	oa, _ := old.SpawnNamedPawn(3, 2, "Ada")
	old.sk = skillState{} // a v1 save: no bios
	old.importBios(nil, nil, nil)
	if p, ok := old.bioFor(oa); !ok || p.Look != g.sk.bios[ada].Look || p.Skills != skill.NeutralSet() {
		t.Fatal("legacy migration")
	}
}

// card is id's dossier, which must exist.
func card(t *testing.T, g *Game, id entity.EntityID) dossierview.Card {
	t.Helper()
	c, ok := g.Dossier(id)
	if !ok {
		t.Fatalf("no card for %d", id)
	}
	return c
}

// TestDossierBreakLevelsShiftWithTraits: the card's levels are the ones the
// mind breaks at, and a running break reports its tier.
func TestDossierBreakLevelsShiftWithTraits(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	plain, _ := g.SpawnNamedPawn(2, 2, "Ada")
	steady := spawnGen(t, g, 3, 2, genPawn("Rain", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Steady} }))
	brittle := spawnGen(t, g, 4, 2, genPawn("Sun", func(p *pawngen.Pawn) { p.Traits = []trait.ID{trait.Brittle} }))
	for _, w := range []struct {
		id   entity.EntityID
		want dossierview.BreakLevels
	}{
		{plain, dossierview.BreakLevels{Minor: 35, Major: 20, Extreme: 5}},
		{steady, dossierview.BreakLevels{Minor: 29, Major: 16, Extreme: 4}},
		{brittle, dossierview.BreakLevels{Minor: 41, Major: 23, Extreme: 5}},
	} {
		if got := card(t, g, w.id).BreakAt; got != w.want {
			t.Errorf("%s: levels %+v, want %+v", g.name(w.id), got, w.want)
		}
	}
	if c := card(t, g, plain); c.BreakTier != mentalbreak.TierNone {
		t.Errorf("no break, tier %v", c.BreakTier)
	}
	g.ForceBreak(plain, mentalbreak.Tantrum)
	if got, want := card(t, g, plain).BreakTier, mentalbreak.TierOf(mentalbreak.Tantrum); got != want {
		t.Errorf("tier %v, want %v", got, want)
	}
}

// TestDossierSaturatedAtCap: a skill at the day's cap reads saturated, others
// don't, and the next day clears it. Reading rolls nothing over.
func TestDossierSaturatedAtCap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id := spawnGen(t, g, 2, 2, genPawn("Rain", func(p *pawngen.Pawn) { p.Skills[skill.Mining].Passion = skill.Major }))
	if c := card(t, g, id); c.Saturated[skill.Mining] {
		t.Fatal("saturated before learning anything")
	}
	g.learn(id, skill.Mining, 100)
	if c := card(t, g, id); c.Saturated[skill.Mining] {
		t.Fatal("saturated after a little learning")
	}
	for i := 0; i < 20000; i++ {
		g.learn(id, skill.Mining, 1)
	}
	c := card(t, g, id)
	if !c.Saturated[skill.Mining] || c.Saturated[skill.Shooting] {
		t.Fatalf("saturated %v", c.Saturated)
	}
	day := g.sk.learned[id].day
	g.clock.Advance(24 * 2500)
	if c := card(t, g, id); c.Saturated[skill.Mining] {
		t.Fatal("still saturated the next day")
	}
	if g.sk.learned[id].day != day {
		t.Fatal("the card rolled the learning log over")
	}
}

// TestDossierActiveSkillExpires: -1 when idle, the skill while it progresses,
// -1 again after the sim's own workingTicks.
func TestDossierActiveSkillExpires(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	if c := card(t, g, id); c.ActiveSkill != dossierview.NoneID || c.WorkType != dossierview.NoneID {
		t.Fatalf("idle: active %d, work %d", c.ActiveSkill, c.WorkType)
	}
	g.jobs.Set(id, job.Record{Kind: job.Mine, State: job.Do})
	g.workProgress(id, skill.Mining, true)
	c := card(t, g, id)
	if c.ActiveSkill != int(skill.Mining) || c.WorkType != int(work.Mine) {
		t.Fatalf("mining: active %d, work %d", c.ActiveSkill, c.WorkType)
	}
	g.clock.Advance(workingTicks)
	if c := card(t, g, id); c.ActiveSkill != int(skill.Mining) {
		t.Fatalf("at workingTicks: active %d", c.ActiveSkill)
	}
	g.clock.Advance(1)
	if c := card(t, g, id); c.ActiveSkill != dossierview.NoneID {
		t.Fatalf("past workingTicks: active %d", c.ActiveSkill)
	}
}

// TestDossierStunUnsaved: a stun shows on the card, moves no state hash, and
// a save does not carry it; a pawn only waiting to step is not stunned.
func TestDossierStunUnsaved(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	id, _ := g.SpawnNamedPawn(2, 2, "Ada")
	now := uint64(g.clock.Now())
	m := g.motionOf(id)
	m.NextMoveAt = now + 50
	g.motion.Set(id, m)
	if c := card(t, g, id); c.StunTicks != 0 {
		t.Fatalf("a walker reads stunned for %d", c.StunTicks)
	}
	before := g.StateHash()
	g.sk.noteStun(id, now+100, now)
	if got := g.StateHash(); got != before {
		t.Fatalf("the stun record moved the hash: %#x, was %#x", got, before)
	}
	hitHarm{g}.Hold(id, now+120)
	if c := card(t, g, id); c.StunTicks != 120 {
		t.Fatalf("stun ticks %d, want 120", c.StunTicks)
	}
	if c := card(t, saveLoad(t, g), id); c.StunTicks != 0 {
		t.Fatalf("a load kept the stun: %d", c.StunTicks)
	}
	hitHarm{g}.SetStaggerUntil(id, now+30)
	if c := card(t, g, id); c.StaggerTicks != 30 {
		t.Fatalf("stagger ticks %d, want 30", c.StaggerTicks)
	}
	g.clock.Advance(121)
	if c := card(t, g, id); c.StunTicks != 0 || c.StaggerTicks != 0 {
		t.Fatalf("spent timers read %d, %d", c.StunTicks, c.StaggerTicks)
	}
	for i := entity.EntityID(1000); i < 1000+stunSweep+1; i++ {
		g.sk.noteStun(i, uint64(g.clock.Now()), uint64(g.clock.Now())) // spent at once
	}
	if len(g.sk.stun) > stunSweep {
		t.Fatalf("spent stuns pile up: %d", len(g.sk.stun))
	}
}

// TestSetNameRoundTripsASave: colonists and tame animals take a name that
// survives a save, "" gives back the generated one even after a load, and
// nobody else can be renamed.
func TestSetNameRoundTripsASave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	ada := spawnGen(t, g, 2, 2, genPawn("Rain", nil))
	pet, err := g.spawnAnimal(animal.Species(0), 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	wild, err := g.spawnAnimal(animal.Species(0), 5, 4)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := g.animals.Get(pet)
	a.Tame, a.Name = true, "Biscuit"
	g.animals.Set(pet, a)
	petName := func(g *Game) string { a, _ := g.animals.Get(pet); return a.Name }
	send := func(g *Game, id entity.EntityID, name string) {
		applySetName(g, command.Command{Kind: command.SetName, Pawn: uint64(id), Name: name})
	}
	clean := g.StateHash()
	send(g, ada, " Stormy ")
	send(g, pet, "Rex")
	send(g, pet, "Rex-Two")
	send(g, wild, "Nope")
	send(g, 999999, "Nope")
	send(g, ada, "Way too long a name for anyone")
	send(g, ada, "R2D2")
	if g.name(ada) != "Stormy" || petName(g) != "Rex-Two" {
		t.Fatalf("renamed: %q, %q", g.name(ada), petName(g))
	}
	if a, _ := g.animals.Get(wild); a.Name != "" {
		t.Fatalf("a wild animal took a name: %q", a.Name)
	}
	if g.isPawn(999999) {
		t.Fatal("a rename made a pawn")
	}
	h := saveLoad(t, g)
	if h.name(ada) != "Stormy" || petName(h) != "Rex-Two" {
		t.Fatalf("after a load: %q, %q", h.name(ada), petName(h))
	}
	if v := h.Snapshot(); v[0].Name != "Stormy" {
		t.Fatalf("snapshot name %q", v[0].Name)
	}
	send(h, ada, "")
	send(h, pet, "")
	if h.name(ada) != "Rain" || petName(h) != "Biscuit" {
		t.Fatalf("restored: %q, %q", h.name(ada), petName(h))
	}
	if len(h.sk.exportPetNames()) != 0 {
		t.Fatal("a restored name stays set aside")
	}
	send(g, ada, "")
	send(g, pet, "")
	if got := g.StateHash(); got != clean {
		t.Fatalf("renamed and restored, hash %#x, was %#x", got, clean)
	}
	send(g, pet, "") // nothing to restore: no change
	if petName(g) != "Biscuit" {
		t.Fatalf("second restore: %q", petName(g))
	}
}

// TestSetNameRefusesVisitorsAndRaiders: through the real command path a
// colonist's new name shows at once, even paused, and a raider keeps its own.
func TestSetNameRefusesVisitorsAndRaiders(t *testing.T) {
	t.Parallel()
	g, ada := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	was := g.name(rid)
	_ = g.Snapshot()
	g.Enqueue(command.Command{Kind: command.SetName, Pawn: ada, Name: "Stormy"})
	g.Enqueue(command.Command{Kind: command.SetName, Pawn: uint64(rid), Name: "Stormy"})
	g.ApplyPending()
	if g.name(rid) != was {
		t.Fatalf("a raider was renamed %q", g.name(rid))
	}
	found := false
	for _, v := range g.Snapshot() {
		found = found || (v.ID == uint64ToID(ada) && v.Name == "Stormy")
	}
	if !found {
		t.Fatal("the paused snapshot missed the new name")
	}
	v, ca := visitorCard(t)
	if ca.Relation != dossierview.RelationNone || ca.Faction == "" || ca.FactionEmblem == "" {
		t.Fatalf("visitor card %+v", ca)
	}
	vn := v.name(v.trade.caravan.Trader)
	applySetName(v, command.Command{Kind: command.SetName, Pawn: uint64(v.trade.caravan.Trader), Name: "Stormy"})
	if v.name(v.trade.caravan.Trader) != vn {
		t.Fatal("a visitor was renamed")
	}
}

// visitorCard is a game with a trader caravan and its trader's card.
func visitorCard(t *testing.T) (*Game, dossierview.Card) {
	t.Helper()
	g, c := caravanGame(t)
	return g, card(t, g, c.Trader)
}

// TestDossierFacts: the rest of the card, for a colonist, a trader, a raider,
// a bill, an attack and animals; an unknown id has no card.
func TestDossierFacts(t *testing.T) {
	t.Parallel()
	g, ada := raidGame(t)
	col := uint64ToID(ada)
	if _, ok := g.Dossier(999999); ok {
		t.Fatal("a card for nobody")
	}
	c := card(t, g, col)
	if c.Relation != dossierview.RelationOwn || c.FactionEmblem != "ui_icon/faction_player" || c.Faction != "" {
		t.Fatalf("colonist %+v", c)
	}
	if c.Stats != stat.Base() && c.Stats != g.sk.stats[col].sheet {
		t.Fatalf("stats %v", c.Stats)
	}
	if c.EatSpeedPct != 100 {
		t.Fatalf("eating speed %d", c.EatSpeedPct)
	}
	if c.AgeDayOfYear != int(g.sk.bios[col].AgeDays)%pawngen.DaysPerYear {
		t.Fatalf("age day %d", c.AgeDayOfYear)
	}
	m := g.meleeOf(col)
	var hit, dodge, yield int
	for _, l := range c.ExtraLines[skill.Melee] {
		switch l.Label {
		case "Melee hit chance":
			hit = l.Pct
		case "Melee dodge chance":
			dodge = l.Pct
		}
	}
	if hit != attack.HitChanceOf(m)/10 || dodge != attack.DodgeChanceOf(m)/10 {
		t.Fatalf("melee lines %d, %d", hit, dodge)
	}
	yield = c.ExtraLines[skill.Mining][0].Pct
	if yield != g.skillPct(col, skill.Mining, 4, 60, 140) {
		t.Fatalf("mining yield %d", yield)
	}

	g.Raid(1)
	rid := g.raiderIDs[0]
	if r := card(t, g, rid); r.Relation != dossierview.RelationFoe {
		t.Fatalf("raider relation %v", r.Relation)
	}
	g.jobs.Set(col, job.Record{Kind: job.Attack, State: job.Do, Target: rid})
	if c := card(t, g, col); c.AttackTarget != g.name(rid) || c.WorkType != dossierview.NoneID {
		t.Fatalf("attacking: %+v", c.AttackTarget)
	}
	g.craft.Bills.Set(77, []recipe.Bill{{Recipe: recipe.CampfireMeal}})
	g.jobs.Set(col, job.Record{Kind: job.Bill, State: job.Do, Dest: 77})
	if c := card(t, g, col); c.WorkType != int(recipe.DefOf(recipe.CampfireMeal).Skill) {
		t.Fatalf("bill work type %d", c.WorkType)
	}
	g.jobs.Set(col, job.Record{Kind: job.Bill, State: job.Do, Dest: 78})
	if c := card(t, g, col); c.WorkType != dossierview.NoneID {
		t.Fatalf("a bill gone from its bench: work type %d", c.WorkType)
	}

	aid, err := g.spawnAnimal(animal.Species(0), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if an := card(t, g, aid); an.Relation != dossierview.RelationWild || an.ActiveSkill != dossierview.NoneID || an.WorkType != dossierview.NoneID {
		t.Fatalf("wild animal %+v", an)
	}
	a, _ := g.animals.Get(aid)
	a.Tame, a.Mode, a.Target = true, animal.Attack, rid
	g.animals.Set(aid, a)
	if an := card(t, g, aid); an.Relation != dossierview.RelationOwn || an.AttackTarget != g.name(rid) {
		t.Fatalf("tame hunter %+v", an)
	}

	v, tc := caravanGame(t)
	if got := card(t, v, tc.Trader).Role; got == "" {
		t.Fatal("a trader without a role")
	}
	if len(tc.Members) > 1 {
		for _, id := range tc.Members {
			if id != tc.Trader && card(t, v, id).Role != "" {
				t.Fatal("a guard with a trader's role")
			}
		}
	}
}
