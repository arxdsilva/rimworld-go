// Package gear is the colonist inspector's Gear tab (GEAR-2,
// .cursor/GEAR_GAP_ANALYSIS.md; MAIN_UI_SCREENS_SPECIFICATION.md §16): the
// summary lines (mass carried, the pack, the comfortable range, overall
// armour, move speed, apparel policy), then Equipment, Apparel in layer
// order and the pack, each item a row with its sprite, label, HP, mass and
// tainted mark and the Info, Drop, Lock, Eat and Unload buttons. The list
// scrolls. It reads game.PawnView and emits typed commands and the
// ports.OpenInfoCard intent through Host; it never touches domain state.
package gear

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
)

// Host is what the Gear tab needs of the inspector (consumer-owned).
type Host interface {
	// Enqueue sends DropGear, ConsumeFromPack, SetApparelForced and
	// MarkUnload.
	ports.Commands
	// Do takes the Info button's ports.OpenInfoCard.
	ports.Dispatcher
	// Kit is the widget kit the tab declares into.
	Kit() *uikit.Kit
	// Image is a ui icon by key ("ui_icon/gear_drop"); nil without art.
	Image(key string) *ebiten.Image
	// ItemImage is an item kind's sprite ("parka", "rifle"); nil without art.
	ItemImage(kind string) *ebiten.Image
	// Shift reports Shift is held: Drop queues.
	Shift() bool
	// Unloading reports the colony's manual-unload habit is on (CSE-038).
	Unloading() bool
}

// Facts are the inspector's garment and temperature formatting the tab
// borrows (temp_ui.go, TEMP-5).
type Facts interface {
	// Tr translates the tab's text.
	lang.Tr
	// Temp formats tenths of a degree C in the player's unit.
	Temp(tenths int) string
	// LayerOrder is the indices of worn in layer order, skin out.
	LayerOrder(worn []string) []int
	// LayerTag is a garment's "Skin: " style layer prefix ("" none).
	LayerTag(name string) string
	// Insulation is a garment's insulation line ("" none).
	Insulation(kind, stuff, quality string) string
	// Note is a garment's display line with what it does ("leather parka
	// (much warmer, hot in summer)"); the kind itself when it has none.
	Note(kind string) string
}
