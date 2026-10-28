package dialogs

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// speedPausedTier is the paused speed tier (ports.Speed).
const speedPausedTier = 0

// Host is what the feature needs of the program that no ports role
// carries (at most eight methods, SOLID_GO.md §6).
type Host interface {
	// Trade is the open trade session; false with no session or when the
	// simulation does not offer one (optional TradeSource).
	Trade() (game.TradeView, bool)
	// Comms is the open comms session; false likewise (optional CommsSource).
	Comms() (game.CommsView, bool)
	// Sites are the settlements a caravan may visit (nil when the
	// simulation lists none).
	Sites() []game.WorldSiteView
	// HoldSpeed sets the speed tier without the speed cue: the dialogs
	// pause the display while open and give the tier back after.
	HoldSpeed(tier int)
}

// Shell composes the roles the feature calls with its Host.
type Shell interface {
	ports.Commands
	ports.World
	ports.Speed
	ports.Widgets
	Host
}

// Dealings is the trade, comms and caravan dialogs' state: which dialog is
// open, which the player dismissed (so the still-open session does not
// reopen it) and the speed tier to resume.
type Dealings struct {
	sh             Shell
	trade          *tradeDraft
	tradeDismissed bool
	comms          *commsDraft
	commsDismissed bool
	paused         bool
	resume         int
}

// New builds the feature over its shell.
func New(sh Shell) *Dealings { return &Dealings{sh: sh} }

func (dl *Dealings) kit() *uikit.Kit { return dl.sh.Kit() }
