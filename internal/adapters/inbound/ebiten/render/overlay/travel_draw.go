package overlay

import (
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// The travel buildings' extra layers (AX-9, AR4's transit family): an
// occupied stasis casket's glow, and a pod launcher's chemfuel gauge in
// its fuel slot (the battery's scheme: gauge0 empty, gauge1 full).

// Travel kinds the power pass draws parts for (construction.go names).
const (
	StasisCasketKind = "stasis casket"
	PodLauncherKind  = "pod launcher"
	// LauncherFuelCap is a launcher's full tank (wiki: Pod launcher).
	LauncherFuelCap = 150
)

// travelPart reports a travel kind with a layer drawn by DrawPowerParts.
func travelPart(kind string) bool { return kind == StasisCasketKind || kind == PodLauncherKind }

// LauncherFill is launcher b's chemfuel, 0..1.
func LauncherFill(b *game.BuildingView) float64 {
	return math.Min(math.Max(float64(b.AX.FuelUnits)/LauncherFuelCap, 0), 1)
}

// drawLauncherBar draws launcher b's fuel between its gauge attachments,
// unlit (an indicator).
func (o *Wiring) drawLauncherBar(v *render.View, dst *ebiten.Image, b *game.BuildingView, key string, tile float64) {
	f := LauncherFill(b)
	if f <= 0 {
		return
	}
	x0, y0, ok0 := attachWorld(v.Art, key, "gauge0", float64(b.X), float64(b.Y))
	x1, y1, ok1 := attachWorld(v.Art, key, "gauge1", float64(b.X), float64(b.Y))
	if !ok0 || !ok1 {
		return
	}
	sx0, sy0 := v.WorldToScreenF(x0, y0)
	sx1, sy1 := v.WorldToScreenF(x1, y1)
	x, y, w, h := BatteryBarRect(float64(sx0), float64(sy0), float64(sx1), float64(sy1), f, batteryBarPx*tile/art.TilePx)
	o.PowerTrace("launcher", key)
	o.solidRect(dst, float32(x), float32(y), float32(w), float32(h), BatteryBarColour(f))
}
