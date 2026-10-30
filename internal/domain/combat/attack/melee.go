package attack

// Melee is what a fighter's melee hit and dodge chances are read from
// (CBT-2): its Melee skill (Skilled false for creatures with none:
// animals and mechanoids), its Sight, Manipulation and Moving capacities
// (permille, 1000 healthy), the traits the wiki names, a stat offset
// (permille, added after the curve) and whether it lies downed (a downed
// fighter never dodges).
type Melee struct {
	Skill                int
	Skilled              bool
	Sight, Manip, Moving Permille
	Brawler, Nimble      bool
	Offset               Permille
	Down                 bool
}

// Fresh is a healthy fighter with melee skill level skill (Skilled).
func Fresh(skill int) Melee {
	return Melee{Skill: skill, Skilled: true, Sight: one, Manip: one, Moving: one}
}

// The melee hit score's parts, in thousandths of a point. wiki: Melee hit
// chance
const (
	// unskilledHit stands in for the skill of a creature with none.
	unskilledHit = 4 * one
	// capacityHit is the score per whole capacity above or below 100 %,
	// counted up to capacityHitCap.
	capacityHit    = 12
	capacityHitCap = 1500
	brawlerHit     = 4 * one
)

// HitScore is m's melee hit base score in thousandths of a point: its
// skill (4 with none) + 12 x (min(sight, 150 %) - 100 %) + the same for
// manipulation + 4 for Brawler. wiki: Melee hit chance
func HitScore(m Melee) int {
	s := unskilledHit
	if m.Skilled {
		s = m.Skill * one
	}
	s += capacityHit*(min(m.Sight, capacityHitCap)-one) + capacityHit*(min(m.Manip, capacityHitCap)-one)
	if m.Brawler {
		s += brawlerHit
	}
	return s
}

// hitCurve is the wiki's post-processing of the hit score (points) into
// a chance (permille); the published table is linear between these
// points and flat outside them. wiki: Melee hit chance
var hitCurve = [...]struct{ score, p int }{
	{-20, 50}, {-10, 100}, {0, 500}, {10, 800}, {20, 900}, {40, 960}, {60, 980},
}

// HitChanceOf is m's melee hit chance (permille): the curve of its score,
// plus its offset, clamped to [0, 1000]. A skill-20 fighter hits 90 % of
// the time, a skill-less creature 62 %. wiki: Melee hit chance
func HitChanceOf(m Melee) Permille {
	return clamp(curve(HitScore(m), hitCurve[:]) + m.Offset)
}

// curve interpolates the points pts (score in points, ascending) at score
// s (thousandths of a point), truncating; flat outside.
func curve(s int, pts []struct{ score, p int }) int {
	if s <= pts[0].score*one {
		return pts[0].p
	}
	for i := 1; i < len(pts); i++ {
		if b := pts[i]; s <= b.score*one {
			a := pts[i-1]
			return a.p + (b.p-a.p)*(s-a.score*one)/((b.score-a.score)*one)
		}
	}
	return pts[len(pts)-1].p
}

// The melee dodge score's parts, in thousandths of a point (skill counts
// a point a level). wiki: Melee dodge chance
const (
	movingDodge    = 18 // per whole Moving above or below 100 %, uncapped
	sightDodge     = 8  // per whole Sight, counted up to sightDodgeCap
	sightDodgeCap  = 1400
	nimbleDodge    = 15 * one
	dodgeFloor     = 5 // no dodge up to this score
	dodgeKnee      = 20
	dodgeTop       = 60
	dodgeMax       = 500
	dodgeKneeValue = 300
)

// DodgeScore is m's melee dodge base score in thousandths of a point:
// melee skill + 18 x (moving - 100 %) + 8 x (min(sight, 140 %) - 100 %)
// + 15 for Nimble. A creature with no skill counts 0 for it (the wiki
// gives it no stand-in). wiki: Melee dodge chance
func DodgeScore(m Melee) int {
	s := 0
	if m.Skilled {
		s = m.Skill * one
	}
	s += movingDodge*(m.Moving-one) + sightDodge*(min(m.Sight, sightDodgeCap)-one)
	if m.Nimble {
		s += nimbleDodge
	}
	return s
}

// DodgeChanceOf is the chance (permille) that m dodges a melee blow that
// would have hit: none up to a score of 5, 2 % a point to 20 (30 %), then
// 0.5 % a point to 60 (50 %), and no more; a downed fighter never dodges.
// wiki: Melee dodge chance
func DodgeChanceOf(m Melee) Permille {
	if m.Down {
		return 0
	}
	s := DodgeScore(m)
	switch {
	case s <= dodgeFloor*one:
		return 0
	case s <= dodgeKnee*one:
		return (s - dodgeFloor*one) * 20 / one
	case s <= dodgeTop*one:
		return dodgeKneeValue + (s-dodgeKnee*one)*5/one
	}
	return dodgeMax
}

// Melee damage varies by a random factor. wiki: Combat (melee damage
// x0.8..1.2)
const (
	MeleeDamageMin = 800
	MeleeDamageMax = 1200
)

// MeleeDamage is a melee blow's damage: amount times a factor drawn
// uniformly from [0.8, 1.2] by roll (one draw), rounded, at least 1 for
// a blow of any damage.
func MeleeDamage(amount int, roll func(int) int) int {
	if amount <= 0 {
		return 0
	}
	f := MeleeDamageMin + roll(MeleeDamageMax-MeleeDamageMin+1)
	return max(1, (amount*f+one/2)/one)
}

// Combat experience per attack. original tuning: the wiki publishes no
// Shooting or Melee rates (wiki: Skills, Shooting, Melee); experience is
// earned per second of the attack's cycle, a hit teaching more than a
// miss, so a fast weapon learns no faster than a slow one.
const (
	HitXPPerSecond  = 80
	MissXPPerSecond = 30
)

// CombatXP is the experience one attack teaches: per second of its cycle
// (cycleTicks, at least one tick), more for a hit, shared over a burst's
// shots (burst at least 1).
func CombatXP(hit bool, cycleTicks, burst int) int {
	rate := MissXPPerSecond
	if hit {
		rate = HitXPPerSecond
	}
	return max(1, rate*max(cycleTicks, 1)/60/max(burst, 1))
}
