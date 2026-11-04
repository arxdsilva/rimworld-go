package dossier

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
)

// BenchmarkDossierDraw is a selected pawn's docked card: a frame is the kit's
// Begin, Declare and Draw between two refreshes, the pointer resting on a bar.
func BenchmarkDossierDraw(b *testing.B) {
	s := newShell(&testing.T{})
	s.set.HUD.RefreshTenths = 30
	img := ebiten.NewImage(int(card.Right())+4, int(card.Bottom())+4)
	s.step(0, 0, false, card, true)
	x, y := s.at(s.find(true, "mood_bar"), card, true)
	s.mx, s.my = x, y
	s.f = inputframe.NewFrame(inputframe.Raw{Mx: x, My: y})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ { // s.n is fixed: the refresh gate stays shut
		s.kit.Begin()
		s.panel.Declare(card)
		s.panel.Draw(img, card)
	}
}

// BenchmarkDossierRefresh is the refresh itself: the facts and every item.
func BenchmarkDossierRefresh(b *testing.B) {
	s := newShell(&testing.T{})
	s.step(0, 0, false, card, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.panel.Invalidate()
		s.step(0, 0, false, card, true)
	}
}

// BenchmarkDossierDeclare is a frame's declaration alone, which is ours to
// keep allocation free (the draw's allocations are Ebitengine's command queue).
func BenchmarkDossierDeclare(b *testing.B) {
	s := newShell(&testing.T{})
	s.set.HUD.RefreshTenths = 30
	s.step(0, 0, false, card, true)
	x, y := s.at(s.find(true, "mood_bar"), card, true)
	s.mx, s.my = x, y
	s.f = inputframe.NewFrame(inputframe.Raw{Mx: x, My: y})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.kit.Begin()
		s.panel.Declare(card)
	}
}
