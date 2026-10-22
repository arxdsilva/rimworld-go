// Package pawngen generates colonists deterministically from a seed: name,
// sex, build, age, backstories, traits, skills, passions, look and
// starting conditions (NG-1 §4.6, §4.7).
package pawngen

import (
	"rimworld-go/internal/domain/pawn/backstory"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/pkg/randx"
)

// Condition is a starting health condition (append only, saved).
type Condition uint8

const (
	NoCondition Condition = iota
	OldFracture
	Nearsighted
	HandTremor
	Exhausted
	numConditions
)

// Temperament mirrors the game's idle temperament (Wanderer, Social,
// Homebody) so the domain can choose it from traits.
type Temperament uint8

const (
	Wanderer Temperament = iota
	SocialType
	Homebody
	numTemperaments
)

// Pawn is a generated colonist's identity. AgeDays is biological age in
// in-game days (60 per year).
type Pawn struct {
	First, Nick, Last    string
	Look                 look.Look
	AgeDays              int32
	Childhood, Adulthood backstory.ID
	Traits               []trait.ID
	Skills               skill.Set
	Conditions           []Condition
	Temperament          Temperament
}

// ForcedTrait makes a trait appear with a chance (percent; 0 or >= 100
// means always).
type ForcedTrait struct {
	Trait  trait.ID
	Chance uint8
}

// Rules are a scenario's constraints on generation.
type Rules struct {
	MinAge, MaxAge int // years; 0 = defaults (14..68)
	Forced         []ForcedTrait
	ForcedCond     Condition
	CondChance     uint8 // chance of ForcedCond in percent; 0 = always
	Tech           backstory.Tag
	NoClothes      bool
	Taken          map[string]bool // nicknames already used
}

// DaysPerYear converts AgeDays to years.
const DaysPerYear = 60

// Generation constants (NG-1 §4.6).
const (
	defaultMinAge = 14
	defaultMaxAge = 68
	adultAge      = 18
	ageMode       = 30
	teenPct       = 8  // share of 14-17 year olds
	condPct       = 12 // chance of a random starting condition
	nickPct       = 35 // chance of a nickname instead of the first name
	neolithicPct  = 10 // chance a non-clan colonist draws a clan backstory
	majorPct      = 30 // share of passion picks that are major
	beardPct      = 45 // men with a beard
	vestPct       = 40
	legacyAge     = 30
	maxNameTries  = 64
)

// Salts for the independent random streams of one candidate.
const (
	bodySalt = 0x50574e47454e4231 // body, story, skills, look
	nameSalt = 0x50574e47454e4e31 // names (kept apart so Taken can't shift the rest)
)

// mix64 is the splitmix64 finaliser.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xBF58476D1CE4E5B9
	x ^= x >> 27
	x *= 0x94D049BB133111EB
	x ^= x >> 31
	return x
}

func streamSeed(seed uint64, slot, roll int, salt uint64) uint64 {
	s := mix64(seed ^ salt)
	s = mix64(s ^ uint64(int64(slot))*0x9E3779B97F4A7C15)
	return mix64(s ^ uint64(int64(roll))*0xD1B54A32D192ED03)
}

// Generate makes candidate slot's colonist on its roll-th reroll. The
// result depends only on (seed, slot, roll, r).
func Generate(seed uint64, slot, roll int, r Rules) Pawn {
	rng := randx.New(streamSeed(seed, slot, roll, bodySalt))
	var p Pawn
	p.Look.Sex = look.Sex(rng.IntN(2))
	p.Look.Build = rollBuild(rng)
	years := rollAge(rng, r)
	p.AgeDays = int32(years*DaysPerYear + rng.IntN(DaysPerYear))

	// Forced traits first, so backstories are chosen around them.
	for _, f := range r.Forced {
		hit := rng.IntN(100) < int(f.Chance) || f.Chance == 0 || f.Chance >= 100
		if hit && f.Trait > trait.None && f.Trait < trait.NumTraits && !conflictsAny(f.Trait, p.Traits) {
			p.Traits = append(p.Traits, f.Trait)
		}
	}
	p.Childhood = pickBackstory(rng, backstory.Childhood, r.Tech, p.Traits)
	if years >= adultAge {
		p.Adulthood = pickBackstory(rng, backstory.Adulthood, r.Tech, p.Traits)
	}
	p.Traits = rollTraits(rng, p.Traits, p.Childhood, p.Adulthood)
	p.Skills = rollSkills(rng, p, years)
	p.Conditions = rollConditions(rng, r)
	p.Look = rollLook(rng, p.Look, years, r.NoClothes)

	p.First, p.Nick, p.Last = rollName(randx.New(streamSeed(seed, slot, roll, nameSalt)), p.Look.Sex, r.Taken)
	p.Temperament = temperamentFor(p)
	return p
}

func rollBuild(rng *randx.Rand) look.Build {
	switch x := rng.IntN(100); {
	case x < 25:
		return look.Thin
	case x < 75:
		return look.Average
	default:
		return look.Heavy
	}
}

// rollAge: teenPct% are 14-17, the rest 18-68 triangular with mode 30,
// both ranges narrowed by the rules.
func rollAge(rng *randx.Rand, r Rules) int {
	lo, hi := defaultMinAge, defaultMaxAge
	if r.MinAge > 0 {
		lo = max(r.MinAge, defaultMinAge)
	}
	if r.MaxAge > 0 {
		hi = r.MaxAge
	}
	hi = max(hi, lo)
	teenLo, teenHi := lo, min(hi, adultAge-1)
	adLo, adHi := max(lo, adultAge), hi
	hasTeen, hasAdult := teenLo <= teenHi, adLo <= adHi
	x := rng.IntN(100)
	if hasTeen && (!hasAdult || x < teenPct) {
		return teenLo + rng.IntN(teenHi-teenLo+1)
	}
	mode := min(max(ageMode, adLo), adHi)
	// Integer triangular weights: both sides peak at (mode-lo+1)*(hi-mode+1).
	total := 0
	w := func(a int) int {
		if a <= mode {
			return (a - adLo + 1) * (adHi - mode + 1)
		}
		return (adHi - a + 1) * (mode - adLo + 1)
	}
	for a := adLo; a <= adHi; a++ {
		total += w(a)
	}
	pick := rng.IntN(total)
	for a := adLo; a <= adHi; a++ {
		if pick -= w(a); pick < 0 {
			return a
		}
	}
	return adHi
}

func conflictsAny(t trait.ID, have []trait.ID) bool {
	for _, h := range have {
		if trait.Conflicts(t, h) {
			return true
		}
	}
	return false
}

func clashesAny(traits []trait.ID, d backstory.Def) bool {
	for _, t := range traits {
		if trait.Clashes(t, d) {
			return true
		}
	}
	return false
}

// techTags is the tag set a backstory is drawn from: clans (neolithic
// only) always draw neolithic; everyone else draws their tags and, 10% of
// the time, a neolithic past.
func techTags(rng *randx.Rand, tech backstory.Tag) backstory.Tag {
	if tech == 0 {
		tech = backstory.TagIndustrial | backstory.TagSpacer
	}
	modern := tech &^ backstory.TagNeolithic
	if modern == 0 || rng.IntN(100) < neolithicPct {
		return backstory.TagNeolithic
	}
	return modern
}

func pickBackstory(rng *randx.Rand, slot backstory.Slot, tech backstory.Tag, traits []trait.ID) backstory.ID {
	tags := techTags(rng, tech)
	all := backstory.All()
	var cands []backstory.Def
	for _, strict := range []bool{true, false} {
		for _, d := range all {
			if d.Slot == slot && d.Fits(tags) && (!strict || !clashesAny(traits, d)) {
				cands = append(cands, d)
			}
		}
		if len(cands) > 0 {
			break
		}
	}
	total := 0
	for _, d := range cands {
		total += max(d.Weight, 1)
	}
	if total == 0 {
		return 0
	}
	x := rng.IntN(total)
	for _, d := range cands {
		if x -= max(d.Weight, 1); x < 0 {
			return d.ID
		}
	}
	return cands[len(cands)-1].ID
}

// traitWeight: Gentle is rarer than the rest.
func traitWeight(t trait.ID) int {
	if t == trait.Gentle {
		return 4
	}
	return 10
}

// rollTraits tops forced up to 1 (30%), 2 (50%) or 3 (20%) traits that
// neither conflict with each other nor clash with the backstories.
func rollTraits(rng *randx.Rand, forced []trait.ID, child, adult backstory.ID) []trait.ID {
	out := append([]trait.ID(nil), forced...)
	n := 1
	switch x := rng.IntN(100); {
	case x >= 80:
		n = 3
	case x >= 30:
		n = 2
	}
	cb, ab := backstory.DefOf(child), backstory.DefOf(adult)
	for len(out) < n {
		total := 0
		var cands []trait.ID
		for t := trait.ID(1); t < trait.NumTraits; t++ {
			if conflictsAny(t, out) || trait.Clashes(t, cb) || (adult != 0 && trait.Clashes(t, ab)) {
				continue
			}
			cands = append(cands, t)
			total += traitWeight(t)
		}
		if total == 0 {
			break
		}
		x := rng.IntN(total)
		for _, t := range cands {
			if x -= traitWeight(t); x < 0 {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

func rollSkills(rng *randx.Rand, p Pawn, years int) skill.Set {
	cb, ab := backstory.DefOf(p.Childhood), backstory.DefOf(p.Adulthood)
	inc := p.Incapable()
	var lv [skill.NumSkills]int
	for i := range lv {
		id := skill.ID(i)
		lv[i] = rng.IntN(4) + int(cb.Skills[id]) + int(ab.Skills[id])
		if years < adultAge {
			lv[i] = lv[i] * 6 / 10
		}
	}
	var capable []skill.ID
	for id := skill.ID(0); id < skill.NumSkills; id++ {
		if !inc.DisablesSkill(id) {
			capable = append(capable, id)
		}
	}
	if years >= adultAge && len(capable) > 0 {
		for range min(8, (years-adultAge)/6) {
			lv[capable[rng.IntN(len(capable))]]++
		}
	}
	for _, t := range p.Traits {
		for id, v := range trait.DefOf(t).SkillBonus {
			lv[id] += int(v)
		}
	}
	var s skill.Set
	for i := range s {
		id := skill.ID(i)
		if inc.DisablesSkill(id) {
			continue
		}
		s[i].Level = uint8(min(max(lv[i], 0), skill.MaxLevel))
		if s[i].Level < skill.MaxLevel {
			s[i].XP = int32(rng.IntN(int(skill.Cost(s[i].Level))))
		}
	}
	// Passions: 1-4 picks weighted by level+2 over capable skills.
	for range 1 + rng.IntN(4) {
		total := 0
		for _, id := range capable {
			if s[id].Passion == skill.NoPassion {
				total += int(s[id].Level) + 2
			}
		}
		if total == 0 {
			break
		}
		x := rng.IntN(total)
		for _, id := range capable {
			if s[id].Passion != skill.NoPassion {
				continue
			}
			if x -= int(s[id].Level) + 2; x < 0 {
				s[id].Passion = skill.Minor
				if rng.IntN(100) < majorPct {
					s[id].Passion = skill.Major
				}
				break
			}
		}
	}
	return s
}

func rollConditions(rng *randx.Rand, r Rules) []Condition {
	if r.ForcedCond > NoCondition && r.ForcedCond < numConditions {
		if r.CondChance == 0 || rng.IntN(100) < int(r.CondChance) {
			return []Condition{r.ForcedCond}
		}
	}
	if rng.IntN(100) < condPct {
		return []Condition{Condition(1 + rng.IntN(int(numConditions)-1))}
	}
	return nil
}

// Hair styles by sex (indices into look.HairStyles).
var (
	maleStyles   = []uint8{0, 1, 3, 5, 6, 7}
	femaleStyles = []uint8{0, 1, 2, 3, 4, 6, 7}
)

func rollLook(rng *randx.Rand, l look.Look, years int, bare bool) look.Look {
	l.Skin = uint8(rng.IntN(look.SkinCount))
	l.Hair = uint8(rng.IntN(look.HairCount))
	styles := maleStyles
	if l.Sex == look.Female {
		styles = femaleStyles
	}
	l.HairStyle = styles[rng.IntN(len(styles))]
	if l.Sex == look.Male && rng.IntN(100) < beardPct {
		l.Beard = uint8(1 + rng.IntN(len(look.Beards)))
	}
	l.Shirt = uint8(rng.IntN(look.ClothCount))
	l.Pants = uint8(rng.IntN(look.ClothCount - 1))
	if l.Pants >= l.Shirt {
		l.Pants++
	}
	if rng.IntN(100) < vestPct {
		l.Vest = true
		l.VestCloth = uint8(rng.IntN(look.ClothCount))
	}
	grey := rng.IntN(100)
	switch {
	case years >= 65:
		l.Grey = grey < 90
	case years >= 50:
		l.Grey = grey < 55
	}
	if bare {
		l.Bare, l.Vest, l.VestCloth = true, false, 0
	}
	return l
}

func rollName(rng *randx.Rand, sex look.Sex, taken map[string]bool) (first, nick, last string) {
	pool := maleNames
	if sex == look.Female {
		pool = femaleNames
	}
	for range maxNameTries {
		first = pool[rng.IntN(len(pool))]
		last = surnames[rng.IntN(len(surnames))]
		nick = first
		if rng.IntN(100) < nickPct {
			nick = nicknames[rng.IntN(len(nicknames))]
		}
		if !taken[nick] {
			return first, nick, last
		}
	}
	// Every draw was taken: fall back to the full name, then a number.
	nick = first + " " + last
	for i := 2; taken[nick]; i++ {
		nick = first + " " + last + " " + itoa(i)
	}
	return first, nick, last
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for ; i > 0; i /= 10 {
		n--
		b[n] = byte('0' + i%10)
	}
	return string(b[n:])
}

func temperamentFor(p Pawn) Temperament {
	for _, t := range p.Traits {
		switch t {
		case trait.Sunny:
			return SocialType
		case trait.Brooding:
			return Homebody
		}
	}
	return TemperamentOfName(p.Nick)
}

// TemperamentOfName is the game's historic name hash (32-bit FNV-1a of
// the name, mod 3): the temperament legacy pawns have always had.
func TemperamentOfName(n string) Temperament {
	h := uint32(2166136261)
	for i := 0; i < len(n); i++ {
		h = (h ^ uint32(n[i])) * 16777619
	}
	return Temperament(h % uint32(numTemperaments))
}

// Legacy is the neutral identity for an existing pawn (old saves,
// SpawnNamedPawn): all skills 7, no traits or incapabilities, age 30,
// male average, the legacy look and today's name-hash temperament.
func Legacy(id uint64, name string, skins, hairs, cloths int) Pawn {
	return Pawn{
		First:       name,
		Nick:        name,
		Look:        look.Legacy(id, skins, hairs, cloths),
		AgeDays:     legacyAge * DaysPerYear,
		Skills:      skill.NeutralSet(),
		Temperament: TemperamentOfName(name),
	}
}

// Incapable is everything p can't do (from backstories and traits).
func (p Pawn) Incapable() backstory.Bits {
	b := backstory.DefOf(p.Childhood).Incapable | backstory.DefOf(p.Adulthood).Incapable
	for _, t := range p.Traits {
		b |= trait.DefOf(t).Incapable
	}
	return b
}

// AgeYears is p's age in whole years.
func (p Pawn) AgeYears() int { return int(p.AgeDays) / DaysPerYear }

// FullName is "First 'Nick' Last", or "First Last" when the nickname is
// the first name (legacy pawns: just the name).
func (p Pawn) FullName() string {
	switch {
	case p.Last == "" && (p.Nick == p.First || p.First == ""):
		return p.Nick
	case p.Nick == p.First || p.Nick == "":
		return p.First + " " + p.Last
	}
	return p.First + " '" + p.Nick + "' " + p.Last
}

// Title is the adulthood title, or the childhood one for the young.
func (p Pawn) Title() string {
	if p.Adulthood != 0 {
		return backstory.DefOf(p.Adulthood).Title
	}
	return backstory.DefOf(p.Childhood).Title
}

// Has reports whether p carries trait t.
func (p Pawn) Has(t trait.ID) bool {
	for _, x := range p.Traits {
		if x == t {
			return true
		}
	}
	return false
}
