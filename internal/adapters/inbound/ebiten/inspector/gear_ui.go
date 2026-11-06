package inspector

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector/gear"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The Gear tab lives in inspector/gear (GEAR-2); gearHost is its Host and
// Facts over the pane.
type gearHost struct{ p *Pane }

var (
	_ gear.Host  = gearHost{}
	_ gear.Facts = gearHost{}
)

func (h gearHost) Enqueue(c command.Command) { h.p.shell.Enqueue(c) }
func (h gearHost) Do(it ports.Intent)        { h.p.shell.Do(it) }
func (h gearHost) Kit() *uikit.Kit           { return h.p.kit() }
func (h gearHost) Shift() bool               { return h.p.env().Shift }
func (h gearHost) Unloading() bool           { return h.p.shell.World().Gameplay.On(gameplay.ManualUnload) }

func (h gearHost) Image(key string) *ebiten.Image {
	if c := h.p.env().Art; c.Has(key) {
		return c.Image(key, color.NRGBA{})
	}
	return nil
}

func (h gearHost) ItemImage(kind string) *ebiten.Image { return h.Image(world.ItemSprite(kind)) }

func (h gearHost) Temp(tenths int) string         { return h.p.shell.Temp(tenths) }
func (h gearHost) LayerOrder(worn []string) []int { return GearLayerOrder(worn) }
func (h gearHost) LayerTag(name string) string    { return GearLayerTag(name) }
func (h gearHost) Note(kind string) string        { return GearLine(kind) }
func (h gearHost) Insulation(kind, stuff, q string) string {
	return InsulationLine(kind, stuff, q)
}

// gearTabBody declares the Gear tab of pawn id inside r.
func (a Tabs) gearTabBody(id entity.EntityID, r gfx.Rect) {
	if p, ok := a.pawnView(id); ok {
		h := gearHost{a.Pane}
		gear.New(h, h).Declare(p, r)
	}
}
