package saves

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/clock"
)

// Env is what the book reads of the program around it, as of the call.
type Env struct {
	CanSave    bool // a colony runs and a save hook is wired
	CanLoad    bool // the save list and loading are wired
	AsyncSaves bool // Save only starts a save; Finished and Wait report it (CI-002)
	Commitment bool // the running colony is in commitment mode (NG-1 §2.13)
	Capture    bool // a scripted capture or the fps probe runs: no autosave
}

// Host is what the book needs of the program that no ports role carries.
type Host interface {
	ports.Texts
	// SaveEnv is the program as of now.
	SaveEnv() Env
	// PaintMenu draws the menu backdrop, the title, an optional body and
	// the buttons.
	PaintMenu(dst *ebiten.Image, sw, sh int, title string, body []string, buttons []uikit.Button)
}

// Saver is the saved-game storage as the book uses it.
type Saver interface {
	// Save writes the running colony to the slot meta names (its
	// commitment slot or the manual one), with the colony meta describes.
	Save(meta ports.SaveMeta) error
	// List reads the save list.
	List() ([]ports.SaveSlot, error)
	// Load replaces the session with the save at path and returns what
	// its header says of it and its tick.
	Load(path string) (ports.SaveMeta, clock.Tick, error)
	// Delete removes the save in slot name (the load list's delete).
	Delete(name string) error
	// Finished returns the background saves written since the last call
	// (Env.AsyncSaves), oldest first.
	Finished() []SaveResult
	// Wait blocks until every background save is written.
	Wait()
	// SaveFailed tells the player a background save failed: a Danger
	// toast.
	SaveFailed(text string)
}

// SaveResult is one background save that finished.
type SaveResult struct {
	Slot string
	Err  error // nil: written
}

// Shell composes the roles the book calls with its Host and Saver. World
// gives the date and tick the autosave reads; Widgets the kit the save
// dialog and the load list's modals declare into.
type Shell interface {
	ports.Hinter
	ports.Settings
	ports.World
	ports.Widgets
	Host
	Saver
}
