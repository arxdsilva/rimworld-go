package ebiten

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Map-generation features (MAPGEN-2 §3.4-3.5): the ruin overlay on
// abandoned walls and the steam vent's puffs. Presentation only: timing
// comes from the simulation tick as a clock and per-vent position hashes,
// never from the sim's state.

const (
	steamVentKind = "steam vent"
	// steam bursts: four frames of steamFrameTicks (150 ms) each, repeating
	// every steamPeriodMin..+steamPeriodSpread ticks, offset by position.
	steamFrameTicks   = 9
	steamFrames       = 4
	steamPeriodMin    = 60 * 4
	steamPeriodSpread = 60 * 3
	steamAlpha        = 0.85
)

func steamHash(x, y int) uint64 { return mix64(uint64(uint32(x))<<32 | uint64(uint32(y)) ^ 0x57e4) }

// steamPeriod is the ticks between this vent's bursts.
func steamPeriod(x, y int) int {
	return steamPeriodMin + int(steamHash(x, y)>>8%(steamPeriodSpread+1))
}

// steamPhase is the vent's offset into its period, in ticks.
func steamPhase(x, y int) uint64 {
	return steamHash(x, y) >> 24 % uint64(steamPeriod(x, y))
}

// steamPuff is the puff frame (0..3) and alpha of the vent at (x, y) at
// tick now; ok is false between bursts. The burst fades out as it plays.
func steamPuff(now uint64, x, y int) (frame int, alpha float64, ok bool) {
	period := uint64(steamPeriod(x, y))
	return puffAt((now + steamPhase(x, y)) % period)
}

// puffAt is the puff frame and alpha t ticks into a period (ok false once
// the four frames have played): the vents' and the generators' exhaust
// (power_draw.go machinePuff).
func puffAt(t uint64) (frame int, alpha float64, ok bool) {
	if t >= steamFrameTicks*steamFrames {
		return 0, 0, false
	}
	return int(t / steamFrameTicks), steamAlpha * (1 - float64(t)/(steamFrameTicks*steamFrames)), true
}

// drawSteamPuffs draws a puff over every vent in view, unlit like the other
// particles, and the exhaust of running generators (PLAY-021: smoke from
// the wood-fired generator, steam from the geothermal one, power_draw.go).
// A vent under a geothermal generator vents through it instead. Reduced
// Motion holds one frame and only fades.
func (r *Renderer) drawSteamPuffs(screen *ebiten.Image, w game.WorldView, tile float64) {
	if r.art == nil {
		return
	}
	now := uint64(r.game.Tick())
	covers := r.power.Covers[:0]
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.Kind == geothermalKind {
			covers = append(covers, [4]int{b.X, b.Y, max(b.W, 1), max(b.H, 1)})
		}
	}
	r.power.Covers = covers
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.PowerOut > 0 && (b.Kind == woodGeneratorKind || b.Kind == geothermalKind) {
			r.drawMachinePuff(screen, now, b, tile)
			continue
		}
		if b.Kind != steamVentKind || coveredVent(covers, b) || !r.onScreenTile(screen, b.X, b.Y, tile) {
			continue
		}
		f, a, ok := steamPuff(now, b.X, b.Y)
		if !ok {
			continue
		}
		r.steamHiss(now, b.X, b.Y)
		if r.reducedMotion {
			f = 1
		}
		sx, sy := r.worldToScreenF(float64(b.X)+1, float64(b.Y)+1) // footprint centre
		if r.drawPuff(screen, f, sx, sy, tile, 1, colorScale{1, 1, 1, float32(a)}) && r.power.Trace != nil {
			r.powerTrace("vent", ventTraceKey(f, b.X, b.Y))
		}
	}
}

// drawPuff draws steam puff frame f with its origin on screen point (sx,
// sy), scaled by k and multiplied by cs (alpha in cs[3]), unlit.
func (r *Renderer) drawPuff(screen *ebiten.Image, f int, sx, sy float32, tile, k float64, cs colorScale) bool {
	key := "effect/steam_puff_" + string(rune('0'+f))
	sp, img := r.art.Sprite(key), r.art.Image(key, color.NRGBA{})
	if sp == nil || img == nil {
		return false
	}
	geo := ebiten.GeoM{}
	geo.Translate(-float64(sp.Meta.Origin[0]), -float64(sp.Meta.Origin[1]))
	geo.Scale(tile/artTilePx*k, tile/artTilePx*k)
	geo.Translate(float64(sx), float64(sy))
	r.drawUnlit(screen, img, geo, cs)
	return true
}

// steamHiss cues the vent's hiss once per burst (the first frame drawn of
// it, whatever the game speed skipped).
func (r *Renderer) steamHiss(now uint64, x, y int) {
	burst := (now+steamPhase(x, y))/uint64(steamPeriod(x, y)) + 1 // 0: never heard
	k := [2]int{x, y}
	if r.steamHeard[k] == burst {
		return
	}
	if r.steamHeard == nil {
		r.steamHeard = map[[2]int]uint64{}
	}
	r.steamHeard[k] = burst
	r.cue("sfx.steam_vent", float64(x)+1, float64(y)+1, 1)
}

// onScreenTile reports whether a tile near (x, y) may be visible; generous
// (a vent's puff rises about three tiles).
func (r *Renderer) onScreenTile(screen *ebiten.Image, x, y int, tile float64) bool {
	sx, sy := r.worldToScreenF(float64(x), float64(y))
	m := float64(tile) * 5
	b := screen.Bounds()
	return float64(sx) > -m && float64(sy) > -m && float64(sx) < float64(b.Dx())+m && float64(sy) < float64(b.Dy())+m*2
}

// ruinOverlayKey is the cracks overlay variant for a wall cell:
// hash(x, y) mod 3, stable per cell.
func ruinOverlayKey(x, y int) string {
	return "structure/wall_ruin_overlay_" + string(rune('0'+steamHash(x+991, y)%3))
}

// wantsRuinOverlay reports whether b gets the ruin overlay: ruined walls only.
func wantsRuinOverlay(b game.BuildingView) bool { return b.Kind == "wall" && b.Ruin }

// drawRuinOverlay draws the damage overlay right after a ruined wall.
func (r *Renderer) drawRuinOverlay(dst *ebiten.Image, b *game.BuildingView, tile float64, sw, sh int) {
	if wantsRuinOverlay(*b) {
		r.drawSprite(dst, ruinOverlayKey(b.X, b.Y), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
	}
}

// ruinTitle is "Ruined granite wall" / "Ruined wooden floor".
func ruinTitle(b game.BuildingView) string {
	adj := strings.ToLower(stuffAdjective(b.Stuff))
	if adj == "" {
		adj = "wooden"
	}
	return "Ruined " + adj + " " + b.Kind
}

// ventLine is the inspector blurb of a steam vent; "" for other buildings.
func ventLine(b game.BuildingView) string {
	if b.Kind != steamVentKind {
		return ""
	}
	return "Warms the air around it. A natural feature: it can't be removed."
}
