package ebiten

import "rimworld-go/internal/domain/sim/entity"

// Buildings' and blueprints' commands and panes (PLAY-018: BUILD-INSPECT
// owns them).

func (a *App) buildingCommands(id entity.EntityID) []cmdButton { return nil }
func (a *App) buildingPane(id entity.EntityID) paneSpec        { return paneSpec{} }

func (a *App) frameCommands(id entity.EntityID) []cmdButton { return nil }
func (a *App) framePane(id entity.EntityID) paneSpec        { return paneSpec{} }
