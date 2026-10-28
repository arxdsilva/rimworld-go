package wizard

import (
	"hash/fnv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
)

// Start and loading (NG-1 §2.10): Start turns the draft into a game.Setup
// and builds the colony in steps behind a progress screen, so a big map
// shows progress instead of freezing.

// LoadPhases are the loading screen's phases.
var LoadPhases = []string{"Preparing the site", "Shaping the land", "Placing rock and ore", "Seeding plants and animals", "Arrival"}

// LoadUnits is the work units stepped per frame.
const LoadUnits = 4000

// loadState is a colony being built.
type loadState struct {
	b     Loader
	Phase string
	frac  float32
}

// Fnv64a hashes text.
func Fnv64a(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// SetupSeedOf is the setup seed (NG-1 §4.7):
// Fnv64a(world seed text) ^ tile * 0x9E3779B97F4A7C15 ^ scenario seed.
func SetupSeedOf(ng *Draft) uint64 {
	text := ng.World.SeedText
	if ng.Planet != nil {
		text = ng.Planet.Params.SeedText
	}
	return Fnv64a(text) ^ uint64(int64(ng.Tile))*0x9E3779B97F4A7C15 ^ ng.Scenario.Seed
}

// cloneFactions copies the generated factions so the colony owns them.
func cloneFactions(fs []faction.Faction) []faction.Faction {
	out := make([]faction.Faction, len(fs))
	for i, f := range fs {
		f.Settlements = append([]int(nil), f.Settlements...)
		f.SettlementNames = append([]string(nil), f.SettlementNames...)
		f.History = append([]int16(nil), f.History...)
		f.Events = append([]string(nil), f.Events...)
		out[i] = f
	}
	return out
}

// BuildSetup turns the draft into a game.Setup. It needs a generated
// planet, a landing tile and the candidate pool.
func (ng *Draft) BuildSetup() (game.Setup, bool) {
	if ng.Planet == nil || ng.Tile < 0 || ng.Pool == nil {
		return game.Setup{}, false
	}
	diff := difficulty.Of(ng.Preset)
	if ng.Preset == difficulty.Custom {
		diff = ng.Custom
	}
	params := ng.Planet.Params
	params.Factions = append([]faction.Slot(nil), params.Factions...)
	st := ng.Planet.Site(ng.Tile)
	return game.Setup{
		Scenario:    ng.Scenario,
		Storyteller: ng.Storyteller,
		Preset:      ng.Preset,
		Difficulty:  diff,
		SaveMode:    ng.SaveMode,
		World:       params,
		Tile:        ng.Tile,
		Site:        st,
		Factions:    cloneFactions(ng.Planet.Factions),
		MapSize:     ng.MapSize,
		Season:      ng.ResolvedSeason(),
		Colonists:   ng.Pool.Selected(),
		Seed:        SetupSeedOf(ng),
	}, true
}

// beginLoading is the Start button: build the setup, start the builder and
// run its first step (a one-step builder is done at once). The shell keeps
// why a start failed (Env.SessionErr).
func (w *Flow) beginLoading() {
	ng := w.draft()
	var b Loader
	var err error
	if setup, ok := ng.BuildSetup(); ok {
		b, err = w.sh.Load(&setup)
	} else {
		b, err = w.sh.Load(nil)
	}
	if err != nil {
		return
	}
	ng.Load = &loadState{b: b, Phase: LoadPhases[0]}
	w.loadingStep()
}

// loadingStep advances the colony builder one frame; when it is done the
// shell installs the colony and drops the draft (Drop).
func (w *Flow) loadingStep() {
	ng := w.d
	if ng == nil || ng.Load == nil {
		return
	}
	ld := ng.Load
	phase, frac, done := ld.b.Step(LoadUnits)
	if phase == "" {
		phase = LoadPhases[min(int(frac*float32(len(LoadPhases))), len(LoadPhases)-1)]
	}
	ld.Phase, ld.frac = phase, frac
	if !done {
		return
	}
	ng.Load = nil
	if err := ld.b.Finish(); err != nil {
		return // the shell keeps why (Env.SessionErr); Create Characters shows it
	}
}

// drawLoading paints the loading screen over the setup backdrop.
func (w *Flow) drawLoading(screen *ebiten.Image, sw, sh int) {
	w.DrawBackdrop(screen, sw, sh, w.d.Planet)
	ld := w.d.Load
	box := gfx.WinRect(sw, sh).Centered(560, 150)
	uikit.DrawPanel(screen, box.X, box.Y, box.W, box.H)
	st := gfx.Large
	st.Color = uikit.Heading
	gfx.Text(screen, "Preparing your colony", float64(box.X+20), float64(box.Y+16), st)
	uikit.DrawProgress(screen, gfx.Rect{X: box.X + 20, Y: box.Y + 56, W: box.W - 40, H: 60}, ld.Phase, float64(ld.frac), "")
}
