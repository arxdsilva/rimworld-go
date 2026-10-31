// Package health is pawn vitality: alive, downed, or dead.
package health

// Status is a pawn's life state.
type Status uint8

const (
	Alive Status = iota
	Downed
	Dead
)

func (s Status) String() string {
	switch s {
	case Downed:
		return "downed"
	case Dead:
		return "dead"
	default:
		return "alive"
	}
}

// Cause is why a pawn is downed or dead (player-facing inspector text).
type Cause uint8

const (
	CauseNone Cause = iota
	CauseNoFood
	CauseNoBed
	CauseInjury
	CauseHypothermia
	CauseHeatstroke
	CauseInfection
	// CauseExecution and CauseSurgery (ENG-003): an executed prisoner, and a
	// botched operation.
	CauseExecution
	CauseSurgery
	// CauseBurns (PLAY-015): burned to death in a fire.
	CauseBurns
	// CauseBloodLoss (MED-01): bled to death.
	CauseBloodLoss
	// CauseOrganFailure (MED-05): a capacity the body cannot live without
	// (blood pumping, filtration, breathing, digestion) fell to 0.
	CauseOrganFailure
	// CauseDisease (MED-04): flu, plague, malaria or sleeping sickness
	// ran its course.
	CauseDisease
	// CauseEuthanasia (MED-06): put down by a doctor. CauseAnesthetic
	// (MED-06): lying under an operation's anaesthetic (downed only).
	CauseEuthanasia
	CauseAnesthetic
	// CauseOverdose (MED-07): a drug overdose killed it.
	CauseOverdose
	// CauseHeartAttack and CauseCarcinoma (MED-08): the heart stopped (a
	// heart attack, or an artery blockage at its end), and a carcinoma
	// destroyed a part the body cannot live without.
	CauseHeartAttack
	CauseCarcinoma
	// CauseCatatonic (SIM-066): a catatonic breakdown; the pawn lies down
	// until the break ends.
	CauseCatatonic
)

func (c Cause) String() string {
	switch c {
	case CauseNoFood:
		return "no food"
	case CauseNoBed:
		return "no bed"
	case CauseInjury:
		return "injured"
	case CauseHypothermia:
		return "hypothermia"
	case CauseHeatstroke:
		return "heatstroke"
	case CauseInfection:
		return "infection"
	case CauseExecution:
		return "executed"
	case CauseSurgery:
		return "botched surgery"
	case CauseBurns:
		return "burns"
	case CauseBloodLoss:
		return "blood loss"
	case CauseOrganFailure:
		return "organ failure"
	case CauseDisease:
		return "disease"
	case CauseEuthanasia:
		return "euthanized"
	case CauseAnesthetic:
		return "under anesthetic"
	case CauseOverdose:
		return "overdose"
	case CauseHeartAttack:
		return "heart attack"
	case CauseCarcinoma:
		return "carcinoma"
	case CauseCatatonic:
		return "catatonic breakdown"
	default:
		return ""
	}
}

// Record is vitality plus a typed cause.
type Record struct {
	Status     Status
	Cause      Cause
	DownedTick uint64
	// Injury is damage taken; the zero value is full health.
	Injury int
}

// Injury thresholds (hit points out of MaxHP). Humanlikes with a body
// go down by their capacities (BodyDowns, MED-01); DownedInjury and
// RecoverInjury remain the legacy rule for a pawn with no body (until the
// animal medical phase, MED-09, moves animals onto bodies too).
const (
	MaxHP = 100
	// DownedInjury downs a pawn; RecoverInjury lets it stand again.
	DownedInjury  = 70
	RecoverInjury = 55
	// HealTicks is how often one point of injury heals (awake, up);
	// resting in bed heals twice as fast, lying downed half as fast.
	HealTicks = 300
)

// HP is remaining hit points.
func (r Record) HP() int {
	hp := MaxHP - r.Injury
	if hp < 0 {
		return 0
	}
	return hp
}

// Capacity downing (MED-01), in permille. A body goes down when
// consciousness falls below DownConsciousness, moving below DownMoving or
// pain reaches its pain shock threshold (PainShock for a plain pawn). It
// stands again only when every one is back past its recover value: the
// gap between the two is the hysteresis band, so a body hovering at a
// threshold does not flap between lying and standing.
const (
	DownConsciousness    = 300 // wiki: Consciousness (incapacitated below 30 %)
	RecoverConsciousness = 350 // original tuning (hysteresis band)
	DownMoving           = 150 // wiki: Moving (incapacitated below 15 %)
	RecoverMoving        = 200 // original tuning (hysteresis band)
	PainShock            = 800 // wiki: Pain (pain shock at 80 %)
	PainShockBand        = 50  // pain must fall this far below the shock threshold; original tuning
)

// BodyDowns reports a body with consciousness cons, moving and pain (all
// permille) incapacitated, with pain shock at shock.
func BodyDowns(cons, moving, pain, shock int32) bool {
	return cons < DownConsciousness || moving < DownMoving || pain >= shock
}

// BodyRecovered reports a downed body able to stand again: every capacity
// past its recover value and pain under the shock threshold by the band.
func BodyRecovered(cons, moving, pain, shock int32) bool {
	return cons >= RecoverConsciousness && moving >= RecoverMoving && pain <= shock-PainShockBand
}
