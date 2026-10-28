package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The New Colony wizard's later screens live in their own files:
// screen_worldgen.go (Create World), screen_landing.go and
// screen_factions.go (Select Landing Site and the Factions dialog),
// screen_pawns.go and portrait.go (Create Characters) and
// screen_loading.go (Start). Their controls are kit widgets, so the
// legacy button lists are empty.

func (a *App) worldGenButtons(sw, sh int) []button { return nil }
func (a *App) landingButtons(sw, sh int) []button  { return nil }
func (a *App) pawnButtons(sw, sh int) []button     { return nil }

// applyWizardClick handles the clicks of the id-driven path (captures and
// tests) that no kit widget answered; it reports id was a wizard one.
func (a *App) applyWizardClick(id string) bool { return a.ngClick(id) }

func (a *App) drawWorldGen(screen *ebiten.Image, sw, sh int) {
	a.drawNGFrame(screen, sw, sh, "Create World")
}

func (a *App) drawPawns(screen *ebiten.Image, sw, sh int) {
	if a.ng != nil && a.ng.Load != nil {
		a.drawLoading(screen, sw, sh)
		return
	}
	a.drawNGFrame(screen, sw, sh, "Create Characters")
}
