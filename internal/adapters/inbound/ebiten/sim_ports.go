package ebiten

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The simulation as the root sees it, in the groups its consumers use
// (ARCH-006, SOLID_GO.md §6): each is small and single-purpose; Simulation
// (renderer.go) is their union for the composition root only.

// SimStepper advances the colony.
type SimStepper interface {
	Step(ticks int)
}

// SimCommander takes the player's typed commands.
type SimCommander interface {
	Enqueue(command.Command)
}

// SimFrame is the colony's clock and map size.
type SimFrame interface {
	Tick() clock.Tick
	Date() clock.Date
	Activity() schedule.Activity
	Width() int
	Height() int
}

// SimWorld is the frame's read models and the one per-tile query the HUD
// follows the cursor with.
type SimWorld interface {
	Snapshot() []game.PawnView
	World() game.WorldView
	// TemperatureAt is the air temperature on a tile, in tenths of a degree C
	// (rooms, weather, vents, fire): the lower-right temperature line
	// follows the cursor with it (PLAY-018).
	TemperatureAt(x, y int) int
}

// SimPlacement is the read-only placement and order rules the Architect
// and the order menu preview; the adapter never re-implements them.
type SimPlacement interface {
	// CanPlaceAt is the placement rule the Architect previews (material,
	// facing and every footprint cell).
	CanPlaceAt(kind, stuff, facing string, x, y int) (bool, string)
	// ReplacesAt names the wall or fence a door or gate would replace
	// there ("" when none): the ghost reads "replaces wall".
	ReplacesAt(kind, stuff string, x, y int) string
	// AutoFacingAt is the facing the sim would give a piece placed here
	// (doors and gates face along their walls, PLAY-011).
	AutoFacingAt(kind, facing string, x, y int) string
	// OrdersAt lists what a pawn can be ordered to do at a tile.
	OrdersAt(pawn entity.EntityID, x, y int) []game.OrderOption
	// CanZoneAt reports whether a zone of kind ("stockpile", "grow",
	// "pen") could claim a tile: the zone drag ghost per cell.
	CanZoneAt(kind string, x, y int) bool
}

// SimColony is the colony-wide read models of the windows and the wizard:
// the wild species, the factions, the storyteller, a colonist's Bio, the
// building catalog with the scenario's locks, the Work tab's "Incapable:
// <why>", the Bio tab's weekly experience and a plant's growth.
type SimColony interface {
	Wildlife() []game.WildlifeRow
	Factions() []game.FactionView
	Story() game.StoryView
	Bio(id entity.EntityID) (game.BioView, bool)
	Buildables() []game.BuildableView
	IncapableReason(id entity.EntityID, t work.Type) string
	LearnedThisWeek(id entity.EntityID) [skill.NumSkills]int
	PlantGrowth(id entity.EntityID) (game.PlantGrowthView, bool)
}

// SimTabs is the ENG-003 social and host-system read models: a colonist's
// Social and Health tabs, a prisoner's tab (ok false: nothing to show),
// the World tab, the allowed areas and the colony History.
type SimTabs interface {
	Social(id entity.EntityID) (game.SocialView, bool)
	Health(id entity.EntityID) (game.HealthView, bool)
	Prisoner(id entity.EntityID) (game.PrisonerView, bool)
	WorldMap() game.WorldMapView
	Areas() []game.AreaView
	History() []game.HistoryEntryView
}

// SimChunks is the renderer's read models (NG-1 E5): a chunk's share of
// World() and its bake versions, the plants in a tile rectangle, and the
// minimap layer when it changed since ver.
type SimChunks interface {
	ChunkLayer(cx, cy int) game.ChunkView
	PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32
	MinimapLayer(ver uint64) (game.MinimapView, bool)
}
