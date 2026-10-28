package ebiten

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Map-generation features (MAPGEN-2 §3.4-3.5) are render/env's
// (features_draw.go there: the ruin overlay and the steam vents' timing).
// The root keeps the vents' draw pass, which shares the generators'
// exhaust, the hiss cue and the steam-puff sprite with the power overlay
// and the sound director, and the ruin title (stone_art.go's adjectives).

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
	covers := r.overlays().Wiring.Parts.Covers[:0]
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.Kind == geothermalKind {
			covers = append(covers, [4]int{b.X, b.Y, max(b.W, 1), max(b.H, 1)})
		}
	}
	r.overlays().Wiring.Parts.Covers = covers
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
		if r.drawPuff(screen, f, sx, sy, tile, 1, colorScale{1, 1, 1, float32(a)}) && r.overlays().Wiring.Parts.Trace != nil {
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

// ruinTitle is "Ruined granite wall" / "Ruined wooden floor".
func ruinTitle(b game.BuildingView) string {
	adj := strings.ToLower(stuffAdjective(b.Stuff))
	if adj == "" {
		adj = "wooden"
	}
	return "Ruined " + adj + " " + b.Kind
}
