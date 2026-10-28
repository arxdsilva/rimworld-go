package devtools

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Tools is the behaviour the debug actions menu needs of the game. It is
// consumer-owned and used only while developer mode is on; nothing else in
// the client touches the game outside commands.
type Tools interface {
	SpawnAnimal(species string, x, y int) (entity.EntityID, error)
	KillAnimal(id entity.EntityID)
	FireIncident(key string) bool
	Raid(n int)
	SpawnStack(kind item.Kind, qty, x, y int) (entity.EntityID, error)
	SetNeeds(id entity.EntityID, hunger, rest int)
	DevLosePart(pawn entity.EntityID, part string) bool
	DevSpawnImplants(near entity.EntityID) bool
}

var _ Tools = (*game.Game)(nil)

// State is developer mode's presentation state.
type State struct {
	Menu   bool // debug actions menu open
	MX, MY int  // where it opened (screen)
	TX, TY int  // the tile under it
	Log    bool // debug log panel
	Insp   bool // debug inspector panel

	Fields FieldsOverlay // the map fields overlay (mapfields.go)
}

// Action is one debug menu entry.
type Action struct {
	Label  string
	Reason string // why it is disabled ("" enabled)
	Run    func(d Tools) string
	Icon   string // ui_icon/<name>, drawn at the row's right
}

// Keys is the frame's fired actions.
type Keys interface {
	Fired(a input.Action) bool
}
