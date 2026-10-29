package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"

	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/skill"
)

func (a *App) windowAnchor() gfx.Rect { return a.win.Anchor() }

// workColSkill mirrors windows.WorkColSkill in its old field names for
// social_tabs_ui_test.go.
var workColSkill = func() (out [len(windows.WorkColSkill)]struct {
	sk skill.ID
	ok bool
}) {
	for i, c := range windows.WorkColSkill {
		out[i].sk, out[i].ok = c.Skill, c.OK
	}
	return out
}()

// animalAreaCell is the Area / Pen column's text over the areas.
func animalAreaCell(an game.AnimalView, areas []game.AreaView) string {
	return windows.AnimalAreaCell(an, architect.AreaRefLabel(areas, an.AreaRef))
}

// animalRow is one line of the Animals tab (animal_species_test.go).
func animalRow(an game.AnimalView) string { return windows.AnimalRow(an, inspector.AnimalLabel(an)) }
