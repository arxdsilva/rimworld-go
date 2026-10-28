// Package mainmenu is the main menu screen: the hero splash and the colony
// diorama behind it, the button rail, Continue/Load/New/Options/Credits/Quit,
// the credits text and the RAM notice. It reaches the shell through a small
// interface and the simulation not at all.
package mainmenu

import (
	"errors"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
)

// ErrNoLoader is what a Host's loaders return when the program was built
// without that loader (the screen then tries again later).
var ErrNoLoader = errors.New("mainmenu: no loader")

// Host is what the menu needs of the program around it that no ports role
// carries: the art loaders, the version line, the menu's animation clock,
// whether Continue has a save, the low-memory notice and the flame the
// colony draws.
type Host interface {
	// LoadSplash reads the hero illustration (ErrNoLoader when unset).
	LoadSplash() (*ebiten.Image, error)
	// LoadMenuScene reads the diorama (ErrNoLoader when unset).
	LoadMenuScene() (*Diorama, error)
	// Version is the build's version string.
	Version() string
	// MenuTick drives the menu's motion; it counts while no colony shows.
	MenuTick() uint64
	// HasSaves reports a loadable save exists (Continue is enabled).
	HasSaves() bool
	// LowRAMNotice is the notice to show now and the memory it is for
	// ("" when none).
	LowRAMNotice() (text string, total uint64)
	// DismissRAMNotice remembers the notice was seen for total bytes.
	DismissRAMNotice(total uint64)
	// DrawFlame draws the campfire flame at a screen point.
	DrawFlame(dst *ebiten.Image, sx, sy float32, t, tile, k float64)
}

// Shell composes the roles the menu calls with its Host.
type Shell interface {
	ports.Sizer
	ports.Settings
	ports.Widgets
	Host
}
