package ebiten

import "rimworld-go/internal/domain/sim/entity"

// Animals', items' and plants' commands and panes (PLAY-018:
// THING-INSPECT owns them).

func (a *App) animalCommands(id entity.EntityID) []cmdButton { return nil }
func (a *App) animalPane(id entity.EntityID) paneSpec        { return paneSpec{} }

func (a *App) itemCommands(id entity.EntityID) []cmdButton { return nil }
func (a *App) itemPane(id entity.EntityID) paneSpec        { return paneSpec{} }

func (a *App) plantCommands(id entity.EntityID) []cmdButton { return nil }
func (a *App) plantPane(id entity.EntityID) paneSpec        { return paneSpec{} }
