package env

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Map-generation features (MAPGEN-2 §3.4-3.5): the ruin overlay on
// abandoned walls and the steam vent's puffs. Presentation only: timing
// comes from the simulation tick as a clock and per-vent position hashes,
// never from the sim's state.

const (
	SteamVentKind = "steam vent"
	// steam bursts: four frames of SteamFrameTicks (150 ms) each, repeating
	// every SteamPeriodMin..+SteamPeriodSpread ticks, offset by position.
	SteamFrameTicks   = 9
	SteamFrames       = 4
	SteamPeriodMin    = 60 * 4
	SteamPeriodSpread = 60 * 3
	steamAlpha        = 0.85
)

func steamHash(x, y int) uint64 { return art.Mix64(uint64(uint32(x))<<32 | uint64(uint32(y)) ^ 0x57e4) }

// SteamPeriod is the ticks between this vent's bursts.
func SteamPeriod(x, y int) int {
	return SteamPeriodMin + int(steamHash(x, y)>>8%(SteamPeriodSpread+1))
}

// SteamPhase is the vent's offset into its period, in ticks.
func SteamPhase(x, y int) uint64 {
	return steamHash(x, y) >> 24 % uint64(SteamPeriod(x, y))
}

// SteamPuff is the puff frame (0..3) and alpha of the vent at (x, y) at
// tick now; ok is false between bursts. The burst fades out as it plays.
func SteamPuff(now uint64, x, y int) (frame int, alpha float64, ok bool) {
	period := uint64(SteamPeriod(x, y))
	return PuffAt((now + SteamPhase(x, y)) % period)
}

// PuffAt is the puff frame and alpha t ticks into a period (ok false once
// the four frames have played): the vents' and the generators' exhaust
// (power_draw.go machinePuff).
func PuffAt(t uint64) (frame int, alpha float64, ok bool) {
	if t >= SteamFrameTicks*SteamFrames {
		return 0, 0, false
	}
	return int(t / SteamFrameTicks), steamAlpha * (1 - float64(t)/(SteamFrameTicks*SteamFrames)), true
}

// OnScreenTile reports whether a tile near (x, y) may be visible; generous
// (a vent's puff rises about three tiles).
func OnScreenTile(v *render.View, screen *ebiten.Image, x, y int, tile float64) bool {
	sx, sy := v.WorldToScreenF(float64(x), float64(y))
	m := float64(tile) * 5
	b := screen.Bounds()
	return float64(sx) > -m && float64(sy) > -m && float64(sx) < float64(b.Dx())+m && float64(sy) < float64(b.Dy())+m*2
}

// RuinOverlayKey is the cracks overlay variant for a wall cell:
// hash(x, y) mod 3, stable per cell.
func RuinOverlayKey(x, y int) string {
	return "structure/wall_ruin_overlay_" + string(rune('0'+steamHash(x+991, y)%3))
}

// WantsRuinOverlay reports whether b gets the ruin overlay: ruined walls only.
func WantsRuinOverlay(b game.BuildingView) bool { return b.Kind == "wall" && b.Ruin }

// DrawRuinOverlay draws the damage overlay right after a ruined wall, in
// sprite pass ps.
func (e *Layer) DrawRuinOverlay(v *render.View, ps sprite.Pass, dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	if WantsRuinOverlay(*b) {
		e.P.Draw(v, ps, dst, RuinOverlayKey(b.X, b.Y), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
	}
}

// VentLine is the inspector blurb of a steam vent; "" for other buildings.
func VentLine(b game.BuildingView) string {
	if b.Kind != SteamVentKind {
		return ""
	}
	return "Warms the air around it. A natural feature: it can't be removed."
}
