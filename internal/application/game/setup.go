package game

// New-game setup (NG-1 §5.1): builds a colony from the wizard's Setup in
// budgeted steps so the loading screen shows progress. Owned by SETUP.

// ColonyBuilder builds a new colony step by step.
type ColonyBuilder struct {
	cfg  Config
	g    *Game
	done bool
}

// NewColonyBuilder starts building cfg's colony. A nil cfg.Setup builds the
// legacy colony in one step.
func NewColonyBuilder(cfg Config) *ColonyBuilder { return &ColonyBuilder{cfg: cfg} }

// Step does up to units of work; phase names the current step for the
// loading screen.
func (b *ColonyBuilder) Step(units int) (phase string, frac float32, done bool) {
	if !b.done {
		b.g, b.done = NewColony(b.cfg), true // NG-1 stub (SETUP): legacy in one step
	}
	return "", 1, true
}

// Game is the built colony (nil until done).
func (b *ColonyBuilder) Game() *Game { return b.g }
