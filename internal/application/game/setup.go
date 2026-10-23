package game

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/generation"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/temperature"
)

// New-game setup (NG-1 §5.1): builds a colony from the wizard's Setup in
// budgeted steps so the loading screen shows progress. Owned by SETUP.
//
// A nil Config.Setup is the legacy colony (NewColony, in one step) and
// every NG-1 hook in the package is a no-op for it, so legacy games, the
// -scenario set pieces and the golden hashes are unchanged bit for bit.

// ng1State is the live state of a colony started from a Setup. The zero
// value is a legacy colony.
type ng1State struct {
	setup *Setup           // nil: legacy
	fx    scenario.Effects // setup.Scenario.Apply() (derived, not saved)
	// factions are the world's other peoples (copied from the Setup).
	factions []faction.Faction
	// story is the storyteller's memory; plan the pending threat (its
	// AtTick relative to the landing day); adapt the difficulty adaptation.
	story storyteller.State
	plan  storyteller.Decision
	adapt difficulty.Adapter
	// startDay is the calendar day the colony landed on: colony days
	// (relDay) count from it.
	startDay int
	// dead are the colonists already counted as lost; lost is set by a
	// death since the last day rollover (adaptation).
	dead []entity.EntityID
	lost bool
	// extreme is the heat wave or cold snap in progress (weather_ng1.go).
	extreme extremeEvent
	// pods are the arrival's drop pods (arrival.go); introDone: the intro
	// letter was posted.
	pods      []podState
	introDone bool
}

// Builder phases (NG-1 §2.10), in order.
var buildPhases = [...]string{
	"Preparing the site",
	"Shaping the land",
	"Placing rock and ore",
	"Seeding plants and animals",
	"Arrival",
}

// ColonyBuilder builds a new colony step by step.
type ColonyBuilder struct {
	cfg  Config
	g    *Game
	res  generation.Result
	step int
	done bool
}

// NewColonyBuilder starts building cfg's colony. A nil cfg.Setup builds the
// legacy colony in one step.
func NewColonyBuilder(cfg Config) *ColonyBuilder { return &ColonyBuilder{cfg: cfg} }

// Step does up to units of work (at least one build step); phase names the
// step now in progress for the loading screen ("" once done) and frac the
// share done.
func (b *ColonyBuilder) Step(units int) (phase string, frac float32, done bool) {
	if b.done {
		return "", 1, true
	}
	if b.cfg.Setup == nil {
		b.g, b.done = NewColony(b.cfg), true
		return "", 1, true
	}
	for n := max(units, 1); n > 0 && !b.done; n-- {
		b.runStep()
	}
	if b.done {
		return "", 1, true
	}
	return buildPhases[b.step], float32(b.step) / float32(len(buildPhases)), false
}

// Game is the built colony (nil until done).
func (b *ColonyBuilder) Game() *Game {
	if !b.done {
		return nil
	}
	return b.g
}

// runStep runs the next build step.
func (b *ColonyBuilder) runStep() {
	s := b.cfg.Setup
	switch b.step {
	case 0:
		b.g = newSetupGame(b.cfg, s)
	case 1:
		side := b.g.grid.Width()
		b.res = generation.GenerateSite(s.Seed, side, side, biome.DefOf(s.Site.Biome), s.Site)
	case 2:
		b.g.applySite(b.res, s)
	case 3:
		g := b.g
		for _, p := range b.res.Plants {
			_, _ = g.spawnPlant(p.Species, p.X, p.Y, p.Growth)
		}
		g.seedGrass() // before anything is built, as in NewColony
		g.spawnWildlife()
	case 4:
		b.g.finishSetup()
		b.g.warmIndexes(true) // jobindex.go: not on the first tick
		b.done = true
	}
	b.step++
}

// mapSideOf is the Setup's map side (MaxEnabledSide when unset).
func mapSideOf(cfg Config) int {
	if cfg.Setup != nil && cfg.Setup.MapSize > 0 {
		return cfg.Setup.MapSize
	}
	if cfg.Width > 0 {
		return cfg.Width
	}
	return MaxEnabledSide
}

// newSetupGame is New for a Setup: the chosen size, biome and climate, the
// clock at 08:00 on the chosen season's first day, and the setup seed.
func newSetupGame(cfg Config, s *Setup) *Game {
	side := mapSideOf(cfg)
	cfg.Width, cfg.Height, cfg.Biome, cfg.Seed = side, side, s.Site.Biome, s.Seed
	g := New(cfg)
	cp := *s
	cp.Factions = cloneFactionList(s.Factions)
	cp.Colonists = slices.Clone(s.Colonists)
	g.ng.setup = &cp
	g.ng.fx = cp.Scenario.Apply()
	g.ng.factions = cloneFactionList(s.Factions)
	g.climate = s.Site.Climate
	if g.climate == (temperature.Climate{}) {
		g.climate = biome.DefOf(s.Site.Biome).Climate
	}
	g.climate = withConditions(g.climate, g.ng.fx.Conditions)
	g.ng.startDay = seasonStartDay(g.climate.PeakDay, s.Season)
	g.clock = clock.NewAt(clock.Tick(g.ng.startDay*clock.TicksPerDay + clock.StartHour*clock.TicksPerHour))
	g.research = research.Progress{} // a new colony knows nothing yet
	return g
}

// cloneFactionList deep-copies factions.
func cloneFactionList(fs []faction.Faction) []faction.Faction {
	out := make([]faction.Faction, len(fs))
	for i, f := range fs {
		f.Settlements = slices.Clone(f.Settlements)
		f.SettlementNames = slices.Clone(f.SettlementNames)
		f.History = slices.Clone(f.History)
		f.Events = slices.Clone(f.Events)
		out[i] = f
	}
	return out
}

// seasonStartDay is the calendar day a season starts on for a climate
// peaking on peak (15 north, 45 south): seasons are 15 days centred on
// the temperature curve (temperature.Climate.SeasonAt), summer first.
func seasonStartDay(peak int, s Season) int {
	var idx int // 0 summer, 1 fall, 2 winter, 3 spring
	switch s {
	case SeasonSummer:
		idx = 0
	case SeasonFall:
		idx = 1
	case SeasonWinter:
		idx = 2
	default:
		idx = 3
	}
	q := clock.DaysPerQuadrum
	d := peak - q/2 + idx*q
	return (d%clock.DaysPerYear + clock.DaysPerYear) % clock.DaysPerYear
}

// withConditions applies the permanent conditions to a climate (NG-1
// §5.1): Endless chill -8 °C; Long summer +6 °C with a 30% smaller swing.
func withConditions(c temperature.Climate, conds []scenario.Condition) temperature.Climate {
	for _, k := range conds {
		switch k {
		case scenario.EndlessChill:
			c.YearMean -= 80
		case scenario.LongSummer:
			c.YearMean += 60
			c.Amplitude = c.Amplitude * 7 / 10
		}
	}
	return c
}

// applySite lays the generated site on the map: terrain, the site's
// stones and ore (with machinery), and a natural roof over the caves.
func (g *Game) applySite(res generation.Result, s *Setup) {
	for y := 0; y < g.grid.Height(); y++ {
		for x := 0; x < g.grid.Width(); x++ {
			g.setTerrain(x, y, res.Terrain.At(x, y))
		}
	}
	g.geo = geology.GenerateWith(s.Seed, g.terr, s.Site.Stones)
	g.structVer++
	w := g.grid.Width()
	for i, c := range res.Cave {
		if c {
			g.roofOnMined(i%w, i/w)
		}
	}
}

// finishSetup is the last build step: arrival, Home, research, scenario
// rules and the storyteller (§5.1 steps 3-10).
func (g *Game) finishSetup() {
	g.landColony() // arrival.go
	g.startHome()  // home.go: around the landing spot (the map centre)
	for _, p := range g.ng.fx.Research {
		g.grantResearch(p)
	}
	g.scheduleThreats() // raids.go: the storyteller's first plan
}

// grantResearch finishes project p and everything it requires.
func (g *Game) grantResearch(p research.Project) {
	if p >= research.NumProjects || g.research.Done(p) {
		return
	}
	d := research.DefOf(p)
	for _, r := range d.Requires {
		g.grantResearch(r)
	}
	g.research[p] = d.Cost
}

// --- helpers for the rest of the package -----------------------------------

// hasSetup reports a colony started from the new-game wizard.
func (g *Game) hasSetup() bool { return g.ng.setup != nil }

// dayOff is the landing day's first tick: colony time starts there.
func (g *Game) dayOff() uint64 { return uint64(g.ng.startDay) * clock.TicksPerDay }

// rel is now as colony time (ticks since the landing day began); a legacy
// colony lands on day 0, so it is now itself.
func (g *Game) rel(now uint64) uint64 {
	if off := g.dayOff(); now >= off {
		return now - off
	}
	return 0
}

// relDay is the colony day of now (0 on the landing day).
func (g *Game) relDay(now uint64) int { return int(g.rel(now) / clock.TicksPerDay) }

// season names the season at now: the map's own calendar (a southern site
// has its summer in the north's winter); the legacy calendar otherwise.
func (g *Game) season(now uint64) string {
	if g.hasSetup() {
		return g.climate.SeasonAt(clock.Tick(now))
	}
	return temperature.Season(clock.Tick(now))
}

// diff is the colony's difficulty (Seasoned for a legacy colony; callers
// check hasSetup where legacy must stay exact).
func (g *Game) diff() difficulty.Settings {
	if g.hasSetup() {
		return g.ng.setup.Difficulty
	}
	return difficulty.Of(difficulty.Seasoned)
}

// Difficulty fields that scale yields (diffScale).
const (
	DiffCrop = iota
	DiffMine
	DiffButcher
	DiffResearch
)

// diffScale scales a yield base by the difficulty's percent for field
// (DiffCrop, DiffMine, DiffButcher, DiffResearch). A legacy colony gets
// base unchanged; a mine yield of at least 1 stays at least 1.
func (g *Game) diffScale(field, base int) int {
	if !g.hasSetup() {
		return base
	}
	s := g.ng.setup.Difficulty
	pct := 100
	switch field {
	case DiffCrop:
		pct = s.CropYield
	case DiffMine:
		pct = s.MineYield
	case DiffButcher:
		pct = s.ButcherYield
	case DiffResearch:
		pct = s.ResearchSpeed
	}
	v := difficulty.Scale(pct, base)
	if field == DiffMine && base > 0 {
		v = max(v, 1)
	}
	return v
}

// moodOffset is the difficulty's constant mood offset (0 for a legacy
// colony); the thoughts show it as a "Difficulty" line when non-zero.
func (g *Game) moodOffset() int {
	if !g.hasSetup() {
		return 0
	}
	return g.ng.setup.Difficulty.MoodOffset
}

// researchCostPct is the scenario's research cost in percent (tech level
// times research-cost parts); 100 for a legacy colony.
func (g *Game) researchCostPct() int {
	if !g.hasSetup() || g.ng.fx.ResearchCostPct <= 0 {
		return 100
	}
	return g.ng.fx.ResearchCostPct
}

// growthPct is plant growth speed in percent under the permanent
// conditions (Ashfall sky: 85); 100 otherwise.
func (g *Game) growthPct() int {
	if g.hasCondition(scenario.AshfallSky) {
		return 85
	}
	return 100
}

// hasCondition reports a permanent scenario condition.
func (g *Game) hasCondition(c scenario.Condition) bool {
	return g.hasSetup() && slices.Contains(g.ng.fx.Conditions, c)
}

// ReasonScenario is the placement reason for a building the scenario
// forbids.
const ReasonScenario = "Not allowed by this scenario"

// scenarioForbids reports a building kind the scenario disallows.
func (g *Game) scenarioForbids(k construction.Kind) bool {
	return g.hasSetup() && g.ng.fx.Disallowed[k]
}

// incidentDisabled reports an incident key the scenario disables.
func (g *Game) incidentDisabled(key string) bool {
	return g.hasSetup() && g.ng.fx.Disabled[key]
}

// spawnGeneratedFn spawns a generated colonist (skills.go).
var spawnGeneratedFn = (*Game).spawnGenerated

// --- saves -------------------------------------------------------------------

// SetupState is the NG-1 state a save carries (nil for a legacy colony).
type SetupState struct {
	Setup     Setup
	Factions  []faction.Faction
	Story     storyteller.State
	Plan      storyteller.Decision
	Adapt     difficulty.Adapter
	Climate   temperature.Climate
	StartDay  int
	Dead      []entity.EntityID
	Lost      bool
	Extreme   extremeEvent
	Pods      []podState
	IntroDone bool
}

// exportSetupState is the setup state for a save (nil: legacy colony).
func (g *Game) exportSetupState() *SetupState {
	if !g.hasSetup() {
		return nil
	}
	st := g.ng
	s := *st.setup
	s.Factions = cloneFactionList(s.Factions)
	s.Colonists = slices.Clone(s.Colonists)
	return &SetupState{
		Setup: s, Factions: cloneFactionList(st.factions), Story: st.story, Plan: st.plan, Adapt: st.adapt,
		Climate: g.climate, StartDay: st.startDay, Dead: slices.Clone(st.dead), Lost: st.lost,
		Extreme: st.extreme, Pods: clonePods(st.pods), IntroDone: st.introDone,
	}
}

// importSetupState restores the setup state after a load (nil leaves a
// legacy colony). It must run after the climate is re-derived from the
// biome: a setup colony keeps its saved climate.
func (g *Game) importSetupState(s *SetupState) {
	if s == nil {
		g.ng = ng1State{}
		return
	}
	setup := s.Setup
	g.ng = ng1State{
		setup: &setup, fx: setup.Scenario.Apply(), factions: cloneFactionList(s.Factions),
		story: s.Story, plan: s.Plan, adapt: s.Adapt, startDay: s.StartDay,
		dead: slices.Clone(s.Dead), lost: s.Lost, extreme: s.Extreme, pods: clonePods(s.Pods), introDone: s.IntroDone,
	}
	if s.Climate != (temperature.Climate{}) {
		g.climate = s.Climate
	}
}
