package infocard

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
)

// Reader is the read model the card is built from.
type Reader interface {
	lang.Tr
	Pawns() []game.PawnView
	World() game.WorldView
}

// Shell composes the roles the window calls: the read models and the kit
// (the card is a kit modal, so Esc and the close button are the kit's).
type Shell interface {
	ports.World
	ports.Widgets
	ports.Texts
}

// Window opens the Info card over a shell.
type Window struct{ sh Shell }

// New builds the window over its shell.
func New(sh Shell) *Window { return &Window{sh: sh} }
