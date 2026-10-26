// Package infection is the race between a condition's severity and the
// body's immunity (MED-02), shared by wound infections and, from MED-04,
// the diseases: StepRace advances one condition hour by hour from its
// definition (body.CondDef), and WoundChance is how likely a fresh wound
// is to fester.
//
// While not yet immune, severity climbs at SevPerDay, less the tend's cut
// (TendMax × tend quality) while a tend is active, and immunity grows at
// ImmPerDay (scaled by the body's immunity gain). Once immunity reaches
// 1000 severity falls at ImmuneSevPerDay (the tend still helping). A
// lethal condition kills at 1000; any condition is cured at 0.
package infection

import "rimworld-go/internal/domain/pawn/body"

const (
	// Full is full severity or immunity (permille).
	Full = 1000
	// Onset is the severity a wound infection starts at (permille). The
	// wiki gives no starting severity: original tuning.
	Onset = 10
	// DelayMin and DelaySpan: an infection roll falls 6–18 hours after
	// the wound (15,000–45,000 ticks). wiki: Infection.
	DelayMin  = 6
	DelaySpan = 12
	// stageBand is how far severity must fall below a stage to leave it
	// (hysteresis; original tuning).
	stageBand = 20
	// hoursPerDay converts the daily rates.
	hoursPerDay = 24
)

// Base infection chances by wound kind (permille). wiki: Infection.
const (
	chanceBiteBurn = 300 // bites and burns
	chanceBleeding = 150 // other bleeding wounds
)

// Tend factor on the chance: 850 at 0 % tend quality falling linearly to
// 50 at 100 %; an untended wound has none (1000). wiki: Infection.
const (
	tendFactorAt0   = 850
	tendFactorAt100 = 50
)

// Room factors on the chance (permille): a roofed, floored room without
// filth halves it; anywhere else leaves it whole. wiki: Infection.
const (
	RoomClean = 500
	RoomOther = 1000
)

// WoundChance is the permille chance a wound of kind festers: its base
// chance × the tend factor (tendQ < 0: untended) × the room factor
// roomPct (permille). Bruises never fester.
func WoundChance(kind body.WoundKind, tendQ int32, roomPct int32) int32 {
	var base int32
	switch kind {
	case body.Bite, body.Burn:
		base = chanceBiteBurn
	case body.Cut, body.Scratch, body.Surgical:
		base = chanceBleeding
	default:
		return 0
	}
	tf := int32(1000)
	if tendQ >= 0 {
		q := min(tendQ, Full)
		tf = tendFactorAt0 - (tendFactorAt0-tendFactorAt100)*q/Full
	}
	return base * tf / 1000 * roomPct / 1000
}

// perHour is a daily rate's share of one hour, rounded to the nearest
// permille (the conditions keep whole permille; the rounding error of
// the infection rates is under 1 %).
func perHour(perDay int32) int32 {
	if perDay < 0 {
		return -((-perDay + hoursPerDay/2) / hoursPerDay)
	}
	return (perDay + hoursPerDay/2) / hoursPerDay
}

// SevPerDay is how fast c's severity moves now (permille per day): the
// definition's rate for its immunity, less the active tend's cut.
func SevPerDay(c body.Condition, d body.CondDef, tended bool) int32 {
	r := d.SevPerDay
	if c.Immunity >= Full && !d.NoImmunity {
		r = d.ImmuneSevPerDay
	}
	if tended {
		r -= d.TendMax * min(max(c.TendQ, 0), Full) / Full
	}
	return r
}

// ImmPerDay is how fast c's immunity grows (permille per day) with the
// body's immunity gain immMul (permille, 1000 normal).
func ImmPerDay(d body.CondDef, immMul int32) int32 {
	if d.NoImmunity {
		return 0
	}
	return d.ImmPerDay * immMul / 1000
}

// StepRace advances c by hours: severity by SevPerDay, immunity by
// ImmPerDay, then its stage (entered at a stage's At, left stageBand
// below it). dead: a lethal condition reached full severity; cured: its
// severity fell to 0. tended says a tend is active for the whole step.
func StepRace(c *body.Condition, d body.CondDef, tended bool, immMul int32, hours int) (dead, cured bool) {
	for h := 0; h < hours; h++ {
		sev := perHour(SevPerDay(*c, d, tended))
		if c.Immunity < Full {
			c.Immunity = min(c.Immunity+perHour(ImmPerDay(d, immMul)), Full)
		}
		c.Sev = min(max(c.Sev+sev, 0), Full)
		c.Stage = stageWithBand(d, c.Sev, c.Stage)
		switch {
		case d.Lethal && c.Sev >= Full:
			return true, false
		case c.Sev <= 0:
			return false, true
		}
	}
	return false, false
}

// stageWithBand is the stage of sev given the previous one: up as soon as
// a stage's At is reached, down only once sev is stageBand below it.
func stageWithBand(d body.CondDef, sev int32, prev uint8) uint8 {
	s := d.StageAt(sev)
	if s >= prev || int(prev) >= len(d.Stages) {
		return s
	}
	if sev >= d.Stages[prev].At-stageBand {
		return prev
	}
	return d.StageAt(sev + stageBand)
}

// Losing reports c on course to kill before immunity wins: severity
// reaches full sooner than immunity does at today's rates (tended: the
// active tend holds the whole way). Never for a non-lethal condition or
// one already immune.
func Losing(c body.Condition, d body.CondDef, tended bool, immMul int32) bool {
	if !d.Lethal || c.Immunity >= Full {
		return false
	}
	sev := SevPerDay(c, d, tended)
	if sev <= 0 {
		return false
	}
	imm := ImmPerDay(d, immMul)
	if imm <= 0 {
		return true
	}
	// days to death (Full−Sev)/sev < days to immunity (Full−Imm)/imm
	return int64(Full-c.Sev)*int64(imm) < int64(Full-c.Immunity)*int64(sev)
}

// HoursToDeath is how many hours c has left at today's rates if it is
// losing (-1 when not).
func HoursToDeath(c body.Condition, d body.CondDef, tended bool, immMul int32) int {
	if !Losing(c, d, tended, immMul) {
		return -1
	}
	sev := SevPerDay(c, d, tended)
	return int((int64(Full-c.Sev)*hoursPerDay + int64(sev) - 1) / int64(sev))
}
