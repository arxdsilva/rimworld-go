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
)

// Gathering is a party or wedding at a spot (saved).
type Gathering struct {
	Kind                   GatherKind
	SpotX, SpotY           int
	Hosts                  [2]ID
	Phase                  GatherPhase
	PhaseAt, BothArrivedAt uint64
}

// Advance moves the gathering on at tick now; changed reports a new phase.
func (g *Gathering) Advance(now uint64, hostsPresent, abort bool) (changed bool) {
	return false // TODO(M5)
}
