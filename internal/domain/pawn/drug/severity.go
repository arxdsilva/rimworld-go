package drug

// Alcohol bands (permille severity).
const (
	alcoholDecayHour = 75
	tipsyAt          = 250
	drunkAt          = 400
	hammeredAt       = 700
)

// Band is how drunk a colonist is.
type Band uint8

const (
	Sober Band = iota
	Warm
	Tipsy
	Drunk
	Hammered
)

var bandLabels = [...]string{"sober", "warm", "tipsy", "drunk", "hammered"}
var bandFight = [...]int32{1000, 1500, 2500, 4000, 5000}

// Label is the original UI word for the band.
func (b Band) Label() string { return bandLabels[b] }

// FightMult is the social-fight multiplier of the band (permille).
func (b Band) FightMult() int32 { return bandFight[b] }

// Band is the current alcohol band.
func (s *State) Band() Band {
	switch {
	case s.Alcohol <= 0:
		return Sober
	case s.Alcohol < tipsyAt:
		return Warm
	case s.Alcohol < drunkAt:
		return Tipsy
	case s.Alcohol < hammeredAt || s.Doses < 1:
		return Drunk
	}
	return Hammered
}

func (s *State) tickAlcohol(now uint64) {
	if s.Alcohol <= 0 {
		return
	}
	s.Alcohol -= alcoholDecayHour
	if s.Alcohol > 0 {
		return
	}
	s.Alcohol, s.Doses = 0, 0
	if s.Peaked {
		s.Peaked = false
		s.Hangover = now + hangoverTicks
		s.HangPend = true
	}
}

// ConsumeHangover reports once that a hangover just began; the caller adds
// the mood.Hungover memory.
func (s *State) ConsumeHangover() bool {
	p := s.HangPend
	s.HangPend = false
	return p
}

// Hungover reports whether a hangover is active at tick now.
func (s *State) Hungover(now uint64) bool { return s.Hangover > now }
