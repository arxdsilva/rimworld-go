// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dialogs

import (
	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeShell is a headless shell over a real kit.
type fakeShell struct {
	kit   *uikit.Kit
	f     inputframe.Frame
	cmds  []command.Command
	tier  int
	tv    *game.TradeView
	cv    *game.CommsView
	sites []game.WorldSiteView
	pawns []game.PawnView
	items item.Book
	cur   [2]int // the pointer, logical pixels
	mark  bool   // T prefixes "~" (which strings were translated)
	// est is what FormEstimate answers (ticks, why) and how often it was asked.
	est struct {
		ticks int
		why   string
		calls int
	}

	w         game.WorldView
	portraits int
	icons     []string
}

func newFake() (*fakeShell, *Dealings) {
	s := &fakeShell{tier: 2, cur: [2]int{-1, -1}}
	s.kit = uikit.New(fakeHost{s})
	return s, New(s)
}

type fakeHost struct{ s *fakeShell }

func (h fakeHost) Frame() *inputframe.Frame { return &h.s.f }
func (h fakeHost) Cursor() (int, int)       { return h.s.cur[0], h.s.cur[1] }
func (h fakeHost) Size() (int, int)         { return 1280, 720 }
func (h fakeHost) Updates() uint64          { return 0 }
func (h fakeHost) KeyRepeat(string) bool    { return false }
func (h fakeHost) Cue(string)               {}
func (h fakeHost) Hint(string)              {}
func (h fakeHost) ReducedMotion() bool      { return true }

func (s *fakeShell) Enqueue(c command.Command) { s.cmds = append(s.cmds, c) }
func (s *fakeShell) World() game.WorldView     { return s.w }
func (s *fakeShell) Pawns() []game.PawnView    { return s.pawns }
func (s *fakeShell) MapSize() (int, int)       { return 0, 0 }
func (s *fakeShell) Tick() clock.Tick          { return 0 }
func (s *fakeShell) Date() clock.Date          { return clock.Date{} }
func (s *fakeShell) Paused() bool              { return s.tier == 0 }
func (s *fakeShell) TogglePause()              {}
func (s *fakeShell) SetSpeed(t int)            { s.tier = t }
func (s *fakeShell) SpeedTier() int            { return s.tier }
func (s *fakeShell) PauseReason() string       { return "" }
func (s *fakeShell) HoldSpeed(t int)           { s.tier = t }
func (s *fakeShell) DrawPortrait(*ebiten.Image, look.Look, float64, float64, float64, float64) bool {
	s.portraits++
	return false
}
func (s *fakeShell) Icon(key string) *ebiten.Image {
	s.icons = append(s.icons, key)
	return nil
}
func (s *fakeShell) Kit() *uikit.Kit                 { return s.kit }
func (s *fakeShell) Items() item.Book                { return s.items }
func (s *fakeShell) Caravans() (CaravanSource, bool) { return fakeCaravans{s}, true }
func (s *fakeShell) Trade() (game.TradeView, bool)   { return opt(s.tv) }
func (s *fakeShell) Comms() (game.CommsView, bool)   { return opt(s.cv) }

// fakeCaravans is the shell's CaravanSource: its sites and its est.
type fakeCaravans struct{ s *fakeShell }

func (c fakeCaravans) WorldSites() []game.WorldSiteView { return c.s.sites }
func (c fakeCaravans) FormEstimate([]entity.EntityID, int, int32) (int, string) {
	c.s.est.calls++
	return c.s.est.ticks, c.s.est.why
}
