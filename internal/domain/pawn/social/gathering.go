package social

// GatherKind is a gathering (append only, saved).
type GatherKind uint8

const (
	Party GatherKind = iota
	Wedding
)

// GatherPhase is how far a gathering has got (append only, saved).
type GatherPhase uint8

const (
	PhaseCalling GatherPhase = iota
	PhaseUnderway
	PhaseOver
	PhaseCeremony
	PhaseReception
	PhaseAborted
)

// Names for the spec's phases.
const (
	PhaseScheduled = PhaseCalling
	PhaseGathering = PhaseUnderway
	PhaseDone      = PhaseOver
)

// Gathering is a party or wedding at a spot (saved).
type Gathering struct {
	Kind                   GatherKind
	SpotX, SpotY           int
	Hosts                  [2]ID
	Phase                  GatherPhase
	PhaseAt, BothArrivedAt uint64
}

// Finished reports Done or Aborted.
func (g *Gathering) Finished() bool { return g.Phase == PhaseOver || g.Phase == PhaseAborted }

// RetryAt is when an aborted gathering may be tried again.
func (g *Gathering) RetryAt() uint64 { return g.PhaseAt + GatherRetryAfter }

func (g *Gathering) set(p GatherPhase, now uint64) bool {
	g.Phase = p
	g.PhaseAt = now
	return true
}

// Advance moves the gathering on at tick now; changed reports a new phase.
// A wedding needs both hosts present for WeddingGatherMin before the
// ceremony, then the reception; a party starts once its host arrives and
// ends after PartyDuration. abort ends it early (threat, draft, downed,
// mental break).
func (g *Gathering) Advance(now uint64, hostsPresent, abort bool) (changed bool) {
	if g.Finished() {
		return false
	}
	if abort {
		return g.set(PhaseAborted, now)
	}
	switch g.Phase {
	case PhaseCalling:
		if !hostsPresent {
			return false
		}
		g.BothArrivedAt = now
		return g.set(PhaseUnderway, now)
	case PhaseUnderway:
		if g.Kind == Party {
			if now >= g.PhaseAt+PartyDuration {
				return g.set(PhaseOver, now)
			}
			return false
		}
		if !hostsPresent {
			// A host wandered off: wait for them again from the start.
			g.BothArrivedAt = 0
			return g.set(PhaseCalling, now)
		}
		if now >= g.BothArrivedAt+WeddingGatherMin {
			return g.set(PhaseCeremony, now)
		}
	case PhaseCeremony:
		if now >= g.PhaseAt+WeddingCeremony {
			return g.set(PhaseReception, now)
		}
	case PhaseReception:
		if now >= g.PhaseAt+WeddingReception {
			return g.set(PhaseOver, now)
		}
	}
	return false
}
