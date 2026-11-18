// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package infocard

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
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

// Kinds is what the card reads of the run's content: the frozen item book
// (a plugin item's label, value, mass and stack limit) and who added a
// definition (SourceOf nil: nobody, PLG-072). It mirrors windows.Env.
type Kinds struct {
	Items    item.Book
	SourceOf func(class, key string) game.Source
}

// Window opens the Info card over a shell.
type Window struct {
	sh Shell
	k  Kinds
}

// New builds the window over its shell and the run's kinds.
func New(sh Shell, k Kinds) *Window { return &Window{sh: sh, k: k} }
