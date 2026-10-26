package body

import "rimworld-go/internal/domain/sim/clock"

// Aging (MED-08): chronic conditions that come with age, and the heart
// attack an artery blockage can bring on. The definitions are in
// cond_aging.go; this file says when they start, how they grow and how
// they end.
//
//   - Onset. From its wiki onset age a kind may start, rolled once a day
//     by the game (OnsetPerDay). The yearly chance starts at Base at the
//     onset age and rises by Slope a year up to Cap (original tuning; the
//     wiki's per-age tables are not published as numbers): onsetCurve.
//   - Permanent kinds (bad back, cataract, frail, dementia, hearing loss)
//     never change. A part-bound one ends when its part is replaced or
//     lost: an optic implant over a cataract eye, a cochlear implant over
//     a deaf ear, a spinal brace in place of the spine (DropLostAging).
//   - Artery blockage and carcinoma grow on a closed form from their
//     onset tick (GrowAging), at a rate drawn once from a hash of the
//     condition and a per-pawn salt (AgingRate), so nothing extra is
//     saved and two runs of the same seed agree. Artery blockage kills
//     at full severity; a carcinoma destroys its part (killing when the
//     part is vital) or, in remission, fades away.
//   - A heart attack starts at 400 and moves by −400..+600 every hour;
//     each tend has a chance to ease it by 300. Below 0 it is over, at
//     1000 the heart stops (StepHeartAttack).

// Severities (permille) and rates.
const (
	// chronicSev is the severity a permanent chronic condition holds:
	// it has no severity to speak of, but a condition at 0 is cured.
	chronicSev = 1
	// arteryOnset is a new artery blockage's severity (original tuning:
	// the wiki gives none).
	arteryOnset = 1
	// CarcinomaOnset is a new carcinoma's severity. wiki: Ailments.
	CarcinomaOnset = 300
	// HeartAttackOnset is a new heart attack's severity. wiki: Ailments.
	HeartAttackOnset = 400
	// The growth rates in millionths of severity a day (a thousandth of a
	// permille), scaled by a factor drawn in permille. wiki: Ailments.
	arteryPerDay        = 700   // +0.0007/day
	arteryFactorLo      = 500   // × 0.5 ..
	arteryFactorHi      = 3000  // .. 3
	carcinomaGrowPerDay = 3000  // +0.003/day growing
	carcinomaGrowLo     = 450   // × 0.45 ..
	carcinomaGrowHi     = 1650  // .. 1.65
	carcinomaRemitDay   = -2000 // −0.002/day in remission
	carcinomaRemitLo    = 700   // × 0.7 ..
	carcinomaRemitHi    = 1500  // .. 1.5
	// carcinomaGrowPct and carcinomaStablePct share out a carcinoma's
	// growth mode, the rest is remission (original tuning: 50/25/25).
	carcinomaGrowPct   = 50
	carcinomaStablePct = 25
	// The heart attack's hourly step: −400 + roll(1001) (wiki: Ailments,
	// −40 % .. +60 % per interval; the interval is taken as an hour,
	// inside the wiki's 0.2–4 h). A tend eases it by heartAttackTendCut
	// with a chance of heartAttackTendChance × tend quality (wiki:
	// Ailments, 65 % × quality, 30 %). One tend an hour (original tuning).
	heartAttackStepLo     = -400
	heartAttackStepSpan   = 1001
	heartAttackTendCut    = 300
	heartAttackTendChance = 650
	heartAttackTendEvery  = 1
	// AgingDeath is the severity that kills (artery blockage, heart
	// attack) or destroys the part (carcinoma).
	AgingDeath = 1000
)

// microPerSev is how many millionths make one permille of severity.
const microPerSev = 1000

// heartAttackMTB is the mean days between heart attacks at each artery
// blockage stage. wiki: Ailments.
var heartAttackMTB = [...]int32{300, 200, 100, 60, 30}

// Chronic is when an aging kind may start: from OnsetAge (wiki: Ailments,
// the first birthday it can come on), at Base permille a year, rising by
// Slope a year past the onset age to at most Cap (original tuning).
// Parts are the parts it is rolled on, one roll each (none: a carcinoma,
// on a part picked by coverage).
type Chronic struct {
	Kind             CondKind
	OnsetAge         int
	Base, Slope, Cap int32
	Parts            []Part
	permanent        bool
}

// chronics is every aging kind that starts by itself, in kind order. The
// heart attack is not here: it comes from an artery blockage.
var chronics = []Chronic{
	{Kind: CondBadBack, OnsetAge: 41, Base: 20, Slope: 8, Cap: 250, Parts: []Part{Spine}, permanent: true},          // wiki: Ailments (41)
	{Kind: CondCataract, OnsetAge: 49, Base: 15, Slope: 6, Cap: 200, Parts: []Part{EyeL, EyeR}, permanent: true},    // wiki: Ailments (49)
	{Kind: CondFrail, OnsetAge: 51, Base: 15, Slope: 8, Cap: 250, Parts: []Part{Torso}, permanent: true},            // wiki: Ailments (51)
	{Kind: CondDementia, OnsetAge: 69, Base: 10, Slope: 5, Cap: 120, Parts: []Part{Brain}, permanent: true},         // wiki: Ailments (69)
	{Kind: CondHearingLoss, OnsetAge: 49, Base: 15, Slope: 6, Cap: 200, Parts: []Part{EarL, EarR}, permanent: true}, // wiki: Ailments (49)
	{Kind: CondArteryBlockage, OnsetAge: 21, Base: 4, Slope: 2, Cap: 120, Parts: []Part{Heart}},                     // wiki: Ailments (21)
	{Kind: CondCarcinoma, OnsetAge: 23, Base: 2, Slope: 1, Cap: 40},                                                 // wiki: Ailments (23)
}

// Chronics lists the kinds that start with age (read only).
func Chronics() []Chronic { return chronics }

// ChronicOf is aging kind k's onset (false: k does not start with age).
func ChronicOf(k CondKind) (Chronic, bool) {
	for _, c := range chronics {
		if c.Kind == k {
			return c, true
		}
	}
	return Chronic{}, false
}

// IsAging reports k one of the aging kinds (the heart attack included).
func IsAging(k CondKind) bool {
	switch k {
	case CondBadBack, CondCataract, CondFrail, CondDementia, CondHearingLoss,
		CondArteryBlockage, CondCarcinoma, CondHeartAttack:
		return true
	}
	return false
}

// Permanent reports aging kind k one that never changes by itself.
func Permanent(k CondKind) bool {
	c, ok := ChronicOf(k)
	return ok && c.permanent
}

// OnsetPerYear is the yearly chance (permille) aging kind k starts at age
// years: 0 before its onset age, then Base rising by Slope a year to Cap.
func OnsetPerYear(k CondKind, age int) int32 {
	c, ok := ChronicOf(k)
	if !ok || age < c.OnsetAge {
		return 0
	}
	return min(c.Base+c.Slope*int32(age-c.OnsetAge), c.Cap)
}

// OnsetScale is the scale of OnsetPerDay: a chance out of a million.
const OnsetScale = 1_000_000

// OnsetPerDay is the daily chance, out of OnsetScale, that aging kind k
// starts (on one of its parts) at age years: the yearly chance spread
// over the year's days.
func OnsetPerDay(k CondKind, age int) int32 {
	return OnsetPerYear(k, age) * (OnsetScale / 1000) / clock.DaysPerYear
}

// CanDevelop reports aging kind k able to start on part p: the part is
// there and natural, it has no k yet, and for an artery blockage the
// heart is not artificial. A nil body is a whole one.
func (b *Body) CanDevelop(k CondKind, p Part) bool {
	if !IsAging(k) || p >= NumParts {
		return false
	}
	if b == nil {
		return true
	}
	if b.isMissing(p) || b.condOn(k, p) {
		return false
	}
	switch k {
	case CondArteryBlockage, CondHeartAttack:
		return !b.HeartAttackImmune()
	case CondBadBack:
		return !b.SpineBraced()
	}
	return true
}

// Develop starts aging kind k on part p at tick now, at its onset
// severity; it reports whether it did (CanDevelop).
func (b *Body) Develop(k CondKind, p Part, now uint64) bool {
	if !b.CanDevelop(k, p) {
		return false
	}
	sev := onsetSev(k)
	return b.AddCond(Condition{Kind: k, Part: p, Sev: sev, Since: now, Stage: DefOfCond(k).StageAt(sev)})
}

// HeartAttackMTB is the mean days to the next heart attack from the
// body's artery blockage stage (0: none can come: no blockage, an
// artificial heart, or one under way).
func (b *Body) HeartAttackMTB() int32 {
	if b == nil || b.HeartAttackImmune() {
		return 0
	}
	if _, on := b.Cond(CondHeartAttack); on {
		return 0
	}
	c, ok := b.Cond(CondArteryBlockage)
	if !ok {
		return 0
	}
	return heartAttackMTB[min(int(c.Stage), len(heartAttackMTB)-1)]
}

// Growth is how a carcinoma behaves, fixed when it starts.
type Growth uint8

const (
	Growing Growth = iota
	Stable
	Remission
)

// String is the growth's Health tab word.
func (g Growth) String() string {
	switch g {
	case Stable:
		return "stable"
	case Remission:
		return "in remission"
	}
	return "growing"
}

// mix is a splitmix64 finaliser: a well-spread hash of x.
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}

// agingKey is condition c's own hash under salt (the game's per-pawn
// salt): its kind, part and onset tick.
func agingKey(c Condition, salt uint64) uint64 {
	return mix(salt ^ mix(c.Since^uint64(c.Kind)<<56^uint64(c.Part)<<48))
}

// between maps hash h onto lo..hi.
func between(h uint64, lo, hi int32) int32 {
	return lo + int32(h%uint64(hi-lo+1))
}

// CarcinomaGrowth is carcinoma c's growth under salt: growing half the
// time, stable or in remission a quarter each (original tuning).
func CarcinomaGrowth(c Condition, salt uint64) Growth {
	switch r := agingKey(c, salt) % 100; {
	case r < carcinomaGrowPct:
		return Growing
	case r < carcinomaGrowPct+carcinomaStablePct:
		return Stable
	}
	return Remission
}

// AgingRate is how fast condition c's severity moves, in millionths a
// day (negative recedes; 0 for every kind that does not grow): artery
// blockage +700 × 0.5..3, a growing carcinoma +3000 × 0.45..1.65, one in
// remission −2000 × 0.7..1.5, the factor drawn from c's hash under salt.
func AgingRate(c Condition, salt uint64) int32 {
	h := mix(agingKey(c, salt) ^ 0x5bd1e995)
	switch c.Kind {
	case CondArteryBlockage:
		return arteryPerDay * between(h, arteryFactorLo, arteryFactorHi) / 1000
	case CondCarcinoma:
		switch CarcinomaGrowth(c, salt) {
		case Growing:
			return carcinomaGrowPerDay * between(h, carcinomaGrowLo, carcinomaGrowHi) / 1000
		case Remission:
			return carcinomaRemitDay * between(h, carcinomaRemitLo, carcinomaRemitHi) / 1000
		}
	}
	return 0
}

// onsetSev is the severity aging kind k starts at.
func onsetSev(k CondKind) int32 {
	switch k {
	case CondArteryBlockage:
		return arteryOnset
	case CondCarcinoma:
		return CarcinomaOnset
	case CondHeartAttack:
		return HeartAttackOnset
	}
	return chronicSev
}

// GrowAging sets growing condition c's severity at tick now from its
// onset severity and rate (millionths a day; AgingRate), clamped to
// 0..AgingDeath, and its stage. It reports a change of stage. Severity
// only ever moves one way, so the stage never flaps.
func GrowAging(c *Condition, rate int32, now uint64) bool {
	var el uint64
	if now > c.Since {
		el = now - c.Since
	}
	d := int64(rate) * int64(el) / (int64(clock.TicksPerDay) * microPerSev)
	sev := int32(min(max(int64(onsetSev(c.Kind))+d, 0), AgingDeath))
	stage := DefOfCond(c.Kind).StageAt(sev)
	changed := stage != c.Stage
	c.Sev, c.Stage = sev, stage
	return changed
}

// hasAging reports an aging condition on the body.
func (b *Body) hasAging() bool {
	for _, c := range b.Conds {
		if IsAging(c.Kind) {
			return true
		}
	}
	return false
}

// HasAging reports an aging condition on the body.
func (b *Body) HasAging() bool { return b != nil && b.hasAging() }

// removeCondOn removes kind k's condition on part p (reports one).
func (b *Body) removeCondOn(k CondKind, p Part) bool {
	n := len(b.Conds)
	kept := b.Conds[:0]
	for _, c := range b.Conds {
		if c.Kind != k || c.Part != p {
			kept = append(kept, c)
		}
	}
	clear(b.Conds[len(kept):])
	b.Conds = kept
	if len(b.Conds) == 0 {
		b.Conds = nil
	}
	return len(kept) != n
}

// DropLostAging removes the aging conditions whose part is gone or
// replaced (a cataract under an optic implant, hearing loss under a
// cochlear one, a bad back under a spinal brace, artery blockage or a
// heart attack under an artificial heart, a carcinoma on an amputated
// limb). It reports whether it removed any.
func (b *Body) DropLostAging() bool {
	if b == nil || len(b.Conds) == 0 {
		return false
	}
	n := len(b.Conds)
	kept := b.Conds[:0]
	for _, c := range b.Conds {
		if !IsAging(c.Kind) || !b.isMissing(c.Part) {
			kept = append(kept, c)
		}
	}
	clear(b.Conds[len(kept):])
	b.Conds = kept
	if len(b.Conds) == 0 {
		b.Conds = nil
	}
	return len(kept) != n
}

// dropAgingOn removes every aging condition on part p (a new natural
// organ in its place).
func (b *Body) dropAgingOn(p Part) {
	for _, c := range append([]Condition(nil), b.Conds...) {
		if IsAging(c.Kind) && c.Part == p {
			b.removeCondOn(c.Kind, p)
		}
	}
}

// dropAgingAfter clears what a successful operation op took away with
// its part (surgery.go succeed): a transplanted organ comes without the
// old one's conditions, and a replaced or removed part loses its own.
func (b *Body) dropAgingAfter(op Operation) {
	if op.Kind == InstallOrgan {
		b.dropAgingOn(op.Part)
	}
	b.DropLostAging()
}

// StepHeartAttack advances the body's heart attack by an hour with roll
// (Roll(n) in [0, n)): a tend given since the last step (TendSum) eases it
// by 300 with a chance of 65 % × its quality, then it moves by −400..+600.
// Below 0 it is over (removed: cured); at AgingDeath the heart stops
// (dead). changed reports a change of stage or its end.
func (b *Body) StepHeartAttack(roll func(int) int) (dead, cured, changed bool) {
	c, ok := b.Cond(CondHeartAttack)
	if !ok {
		return false, false, false
	}
	if c.TendSum > 0 {
		if int32(roll(1000)) < min(heartAttackTendChance*c.TendQ/1000, 1000) {
			c.Sev -= heartAttackTendCut
		}
		c.TendSum = 0
	}
	if c.Sev > 0 {
		c.Sev += heartAttackStepLo + int32(roll(heartAttackStepSpan))
	}
	switch {
	case c.Sev <= 0:
		b.RemoveCond(CondHeartAttack)
		return false, true, true
	case c.Sev >= AgingDeath:
		c.Sev = AgingDeath
		return true, false, true
	}
	stage := DefOfCond(CondHeartAttack).StageAt(c.Sev)
	changed = stage != c.Stage
	c.Stage = stage
	return false, false, changed
}

// AgingStep is what an hour of aging did to a body: Died names the kind
// that killed (CondNone: none), Lost the parts a carcinoma destroyed,
// Ended the kinds that went away by themselves (a carcinoma in remission,
// a heart attack that passed), Changed any change of stage or condition.
type AgingStep struct {
	Died    CondKind
	DiedOn  Part
	Lost    []Part
	Ended   []CondKind
	Changed bool
}

// StepAging advances the body's aging conditions to tick now (hourly):
// the lost parts' conditions go, the heart attack steps (roll), artery
// blockage and carcinomas grow by their rates under salt. A carcinoma at
// full severity destroys its part, or kills when losing the part would.
func (b *Body) StepAging(now, salt uint64, roll func(int) int) AgingStep {
	var out AgingStep
	if b == nil || !b.hasAging() {
		return out
	}
	out.Changed = b.DropLostAging()
	dead, cured, changed := b.StepHeartAttack(roll)
	out.Changed = out.Changed || changed
	if dead {
		out.Died, out.DiedOn = CondHeartAttack, Heart
		return out
	}
	if cured {
		out.Ended = append(out.Ended, CondHeartAttack)
	}
	var lost []Part
	var faded []Part
	for i := range b.Conds {
		c := &b.Conds[i]
		if c.Kind != CondArteryBlockage && c.Kind != CondCarcinoma {
			continue
		}
		out.Changed = GrowAging(c, AgingRate(*c, salt), now) || out.Changed
		switch {
		case c.Sev >= AgingDeath && c.Kind == CondArteryBlockage:
			out.Died, out.DiedOn = CondArteryBlockage, c.Part
			return out
		case c.Sev >= AgingDeath && b.LossKills(c.Part):
			out.Died, out.DiedOn = CondCarcinoma, c.Part
			return out
		case c.Sev >= AgingDeath:
			lost = append(lost, c.Part)
		case c.Sev <= 0:
			faded = append(faded, c.Part)
		}
	}
	for _, p := range faded {
		b.removeCondOn(CondCarcinoma, p)
		out.Ended, out.Changed = append(out.Ended, CondCarcinoma), true
	}
	for _, p := range lost {
		if b.isMissing(p) {
			continue // went with an earlier part
		}
		b.removeCondOn(CondCarcinoma, p)
		b.destroy(p)
		out.Lost, out.Changed = append(out.Lost, p), true
	}
	if len(lost) > 0 {
		b.DropLostAging()
	}
	return out
}
