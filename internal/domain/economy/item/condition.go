package item

import "rimworld-go/internal/domain/economy/quality"

// Item condition (PRD-2): made things (apparel and weapons) have a
// quality, hit points and, when stuffed, a material. A stack keeps the
// hit points it lost in Stack.Wear, so 0 is pristine and older saves load
// as new. Raw resources and food have no condition: food rots instead,
// and the wiki gives no per-item deterioration rate for the raw
// resources, so they keep forever as before.

// HasQuality reports a kind made with a quality roll and with hit points:
// apparel and weapons (wiki: Quality; furniture and art are buildings),
// but not grenades, thrown from a stack.
func HasQuality(k Kind) bool {
	if k == FragGrenades || k == MolotovCocktails || k == EMPGrenades {
		return false
	}
	return k.IsApparel() || k.IsWeapon()
}

// maxHP is the base hit points per kind. wiki: Apparel (parka 140, tuque
// 80, duster 200); the wool parka is our own garment and takes the
// parka's; flak vest 200, flak jacket 260, plate armor 290, marine armor
// 340. The wiki publishes no hit points for the weapons and the rest of
// the armor here: 100, the common value (original tuning).
var maxHP = [numKinds]int{Parka: 140, Tuque: 80, Duster: 200, WoolParka: 140,
	Club: 100, Knife: 100, Spear: 100, Rifle: 100,
	FlakVest: 200, FlakJacket: 260, PlateArmor: 290, MarineArmor: 340}

// defaultMaxHP is a quality kind's hit points when the table has no row
// (kinds added later).
const defaultMaxHP = 100

// MaxHP is kind k's base hit points (0: it has no condition).
func MaxHP(k Kind) int {
	if !HasQuality(k) {
		return 0
	}
	if k < numKinds && maxHP[k] > 0 {
		return maxHP[k]
	}
	return defaultMaxHP
}

// MaxHP is the stack's hit points when new: its kind's, scaled by its
// material's (a wooden club has 65; wiki: Materials).
func (s Stack) MaxHP() int {
	hp := MaxHP(s.Kind)
	if hp == 0 {
		return 0
	}
	if p, ok := StuffOf(s.StuffOfStack()); ok && p.MaxHP > 0 {
		hp = max(1, hp*p.MaxHP/1000)
	}
	return hp
}

// HP is the stack's hit points left (0 for a kind with no condition).
func (s Stack) HP() int { return max(0, s.MaxHP()-s.Wear) }

// WornOut reports a stack with a condition and no hit points left.
func (s Stack) WornOut() bool { return s.MaxHP() > 0 && s.Wear >= s.MaxHP() }

// Tainted reports apparel that was on a pawn when it died: Of names the
// dead wearer (Of is otherwise only a corpse's or its bones').
func (s Stack) Tainted() bool { return s.Kind.IsApparel() && s.Of != 0 }

// QualityLevel is the stack's quality (ok false: none, not made).
func (s Stack) QualityLevel() (quality.Level, bool) { return quality.FromStored(s.Quality) }

// Wear stages of apparel (wiki: Apparel): ratty at or below RattyPct of
// its hit points, tattered below TatteredPct. The wiki's Deterioration
// page says 25 % for tattered; the Apparel page's 20 % is used.
const (
	RattyPct    = 50
	TatteredPct = 20
)

// Stage is how worn a garment looks.
type Stage uint8

const (
	Fine Stage = iota
	Ratty
	Tattered
)

// StageOf is the wear stage of a garment with hp of maxHP left.
func StageOf(hp, maxHP int) Stage {
	switch {
	case maxHP <= 0:
		return Fine
	case hp*100 < TatteredPct*maxHP:
		return Tattered
	case hp*100 <= RattyPct*maxHP:
		return Ratty
	}
	return Fine
}

// Deterioration (wiki: Deterioration): things left under the open sky
// lose hit points every day, five times as fast in rain and three times
// on water or marsh; nothing deteriorates under a roof. Metals and stone
// never do. The wiki publishes no base rate for apparel and weapons:
// DeteriorationPerDay is original tuning. Worn apparel instead loses one
// hit point on WornWearChance permille of days (wiki: Apparel).
const (
	DeteriorationPerDay = 2000 // milli-HP per day outdoors, dry, Normal quality
	RainFactor          = 5
	WetFactor           = 3
	WornWearChance      = 400
)

// DeteriorationRate is how fast stack s, lying under the open sky, loses
// hit points, in milli-HP per day: 0 for a kind with no condition or an
// immune material; quality scales it (wiki: Quality, deterioration rate).
func DeteriorationRate(s Stack, rain, wet bool) int {
	if s.MaxHP() == 0 {
		return 0
	}
	if p, ok := StuffOf(s.StuffOfStack()); ok && p.Immune {
		return 0
	}
	r := DeteriorationPerDay
	if l, ok := s.QualityLevel(); ok {
		r = quality.Apply(r, l, quality.Deterioration)
	}
	if rain {
		r *= RainFactor
	}
	if wet {
		r *= WetFactor
	}
	return r
}

// HPValuePct is the share (permille) of its value a thing keeps at hp of
// maxHP (wiki: Deterioration): all of it down to 90 %, then 1.67 points
// per point to 60 % (50 % left), 4 per point to 50 % (10 % left), and
// 0.2 per point to nothing.
func HPValuePct(hp, maxHP int) int {
	if maxHP <= 0 || hp >= maxHP {
		return 1000
	}
	pm := max(0, hp) * 1000 / maxHP // hit points, permille
	switch {
	case pm >= 900:
		return 1000
	case pm >= 600:
		return 500 + (pm-600)*500/300
	case pm >= 500:
		return 100 + (pm-500)*400/100
	}
	return pm * 100 / 500
}

// StackValue is the market value of one unit of s: its kind's value,
// scaled by its material against the default one, its quality and its
// hit points.
func StackValue(s Stack) int {
	v := s.Kind.Value()
	if MadeFrom(s.Kind) != 0 {
		v = v * StuffFactor(s.Kind, s.StuffOfStack(), func(p StuffProps) int { return p.Value }) / 1000
	}
	if l, ok := s.QualityLevel(); ok {
		v = quality.Apply(v, l, quality.MarketValue)
	}
	return v * HPValuePct(s.HP(), s.MaxHP()) / 1000
}

// Insulation is garment k's cold and heat insulation (tenths of a
// degree) made of material m at quality stored q (0: none, read as
// Normal): the kind's numbers scaled by the material's insulation against
// the default material's, then by quality (wiki: Quality, insulation).
func Insulation(k, m Kind, q uint8) (cold, heat int) {
	cold, heat = k.Insulation()
	if MadeFrom(k) != 0 && m != 0 {
		cold = cold * StuffFactor(k, m, func(p StuffProps) int { return p.ColdInsulation }) / 1000
		heat = heat * StuffFactor(k, m, func(p StuffProps) int { return p.HeatInsulation }) / 1000
	}
	if l, ok := quality.FromStored(q); ok {
		if cold > 0 {
			cold = quality.Apply(cold, l, quality.Insulation)
		}
		if heat > 0 { // a garment hot to wear gets no hotter for being good
			heat = quality.Apply(heat, l, quality.Insulation)
		}
	}
	return cold, heat
}
