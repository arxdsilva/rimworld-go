package world

import (
	"image/color"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
)

// Floor art (AX-3, sprites from the flooring family): every floor-layer
// building kind draws one tileable sprite. Stone tiles and flagstones have
// a sprite per stone, drawn in stone neutrals and multiplied by the
// stone's tint; the carpet floor is one neutral weave multiplied by its
// colour (the palette's `carpet` tints); the other floors draw as they
// are.

// FloorSprite is the world sprite of floor kind in stuff ("" for a kind
// that is not a floor). A wood floor of stone blocks (a read model from
// before save v11) draws as that stone's tile.
func FloorSprite(kind, stuff string) string {
	switch kind {
	case "floor":
		if sprite.StuffMaterial(stuff) != "stone" {
			return "structure/floor_wood"
		}
		return stoneFloorSprite("stone_tile", stuff)
	case "stone tile":
		return stoneFloorSprite("stone_tile", stuff)
	case "flagstone":
		return stoneFloorSprite("flagstone", stuff)
	case "carpet floor":
		return "structure/floor_carpet"
	case "straw matting", "concrete", "paved tile", "steel tile", "silver tile", "gold tile", "sterile tile":
		return "structure/floor_" + strings.ReplaceAll(kind, " ", "_")
	}
	return ""
}

// stoneFloorSprite is structure/floor_<family>_<stone> for stone blocks
// stuff (granite when stuff names no stone).
func stoneFloorSprite(family, stuff string) string {
	s := sprite.StuffStone(stuff)
	if s == "" {
		s = sprite.DefaultStone
	}
	return "structure/floor_" + family + "_" + s
}

// floorTint is the tint a floor of kind in stuff (and colour, for a carpet
// floor) is drawn with; zero draws the sprite as it is.
func (b *base) floorTint(kind, stuff, colour string) color.NRGBA {
	switch kind {
	case "floor", "stone tile", "flagstone":
		return b.stuffTint(stuff) // zero for wood
	case "carpet floor":
		return b.art().Tint("carpet", colour)
	}
	return color.NRGBA{}
}
