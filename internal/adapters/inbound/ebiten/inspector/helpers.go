package inspector

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Small helpers the inspector's files share.

// bedRowH is a row of the bed owner picker (the Assign window's row).
const bedRowH = 30

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// drawIconCS draws img scaled to size x size at the logical point (x, y),
// multiplied by cs (a stone item's tint, a disabled command's grey).
func drawIconCS(dst, img *ebiten.Image, x, y, size float64, cs sprite.ColorScale) {
	if img == nil || img.Bounds().Dx() == 0 {
		return
	}
	var op ebiten.DrawImageOptions
	op.Filter = ebiten.FilterLinear
	if cs != sprite.NoCS {
		op.ColorScale.Scale(cs[0], cs[1], cs[2], cs[3])
	}
	b := img.Bounds()
	gfx.Image(dst, img, x, y, size, size*float64(b.Dy())/float64(b.Dx()), &op)
}

// icon is the ui_icon called name (nil without art or with no such icon).
func (a Commands) icon(name string) *ebiten.Image {
	c := a.env().Art
	if c == nil {
		return nil
	}
	return c.Image("ui_icon/"+name, color.NRGBA{})
}

// modalTop is the id of the top stacked modal ("" none).
func (a Building) modalTop() string {
	if k := a.kit(); k.ModalOpen() {
		return k.Top()
	}
	return ""
}

// stuffAdjective names a non-wood material for titles: "Granite",
// "Steel", "Stone" (legacy); "" for wood.
func stuffAdjective(stuff string) string {
	if s := sprite.StuffStone(stuff); s != "" {
		if stuff == "stone" {
			return "Stone"
		}
		return Capitalize(s)
	}
	if stuff == "steel" {
		return "Steel"
	}
	return ""
}

// materialTitle is "Granite wall" / "Steel door" / "Wall".
func materialTitle(kind, stuff string) string {
	if adj := stuffAdjective(stuff); adj != "" {
		return adj + " " + kind
	}
	return Capitalize(kind)
}

// ruinTitle is "Ruined granite wall" / "Ruined wooden floor".
func ruinTitle(b game.BuildingView) string {
	adj := strings.ToLower(stuffAdjective(b.Stuff))
	if adj == "" {
		adj = "wooden"
	}
	return "Ruined " + adj + " " + b.Kind
}
