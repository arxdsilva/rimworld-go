package game

import (
	"strings"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Skills, stats and traits in play (NG-1 §5.4). Every colonist (and any
// raider SETUP gives a bio) has a generated identity (pawngen.Pawn), a
// live skill set, and a derived stat sheet. A neutral pawn — what
// SpawnNamedPawn and old saves make: all skills 7, no traits, no
// incapabilities — progresses work exactly one unit per tick, hits as
// often, eats, sleeps and walks as before, so legacy timings are
// unchanged. Owned by PAWN-G.

// Learning and work constants (NG-1 §5.4).
const (
	// learnCapDay is the most experience one skill can gain per day
	// (saturation), so fast game speeds can't over-level.
	learnCapDay = 4000
	// chatXP is the Social experience of one finished chat.
	chatXP = 40
	// workingTicks: a pawn counts as doing skilled work (passion joy)
	// this long after its last unit of progress.
	workingTicks = 2
	// fumbleBelow: harvesting below this Plants level may fail.
	fumbleBelow = 4
	// lovesWorkMajor is the passion thought for a major passion.
	lovesWorkMajor = 4
)

// skillState is the per-pawn identity, skill and stat data.
type skillState struct {
	bios      map[entity.EntityID]pawngen.Pawn // generated identity (saved)
	skills    map[entity.EntityID]*skill.Set   // live skills (saved)
	stats     map[entity.EntityID]*pawnFx      // derived, rebuilt on change
	workCarry map[entity.EntityID]int32        // fractional work, permille (saved)
	learned   map[entity.EntityID]*learnLog    // saturation and weekly learning
	working   map[entity.EntityID]workingRec   // last skilled work (transient)
	carry     map[entity.EntityID]*needCarry   // fractional need drains
	// research: fractional research points (hundredths) under a
	// research speed or cost that is not 100%.
	research int
}

// pawnFx is everything derived from a bio: the stat sheet and the per
// skill trait/condition factors.
type pawnFx struct {
	sheet     stat.Sheet
	workPct   [skill.NumSkills]int16 // pawngen.SkillWorkPct
	learnPct  [skill.NumSkills]int16 // pawngen.LearnPct
	incapable backstory.Bits
	traits    []trait.ID
	neutral   bool    // base sheet, all factors 100%: the fast path
	body      bodyKey // the capacities the body layer was built from (body.go)
}

// learnLog is how much each skill learned today and in the last 7 days,
// and the hundredths of a point learning-rate factors left over.
type learnLog struct {
	day   int32
	today [skill.NumSkills]int32
	week  [7][skill.NumSkills]int32 // by day % 7
	carry [skill.NumSkills]int16
}

type workingRec struct {
	sk   skill.ID
	tick uint64
}

// needCarry holds the permille left over by non-neutral need rates.
type needCarry struct{ hunger, rest, joy int32 }

func (s *skillState) ensure() {
	if s.bios != nil {
		return
	}
	s.bios = make(map[entity.EntityID]pawngen.Pawn)
	s.skills = make(map[entity.EntityID]*skill.Set)
	s.stats = make(map[entity.EntityID]*pawnFx)
	s.workCarry = make(map[entity.EntityID]int32)
	s.learned = make(map[entity.EntityID]*learnLog)
	s.working = make(map[entity.EntityID]workingRec)
	s.carry = make(map[entity.EntityID]*needCarry)
}

// legacyBio is the neutral identity of pawn id called name (old saves,
// SpawnNamedPawn): its look is exactly the renderer's historic look.
func legacyBio(id entity.EntityID, name string) pawngen.Pawn {
	return pawngen.Legacy(uint64(id), name, look.SkinCount, look.HairCount, look.ClothCount)
}

// giveBio installs p as id's identity: skills from p, stats rebuilt.
// SETUP calls it for raiders it wants generated; spawnGenerated and
// SpawnNamedPawn call it for colonists.
func (g *Game) giveBio(id entity.EntityID, p pawngen.Pawn) {
	g.sk.ensure()
	g.sk.bios[id] = p
	set := p.Skills
	g.sk.skills[id] = &set
	g.rebuildStats(id)
}

// spawnGenerated creates colonist p at (x, y): the same stores as
// SpawnNamedPawn plus its bio, skills and stats; incapable work types
// start at priority 0. It fails when the tile is not walkable. SETUP
// spawns through it (spawnGeneratedFn = (*Game).spawnGenerated).
func (g *Game) spawnGenerated(p pawngen.Pawn, x, y int) (entity.EntityID, error) {
	name := p.Nick
	if name == "" {
		name = p.First
	}
	id, err := g.SpawnNamedPawn(x, y, name)
	if err != nil {
		return 0, err
	}
	g.giveBio(id, p)
	pr := g.prio[id]
	inc := p.Incapable().WorkMask()
	for t := work.Type(0); t < work.NumTypes; t++ {
		if inc.Enabled(t) {
			pr[t] = 0
		}
	}
	g.prio[id] = pr
	g.labor.Set(id, pr.Mask())
	for _, c := range p.Conditions {
		if c == pawngen.Exhausted {
			n, _ := g.need.Get(id)
			n.Rest = needs.Full * pawngen.ExhaustedRest / 100
			g.need.Set(id, n)
		}
	}
	// NoClothes (p.Look.Bare): colonists arrive with no worn apparel
	// anyway; the bare look is the renderer's.
	return id, nil
}

// bioFor returns id's identity (ok false for pawns without a bio).
func (g *Game) bioFor(id entity.EntityID) (pawngen.Pawn, bool) {
	p, ok := g.sk.bios[id]
	return p, ok
}

// rebuildStats derives id's stat sheet and factors from its bio: body,
// age, conditions, traits, then the difficulty layer, then the wounded
// body's capacities (body.go; a pawn without a bio gets that layer alone).
func (g *Game) rebuildStats(id entity.EntityID) {
	key := keyOf(g.bodies[id]) // body.go: capacities, recomputed only on change
	p, ok := g.sk.bios[id]
	if !ok {
		if key == healthyKey {
			delete(g.sk.stats, id)
			return
		}
		fx := &pawnFx{sheet: stat.Build(stat.Base(), bodyMods(key)), body: key}
		for sk := skill.ID(0); sk < skill.NumSkills; sk++ {
			fx.workPct[sk], fx.learnPct[sk] = 100, 100
		}
		g.sk.stats[id] = fx
		return
	}
	layers := append(pawngen.StatLayers(p), g.difficultyMods(), bodyMods(key))
	fx := &pawnFx{sheet: stat.Build(stat.Base(), layers...), incapable: p.Incapable(), traits: p.Traits, body: key}
	fx.neutral = fx.sheet == stat.Base()
	for sk := skill.ID(0); sk < skill.NumSkills; sk++ {
		fx.workPct[sk] = int16(pawngen.SkillWorkPct(p, sk))
		fx.learnPct[sk] = int16(pawngen.LearnPct(p, sk))
		if fx.workPct[sk] != 100 || fx.learnPct[sk] != 100 {
			fx.neutral = false
		}
	}
	g.sk.stats[id] = fx
}

// rebuildAllStats rebuilds every sheet (difficulty change; SETUP calls it
// after command.SetDifficulty).
func (g *Game) rebuildAllStats() {
	for id := range g.sk.bios {
		g.rebuildStats(id)
	}
}

// difficultyMods is the difficulty step of the stat sheet: pain
// tolerance (mood is a thought line, see thoughts).
func (g *Game) difficultyMods() []stat.Mod {
	if pt := g.diffPainTolerance(); pt != 0 {
		return []stat.Mod{{Stat: stat.PainTolerance, Add: int32(pt)}}
	}
	return nil
}

// statOf is one stat of id (the neutral value for pawns without a bio).
func (g *Game) statOf(id entity.EntityID, s stat.ID) int32 {
	if fx := g.sk.stats[id]; fx != nil {
		return fx.sheet[s]
	}
	return stat.Base()[s]
}

// skillOf is id's skill sk (neutral for pawns without a bio).
func (g *Game) skillOf(id entity.EntityID, sk skill.ID) skill.Skill {
	if s := g.sk.skills[id]; s != nil && sk < skill.NumSkills {
		return s[sk]
	}
	return skill.Skill{Level: skill.Neutral}
}

// incapableOf is everything id can't do.
func (g *Game) incapableOf(id entity.EntityID) backstory.Bits {
	if fx := g.sk.stats[id]; fx != nil {
		return fx.incapable
	}
	return 0
}

// incapableWork is the work types id can never do.
func (g *Game) incapableWork(id entity.EntityID) work.Mask { return g.incapableOf(id).WorkMask() }

// canViolence reports whether id may attack at all.
func (g *Game) canViolence(id entity.EntityID) bool {
	return g.incapableOf(id)&backstory.Violence == 0
}

// incapableWhy names why id can't do work type t: "Violence (backstory:
// Convoy mechanic)"; "" when it can.
func (g *Game) incapableWhy(id entity.EntityID, t work.Type) string {
	if !g.incapableWork(id).Enabled(t) {
		return ""
	}
	p := g.sk.bios[id]
	reasons := incapableReasons(p) // one per bit, in bit order
	i := 0
	for b := backstory.Bits(1); b&backstory.AllBits != 0; b <<= 1 {
		if p.Incapable()&b == 0 {
			continue
		}
		if b.WorkMask().Enabled(t) && i < len(reasons) {
			return reasons[i]
		}
		i++
	}
	return "incapable"
}

// IncapableReason is the Work tab tooltip of colonist id's cell for work
// type t ("Incapable: Violence (backstory: Convoy mechanic)"), "" when it
// can do that work.
func (g *Game) IncapableReason(id entity.EntityID, t work.Type) string {
	if why := g.incapableWhy(id, t); why != "" {
		return "Incapable: " + why
	}
	return ""
}

// --- work progress and learning ------------------------------------

// workProgress is how much work pawn id does this tick on work trained
// by skill sk (hasSkill false: work speed only). rate = mood gate ×
// WorkSpeed × skill speed × trait/condition skill factor, carried in
// permille, so a neutral pawn does exactly 1 unit a tick — and 0 on the
// low-mood ticks workTick has always skipped. Skilled work also trains sk.
func (g *Game) workProgress(id entity.EntityID, sk skill.ID, hasSkill bool) int {
	return g.workProgressPct(id, sk, hasSkill, 100)
}

// workProgressPct is workProgress with an extra speed factor in percent
// (a bench's tool cabinets).
func (g *Game) workProgressPct(id entity.EntityID, sk skill.ID, hasSkill bool, pct int) int {
	if hasSkill {
		g.sk.ensure()
		g.sk.working[id] = workingRec{sk: sk, tick: uint64(g.clock.Now())}
	}
	if !g.workTick(id) {
		return 0
	}
	rate := int64(1000)
	if fx := g.sk.stats[id]; fx != nil && !fx.neutral || hasSkill && g.skillOf(id, sk).Level != skill.Neutral || pct != 100 {
		rate = int64(g.statOf(id, stat.WorkSpeed)) * int64(pct) / 100
		if hasSkill {
			rate = rate * int64(skill.SpeedPermil(g.skillOf(id, sk).Level)) / 1000
			if fx := g.sk.stats[id]; fx != nil {
				rate = rate * int64(fx.workPct[sk]) / 100
			}
		}
	}
	units := 1
	if c := g.sk.workCarry[id]; rate != 1000 || c != 0 {
		g.sk.ensure()
		total := int64(c) + max(rate, 1)
		units = int(total / 1000)
		if rest := int32(total % 1000); rest != 0 {
			g.sk.workCarry[id] = rest
		} else {
			delete(g.sk.workCarry, id)
		}
	}
	if hasSkill && units > 0 {
		g.learn(id, sk, units)
	}
	return units
}

// learn trains id's skill sk by xp points of raw experience (× LearnRate
// × trait learning factor; the passion multiplier is skill.Learn's),
// capped at learnCapDay per skill per day. Incapable skills never learn;
// pawns without a skill set (animals, plain raiders) learn nothing.
func (g *Game) learn(id entity.EntityID, sk skill.ID, xp int) {
	set := g.sk.skills[id]
	if set == nil || sk >= skill.NumSkills || xp <= 0 || g.incapableOf(id).DisablesSkill(sk) {
		return
	}
	lg := g.learnLogOf(id)
	if fx := g.sk.stats[id]; fx != nil && !fx.neutral {
		// LearnRate × trait learning, hundredths carried per skill
		f := int64(fx.sheet[stat.LearnRate]) * int64(fx.learnPct[sk]) / 1000
		raw := int64(xp)*f + int64(lg.carry[sk])
		xp, lg.carry[sk] = int(raw/100), int16(raw%100)
		if xp <= 0 {
			return
		}
	}
	s := &set[sk]
	pct := int64(passionPct(s.Passion))
	left := int64(learnCapDay - lg.today[sk])
	if left <= 0 {
		return
	}
	if (int64(xp)*pct+int64(s.Carry))/100 > left { // skill.Learn's exact gain
		xp = int(((left+1)*100 - 1 - int64(s.Carry)) / pct)
		if xp <= 0 {
			return
		}
	}
	before, lv := totalXP(*s), s.Level
	skill.Learn(s, xp)
	gain := int32(totalXP(*s) - before)
	lg.today[sk] += gain
	lg.week[lg.day%7][sk] += gain
	if s.Level != lv {
		g.rebuildStats(id)
	}
}

// passionPct mirrors skill.Learn's passion multiplier (for the cap).
func passionPct(p skill.Passion) int {
	switch p {
	case skill.Minor:
		return 100
	case skill.Major:
		return 150
	}
	return 35
}

// totalXP is a skill's experience since level 0.
func totalXP(s skill.Skill) int64 {
	t := int64(s.XP)
	for l := uint8(0); l < s.Level; l++ {
		t += int64(skill.Cost(l))
	}
	return t
}

// learnLogOf is id's learning log, rolled over to today: a new day
// clears the saturation count and that weekday's slot.
func (g *Game) learnLogOf(id entity.EntityID) *learnLog {
	g.sk.ensure()
	day := int32(uint64(g.clock.Now()) / clock.TicksPerDay)
	lg := g.sk.learned[id]
	if lg == nil {
		lg = &learnLog{day: day}
		g.sk.learned[id] = lg
	}
	if lg.day != day {
		for d := lg.day + 1; d <= day && d <= lg.day+7; d++ {
			lg.week[d%7] = [skill.NumSkills]int32{}
		}
		lg.today, lg.day = [skill.NumSkills]int32{}, day
	}
	return lg
}

// LearnedThisWeek is the experience each skill of colonist id gained in
// the last 7 days (Bio tab "This week: +N"), in skill.ID order.
func (g *Game) LearnedThisWeek(id entity.EntityID) [skill.NumSkills]int {
	var out [skill.NumSkills]int
	lg := g.sk.learned[id]
	if lg == nil {
		return out
	}
	day := int32(uint64(g.clock.Now()) / clock.TicksPerDay)
	for d := day - 6; d <= day; d++ {
		if d < 0 || d > lg.day || lg.day-d >= 7 {
			continue
		}
		for sk := range out {
			out[sk] += int(lg.week[d%7][sk])
		}
	}
	return out
}

// combatLearn trains the attack skill of a colonist that just struck by
// xp (CBT-2: attack.CombatXP, a hit teaching more than a miss).
func (g *Game) combatLearn(id entity.EntityID, ranged bool, xp int) {
	sk := skill.Melee
	if ranged {
		sk = skill.Shooting
	}
	g.learn(id, sk, xp)
}

// skillsDaily runs once a day at id's hour bucket (id % 24, as mental
// breaks spread theirs): skill decay, a day of age, and on birthdays a
// stat rebuild (age bands move work and move speed).
func (g *Game) skillsDaily(id entity.EntityID, now uint64) {
	if (now+uint64(clock.TicksPerDay)-uint64(id%24)*clock.TicksPerHour)%clock.TicksPerDay != 0 {
		return
	}
	g.dayPasses(id)
}

// dayPasses is one day for id's skills and age.
func (g *Game) dayPasses(id entity.EntityID) {
	p, ok := g.sk.bios[id]
	if !ok {
		return
	}
	if set := g.sk.skills[id]; set != nil {
		changed := false
		for sk := range set {
			l := set[sk].Level
			skill.DecayDay(&set[sk])
			changed = changed || set[sk].Level != l
		}
		if changed {
			g.rebuildStats(id)
		}
	}
	p.AgeDays++
	g.sk.bios[id] = p
	if p.AgeDays%pawngen.DaysPerYear == 0 {
		g.rebuildStats(id) // birthday (letter: optional, off; not shown)
	}
}

// --- yields, accuracy, traits ---------------------------------------

// skillPct is the ±4%/level factor around level 7, clamped to lo..hi.
func (g *Game) skillPct(id entity.EntityID, sk skill.ID, perLevel, lo, hi int) int {
	l := int(g.skillOf(id, sk).Level)
	return min(max(100+perLevel*(l-skill.Neutral), lo), hi)
}

// skillYield scales a harvest/mining/butcher yield by sk: ×clamp(100 +
// 4·(L−7), 60, 140)% (exact for a neutral pawn). Positive yields stay at
// least 1.
func (g *Game) skillYield(id entity.EntityID, sk skill.ID, qty int) int {
	pct := g.skillPct(id, sk, 4, 60, 140)
	if pct == 100 || qty <= 0 {
		return qty
	}
	return max(1, qty*pct/100)
}

// butcherYield scales a butchering yield (meat, leather) by Cooking and
// the difficulty's butcher yield.
func (g *Game) butcherYield(id entity.EntityID, qty int) int {
	return g.skillYield(id, skill.Cooking, g.diffYield(DiffButcher, qty))
}

// tameChance is a tame attempt's chance: ×(100 + 6·(L−7))% for Animals
// and ×(100 + 2·(L−7))% for Social (identity for a neutral pawn).
func (g *Game) tameChance(id entity.EntityID, chance int) int {
	a, s := g.skillPct(id, skill.Animals, 6, 0, 1000), g.skillPct(id, skill.Social, 2, 0, 1000)
	if a == 100 && s == 100 {
		return chance
	}
	return chance * a / 100 * s / 100
}

// productYield scales a milk/wool/egg yield ±3% per Animals level.
func (g *Game) productYield(id entity.EntityID, qty int) int {
	pct := g.skillPct(id, skill.Animals, 3, 0, 1000)
	if pct == 100 || qty <= 0 {
		return qty
	}
	return max(1, qty*pct/100)
}

// meetsMinSkill reports whether id may take a bill needing level min of
// sk (a recipe's MinSkill; ReasonSkill when refused as an order).
func (g *Game) meetsMinSkill(id entity.EntityID, sk skill.ID, min uint8) bool {
	return min == 0 || g.skillOf(id, sk).Level >= min
}

// ReasonSkill refuses work the colonist is not skilled enough for.
const ReasonSkill = "not skilled enough"

// fumbled reports a failed harvest: below Plants 4 a harvest fails
// (4−L)·6% of the time. Neutral pawns never roll (no rng use).
func (g *Game) fumbled(id entity.EntityID) bool {
	l := int(g.skillOf(id, skill.Plants).Level)
	if l >= fumbleBelow {
		return false
	}
	return g.rng.IntN(100) < (fumbleBelow-l)*6
}

// meleeOf is id's melee numbers for the wiki's hit and dodge curves
// (CBT-2): its Melee level (a creature has none), Sight, Manipulation and
// Moving, the MeleeAcc stat as a points offset after the curve (original
// tuning), downed. The game has no Brawler or Nimble trait yet.
func (g *Game) meleeOf(id entity.EntityID) attack.Melee {
	k := keyOf(g.bodies[id]) // body.go: Moving, Manipulation, Sight (healthy without a body)
	m := attack.Melee{Moving: int(k[0]), Manip: int(k[1]), Sight: int(k[2])}
	if v, ok := g.vital.Get(id); ok {
		m.Skill, m.Skilled, m.Offset, m.Down = int(g.skillOf(id, skill.Melee).Level), true, 10*int(g.statOf(id, stat.MeleeAcc)), v.Status != health.Alive
	}
	return m
}

// aimTicksFor scales an aim or cooldown time by the AimCooldown stat.
func (g *Game) aimTicksFor(id entity.EntityID, ticks int) int {
	if r := g.statOf(id, stat.AimCooldown); r != 1000 {
		return max(1, ticks*int(r)/1000)
	}
	return ticks
}

// paceStep scales a step time by MoveSpeed (time ÷ speed).
func (g *Game) paceStep(id entity.EntityID, ticks int) int {
	if r := g.statOf(id, stat.MoveSpeed); r != 1000 && r > 0 {
		return max(1, (ticks*1000+int(r)-1)/int(r))
	}
	return ticks
}

// lovesWork is the passion of the skill id is working with right now
// (NoPassion when idle or not on skilled work).
func (g *Game) lovesWork(id entity.EntityID) skill.Passion {
	w, ok := g.sk.working[id]
	if !ok || uint64(g.clock.Now())-w.tick > workingTicks {
		return skill.NoPassion
	}
	if j, _ := g.jobs.Get(id); !j.Active() || j.Kind == job.Sleep || j.Kind == job.Eat {
		return skill.NoPassion
	}
	return g.skillOf(id, w.sk).Passion
}

// traitThoughts applies id's traits and the difficulty to its thoughts:
// thought scales and additions, the passion state thought, the trait mood
// offset and the difficulty's mood line (hidden when 0).
func (g *Game) traitThoughts(id entity.EntityID, ts []mood.Thought) []mood.Thought {
	if fx := g.sk.stats[id]; fx != nil && len(fx.traits) > 0 {
		for i := range ts {
			for _, t := range fx.traits {
				d := trait.DefOf(t)
				if sc, ok := d.ThoughtScale[ts[i].Kind]; ok {
					ts[i].Value = ts[i].Value * int(sc) / 100
				}
				if add, ok := d.AddThought[ts[i].Kind]; ok {
					ts[i].Value += int(add)
				}
			}
		}
		for _, t := range fx.traits {
			for _, m := range trait.DefOf(t).Mods {
				if m.Stat == stat.MoodOffset && m.Add != 0 {
					ts = append(ts, mood.Thought{Kind: mood.Difficulty, Label: strings.ToLower(trait.DefOf(t).Label), Value: int(m.Add)})
				}
			}
		}
	}
	switch g.lovesWork(id) {
	case skill.Minor:
		ts = append(ts, mood.Thought{Kind: mood.LovesWork, Label: mood.DefOf(mood.LovesWork).Label, Value: mood.DefOf(mood.LovesWork).Value})
	case skill.Major:
		ts = append(ts, mood.Thought{Kind: mood.LovesWork, Label: mood.DefOf(mood.LovesWork).Label, Value: lovesWorkMajor})
	}
	if off := g.moodOffset(); off != 0 {
		ts = append(ts, mood.Thought{Kind: mood.Difficulty, Label: mood.DefOf(mood.Difficulty).Label, Value: off})
	}
	return ts
}

// comfortShift is id's ColdComfort and HeatComfort (tenths of a degree
// added to the comfort floor and ceiling).
func (g *Game) comfortShift(id entity.EntityID) (cold, heat int) {
	return int(g.statOf(id, stat.ColdComfort)), int(g.statOf(id, stat.HeatComfort))
}

// breakOffset is the trait shift of id's break threshold (Steady −6:
// breaks later; Brittle +6).
func (g *Game) breakOffset(id entity.EntityID) int { return int(g.statOf(id, stat.BreakThreshold)) }

// decayNeeds is one awake tick of hunger and rest loss at id's rates
// (needs.Decay itself for neutral rates).
func (g *Game) decayNeeds(id entity.EntityID, n needs.Pair, now uint64) needs.Pair {
	hr, rr := g.statOf(id, stat.HungerRate), g.statOf(id, stat.RestRate)
	hr, rr = g.diseaseNeedRates(id, hr, rr) // disease.go (MED-04): gut worms, muscle parasites
	if hr == 1000 && rr == 1000 {
		return needs.Decay(n, now)
	}
	c := g.needCarryOf(id)
	if now%uint64(needs.HungerPeriod) == 0 {
		c.hunger += hr
		for ; c.hunger >= 1000; c.hunger -= 1000 {
			n.Hunger = max(n.Hunger-1, 0)
		}
	}
	if now%uint64(needs.RestAwakePeriod) == 0 {
		c.rest += rr
		for ; c.rest >= 1000; c.rest -= 1000 {
			n.Rest = max(n.Rest-1, 0)
		}
	}
	return n
}

// drainJoy is one awake tick of recreation drain: RecreationRate, less
// 20%/40% while doing minor/major passion work.
func (g *Game) drainJoy(id entity.EntityID, now uint64) {
	if now%recreation.DrainPeriod != 0 || g.joy[id] <= 0 {
		return
	}
	r := g.statOf(id, stat.RecreationRate)
	switch g.lovesWork(id) {
	case skill.Minor:
		r = r * 800 / 1000
	case skill.Major:
		r = r * 600 / 1000
	}
	if r == 1000 {
		g.joy[id]--
		return
	}
	c := g.needCarryOf(id)
	for c.joy += r; c.joy >= 1000 && g.joy[id] > 0; c.joy -= 1000 {
		g.joy[id]--
	}
}

func (g *Game) needCarryOf(id entity.EntityID) *needCarry {
	g.sk.ensure()
	c := g.sk.carry[id]
	if c == nil {
		c = &needCarry{}
		g.sk.carry[id] = c
	}
	return c
}

// researchPoints converts units of research work to project points: ×
// the difficulty's research speed ÷ the scenario's research cost, with
// the fraction carried (exactly units at 100%/100%).
func (g *Game) researchPoints(units int) int {
	cost := g.researchCostPct()
	scaled := g.diffScale(DiffResearch, units*10000) / max(cost, 1) // hundredths of a point
	if scaled == units*100 && g.sk.research == 0 {
		return units
	}
	total := g.sk.research + scaled
	g.sk.research = total % 100
	return total / 100
}

// --- incapability ----------------------------------------------------

// incapableToast tells the player a work type can't be enabled.
func (g *Game) incapableToast(id entity.EntityID, t work.Type) {
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "Incapable: %s", g.incapableWhy(id, t))
}

// ReasonIncapableOf prefixes a forced order refused by an incapability.
const ReasonIncapableOf = "Incapable: "

// orderWork is the work type each forced order needs.
var orderWork = map[command.Order]work.Type{
	command.OrderBuild:       work.Construct,
	command.OrderDeconstruct: work.Construct,
	command.OrderSmooth:      work.Construct,
	command.OrderFlick:       work.Construct, // PLAY-021: flicking is construction work
	command.OrderSupply:      work.Haul,
	command.OrderHaul:        work.Haul,
	command.OrderHarvest:     work.Grow,
	command.OrderSow:         work.Grow,
	command.OrderCut:         work.Grow,
	command.OrderMine:        work.Mine,
	command.OrderHunt:        work.Hunt,
}

// incapableOrder is the refusal of a forced order id can never do, ""
// when it may. Attack needs violence like Hunt.
func (g *Game) incapableOrder(id entity.EntityID, o command.Order) string {
	if g.incapableOf(id) == 0 {
		return ""
	}
	t, ok := orderWork[o]
	if o == command.OrderAttack {
		t, ok = work.Hunt, true
	}
	if ok && g.incapableWork(id).Enabled(t) {
		return ReasonIncapableOf + g.incapableWhy(id, t)
	}
	return ""
}

// --- read models -----------------------------------------------------

// fillPawnBio fills a pawn view's identity: sex, build, age, look,
// incapable work, skill levels and passions, title. Pawns without a bio
// (raiders SETUP did not generate) read as legacy pawns.
func (g *Game) fillPawnBio(pv *PawnView) {
	p, ok := g.sk.bios[pv.ID]
	if !ok {
		p = legacyBio(pv.ID, pv.Name)
	}
	pv.Sex, pv.Build, pv.AgeYears = sexLabel(p.Look.Sex), buildLabel(p.Look.Build), p.AgeYears()
	pv.Look, pv.Title = p.Look, p.Title()
	pv.Incapable = p.Incapable().WorkMask()
	for sk := skill.ID(0); sk < skill.NumSkills; sk++ {
		s := g.skillOf(pv.ID, sk)
		if !ok {
			s = skill.Skill{Level: skill.Neutral}
		}
		pv.SkillLv[sk], pv.Passion[sk] = s.Level, uint8(s.Passion)
	}
}

// bioView is the Bio tab model of id: its identity with the live skills.
func (g *Game) bioView(id entity.EntityID) (BioView, bool) {
	p, ok := g.sk.bios[id]
	if !ok {
		return BioView{}, false
	}
	if set := g.sk.skills[id]; set != nil {
		p.Skills = *set
	}
	return bioOf(p), true
}

// --- saves ------------------------------------------------------------
// save.go (lead) carries these as saveState.Bios, .Skills and .WorkCarry.

// exportBios is the pawn identity state a save carries: every bio, the
// live skills and the fractional work.
func (g *Game) exportBios() (bios map[entity.EntityID]pawngen.Pawn, skills map[entity.EntityID]skill.Set, carry map[entity.EntityID]int32) {
	if len(g.sk.bios) == 0 {
		return nil, nil, nil
	}
	bios = make(map[entity.EntityID]pawngen.Pawn, len(g.sk.bios))
	skills = make(map[entity.EntityID]skill.Set, len(g.sk.skills))
	for id, p := range g.sk.bios {
		bios[id] = p
	}
	for id, set := range g.sk.skills {
		skills[id] = *set
	}
	if len(g.sk.workCarry) > 0 {
		carry = make(map[entity.EntityID]int32, len(g.sk.workCarry))
		for id, c := range g.sk.workCarry {
			carry[id] = c
		}
	}
	return bios, skills, carry
}

// importBios restores exportBios' state and migrates older saves: every
// colonist without a bio (a v1 save) gets its legacy identity, so its
// look and temperament are unchanged. Call it once the colonists' names
// and the Setup (difficulty) are restored.
func (g *Game) importBios(bios map[entity.EntityID]pawngen.Pawn, skills map[entity.EntityID]skill.Set, carry map[entity.EntityID]int32) {
	g.sk.ensure()
	for id, p := range bios {
		g.giveBio(id, p)
	}
	for id, set := range skills {
		if cur := g.sk.skills[id]; cur != nil {
			*cur = set
		}
	}
	for id, c := range carry {
		g.sk.workCarry[id] = c
	}
	for _, id := range g.pawnIDs {
		if _, ok := g.sk.bios[id]; !ok {
			n, _ := g.names.Get(id)
			g.giveBio(id, legacyBio(id, n))
		}
	}
}

// --- difficulty and scenario hooks ----------------------------------
// SETUP's difficulty and scenario state (setup.go): a legacy colony gets
// every factor at 100%.

// diffYield scales a yield by difficulty field f (DiffCrop, DiffMine,
// DiffButcher); positive yields stay at least 1.
func (g *Game) diffYield(f, qty int) int {
	v := g.diffScale(f, qty)
	if qty > 0 {
		v = max(v, 1)
	}
	return v
}

// diffPainTolerance is the difficulty's pain tolerance offset, percent
// (0 for a legacy colony).
func (g *Game) diffPainTolerance() int {
	if !g.hasSetup() {
		return 0
	}
	return g.diff().PainTolerance
}

// Skill aliases for the work sites in other files (no extra imports
// there).
const (
	skShooting     = skill.Shooting
	skMelee        = skill.Melee
	skConstruction = skill.Construction
	skMining       = skill.Mining
	skCooking      = skill.Cooking
	skPlants       = skill.Plants
	skAnimals      = skill.Animals
	skMedicine     = skill.Medicine
	skSocial       = skill.Social
	skIntellectual = skill.Intellectual
	skNone         = skill.NumSkills // work speed only (hasSkill false)
)
