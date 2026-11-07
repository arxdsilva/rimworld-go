package architect

import (
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/building/construction"
)

// The carpet floor's colour picker (AX-3): one swatch button per carpet
// colour (construction.CarpetColours) in the description box, under the
// material row. The swatch colours are the `carpet` tints of the palette
// (assets/palettes/base.yaml); the picked colour rides DesignateBuild's
// Index.

const (
	archSwatch      = 20 // a colour swatch button, square
	archSwatchInset = 3  // the colour inside the swatch's border
	// archColourPerRow is the swatches per row that fit InnerW.
	archColourPerRow = (InnerW + archSwatchGap) / (archSwatch + archSwatchGap)
	archSwatchGap    = 3
)

// ColourTool reports tool d paints a colour (the carpet floor).
func ColourTool(d ToolDef) bool { return d.Build == construction.CarpetFloor.String() }

// ToolColour is the colour the tool lays a carpet floor in: the player's
// pick, else the first colour.
func (t *Tool) ToolColour() string {
	if _, ok := construction.ParseColour(t.Colour); ok {
		return t.Colour
	}
	return construction.CarpetColours[0]
}

// ColourIndex is the DesignateBuild Index of carpet colour name (0 for
// an unknown name: the first colour).
func ColourIndex(name string) int {
	i, _ := construction.ParseColour(name)
	return int(i)
}

// ColourLabel is the picked colour's line: "Colour: Powder blue".
func ColourLabel(name string) string { return "Colour: " + StuffLabel(name) }

// colourRows is how many rows the swatches take.
func colourRows() int {
	return (len(construction.CarpetColours) + archColourPerRow - 1) / archColourPerRow
}

// colourButtons are the swatches inside description box r, from top.
func (t *Tool) colourButtons(r gfx.Rect, top float32) []uikit.Button {
	sel := t.ToolColour()
	out := make([]uikit.Button, 0, len(construction.CarpetColours))
	for i, name := range construction.CarpetColours {
		row, col := i/archColourPerRow, i%archColourPerRow
		out = append(out, uikit.Button{
			ID: ids.PrefixArchColour + name, Tip: StuffLabel(name),
			X: r.X + archPad + float32(col)*(archSwatch+archSwatchGap),
			Y: r.Y + top + float32(row)*(archSwatch+archSwatchGap),
			W: archSwatch, H: archSwatch, Selected: name == sel,
		})
	}
	return out
}

// drawSwatch fills colour button b with its carpet tint (nothing for any
// other button, or while the palette is missing).
func (t *Tool) drawSwatch(screen *ebiten.Image, b uikit.Button) {
	name, ok := strings.CutPrefix(b.ID, ids.PrefixArchColour)
	if !ok || t.sh.Env().Art == nil {
		return
	}
	c := t.sh.Env().Art.Tint("carpet", name)
	if c == (color.NRGBA{}) {
		return
	}
	gfx.FillRect(screen, b.X+archSwatchInset, b.Y+archSwatchInset, b.W-2*archSwatchInset, b.H-2*archSwatchInset, c, false)
}

// paintedSame is the cells where a carpet floor of the picked colour
// already lies, while tool d lays carpet floors (nil otherwise): laying
// the same colour again would change nothing, so the ghost shows them as
// the same piece and the drag skips them. Another colour repaints the
// floor (the sim swaps it in place, AX-3).
func (t *Tool) paintedSame(d ToolDef) map[[2]int]bool {
	if !ColourTool(d) {
		return nil
	}
	want := t.ToolColour()
	var out map[[2]int]bool
	for _, b := range t.sh.World().Buildings {
		if b.Kind == d.Build && b.AX.Colour == want {
			if out == nil {
				out = map[[2]int]bool{}
			}
			out[[2]int{b.X, b.Y}] = true
		}
	}
	return out
}
