// Package wizard is the New Colony flow (NG-1): scenario choice and the
// Scenario Editor, storyteller and difficulty, world generation and the
// landing site on the globe, the factions, the starting colonists and the
// loading step. Flow owns the draft and reaches the shell through a small
// interface (Shell); the colony itself is built and installed by the shell.
package wizard

import (
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/platform/config"
)

// ScenarioStore keeps custom scenario files (NG-1 §2.3, §5.7). It is
// consumer-owned: cmd/game wires the outbound scenariofile adapter.
type ScenarioStore interface {
	List() ([]ScenarioFile, error)
	Load(name string) ([]byte, error)
	Save(name string, data []byte) error
	Delete(name string) error
}

// ScenarioFile is one saved custom scenario.
type ScenarioFile struct {
	Name     string // file name without extension
	Modified string // display date
}

// Loader builds the colony of a setup in steps behind the loading screen.
type Loader interface {
	// Step does up to units of work and names the phase now running ("":
	// the screen names it from the progress).
	Step(units int) (phase string, frac float32, done bool)
	// Finish installs the built colony (once Step reported done) and drops
	// the draft (Flow.Drop), or reports why it could not.
	Finish() error
}

// Env is what the wizard reads of the program around it, as of the call:
// the screen on top, the wiring (scenario files, art, the -seed flag,
// whether a scripted capture runs), the dev-mode setting, the menu clock,
// why the last start failed and the wall clock.
type Env struct {
	Screen      ports.Screen
	Scenarios   ScenarioStore // nil: custom scenarios unavailable
	Art         *art.Catalog  // nil while the art is missing
	DefaultSeed uint64        // pre-fills the world seed (0: a random phrase)
	Capture     bool          // a scripted capture runs (deterministic frames)
	DevMode     bool
	MenuTick    uint64 // drives the backdrop's drift
	SessionErr  string // why the last start failed ("" none)
	Now         time.Time
}

// Host is what the wizard needs of the program that no ports role
// carries.
type Host interface {
	// Env is the program as of now.
	Env() Env
	// KeepDraft hands the shell the draft (nil: none), which it keeps for
	// the colony's World tab (the founding planet).
	KeepDraft(*Draft)
	// SaveSettings stores changed Options (the last storyteller choices).
	SaveSettings(config.Settings)
	// SliderTick gives a slider the shell's step sound.
	SliderTick(uikit.SliderSpec) uikit.SliderSpec
	// Load starts building the colony of setup s (nil: the setup is
	// incomplete); the shell keeps why it failed (Env.SessionErr).
	Load(s *game.Setup) (Loader, error)
	// DrawPortrait draws a colonist of look l fitted into the box; false
	// while the art is missing.
	DrawPortrait(dst *ebiten.Image, l look.Look, x, y, w, h float64) bool
	// DrawDiorama draws the main menu's colony diorama dimmed by dim; false
	// when there is none.
	DrawDiorama(dst *ebiten.Image, sw, sh int, dim float64) bool
	// MapSizeNote is the memory warning for a side x side map on this
	// computer ("" none).
	MapSizeNote(side int) string
}

// Shell composes the roles the wizard calls with its Host.
type Shell interface {
	ports.Hinter
	ports.Settings
	ports.Widgets
	ports.Dispatcher
	Host
}
