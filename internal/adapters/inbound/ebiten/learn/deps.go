package learn

import (
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/platform/config"
)

// Sensed is what the colony shows of itself that a UI-event lesson
// compares between Updates.
type Sensed struct {
	EdgePan      bool   // the pointer is in an edge-scroll band of a focused window
	Selection    string // names what is selected ("" nothing); a change is a new selection
	Speed        int    // the game speed tier (0 paused)
	ThreatPaused bool   // the game paused itself on a threat
}

// Host is what the feature needs of the program that no ports role
// carries (at most eight methods, SOLID_GO.md §6).
type Host interface {
	// Colony identifies the running colony (nil: none, a menu screen) and
	// says whether a scripted capture runs it.
	Colony() (colony any, scripted bool)
	// Chain makes observe see every command the player sends, after any
	// observer already on the colony.
	Chain(observe func(command.Command))
	// Frame is this Update's resolved input.
	Frame() *inputframe.Frame
	// Sense reads the colony state the UI-event lessons compare.
	Sense() Sensed
	// Layout is this frame's HUD layout (never called from Height).
	Layout() hudlayout.Layout
	// Updates counts the Updates run so far.
	Updates() uint64
	// Save stores changed settings (normalised, no re-apply) and reports
	// why they could not be written.
	Save(config.Settings) error
}

// Shell composes the roles the feature calls with its Host.
type Shell interface {
	ports.Hinter
	ports.Sizer
	ports.Settings
	ports.Widgets
	Host
}
