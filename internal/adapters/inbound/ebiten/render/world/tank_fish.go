package world

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// FishTankKind is the fish tank (FISH): drawn as a basin is, plus the stock
// as an overlay that grows with the stage.
const FishTankKind = "fish tank"

// tankFishKeys[stage-1][facing] are the overlay keys, built once so a frame
// allocates nothing.
var tankFishKeys = func() (k [3][4]string) {
	for s := range k {
		for f, name := range [4]string{"south", "north", "east", "west"} {
			k[s][f] = "furniture/fish_tank_fish_" + string(rune('1'+s)) + "_" + name
		}
	}
	return k
}()

// TankFishStage is the overlay stage of a tank: 0 when nothing is stocked,
// else 1..3 by growth (<34, <67, the rest).
func TankFishStage(t *game.TankView) int {
	switch {
	case t == nil || !t.Stocked:
		return 0
	case t.GrowthPct < 34:
		return 1
	case t.GrowthPct < 67:
		return 2
	}
	return 3
}

// TankFishKey is the overlay sprite key of a tank at a stage and facing, ""
// for no stock.
func TankFishKey(stage int, facing string) string {
	if stage < 1 || stage > 3 {
		return ""
	}
	f := 0
	switch facing {
	case "north":
		f = 1
	case "east":
		f = 2
	case "west":
		f = 3
	}
	return tankFishKeys[stage-1][f]
}

// drawTankFish draws a stocked tank's fish over its sprite and glow, lit.
func (o *Objects) drawTankFish(dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	if b.Kind != FishTankKind {
		return
	}
	if key := TankFishKey(TankFishStage(b.Tank), b.Facing); key != "" && o.art().Has(key) {
		o.drawSprite(dst, key, color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
	}
}
