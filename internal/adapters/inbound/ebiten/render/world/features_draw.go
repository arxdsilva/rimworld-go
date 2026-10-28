package world

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Map-generation features (MAPGEN-2 §3.4-3.5) are render/env's (the ruin
// overlay and the steam vents' timing). The pipeline keeps the vents'
// draw pass, which shares the generators' exhaust (render/overlay), the
// hiss cue and the steam-puff sprite.

// DrawSteamPuffs draws a puff over every vent in view, unlit like the
// other particles, and the exhaust of running generators (PLAY-021: smoke
// from the wood-fired generator, steam from the geothermal one). A vent
// under a geothermal generator vents through it instead. Reduced Motion
// holds one frame and only fades.
func (lb *Labels) DrawSteamPuffs(screen *ebiten.Image, w game.WorldView, tile float64) {
	if lb.art() == nil {
		return
	}
	v := lb.v
	now := uint64(v.Tick)
	parts := &lb.Overlays.Wiring.Parts
	covers := parts.Covers[:0]
	for i := range w.Buildings {
		if b := &w.Buildings[i]; b.Kind == overlay.GeothermalKind {
			covers = append(covers, [4]int{b.X, b.Y, max(b.W, 1), max(b.H, 1)})
		}
	}
	parts.Covers = covers
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if b.PowerOut > 0 && (b.Kind == overlay.WoodGeneratorKind || b.Kind == overlay.GeothermalKind) {
			lb.Overlays.Wiring.DrawMachinePuff(v, screen, now, b, tile)
			continue
		}
		if b.Kind != env.SteamVentKind || overlay.CoveredVent(covers, b) || !env.OnScreenTile(v, screen, b.X, b.Y, tile) {
			continue
		}
		f, a, ok := env.SteamPuff(now, b.X, b.Y)
		if !ok {
			continue
		}
		lb.SteamHiss(now, b.X, b.Y)
		if v.ReducedMotion {
			f = 1
		}
		sx, sy := lb.worldToScreenF(float64(b.X)+1, float64(b.Y)+1) // footprint centre
		if lb.DrawPuff(screen, f, sx, sy, tile, 1, sprite.ColorScale{1, 1, 1, float32(a)}) && parts.Trace != nil {
			lb.Overlays.Wiring.PowerTrace("vent", overlay.VentTraceKey(f, b.X, b.Y))
		}
	}
}

// DrawPuff draws steam puff frame f with its origin on screen point (sx,
// sy), scaled by k and multiplied by cs (alpha in cs[3]), unlit.
func (lb *Labels) DrawPuff(screen *ebiten.Image, f int, sx, sy float32, tile, k float64, cs sprite.ColorScale) bool {
	key := "effect/steam_puff_" + string(rune('0'+f))
	sp, img := lb.art().Sprite(key), lb.art().Image(key, color.NRGBA{})
	if sp == nil || img == nil {
		return false
	}
	geo := ebiten.GeoM{}
	geo.Translate(-float64(sp.Meta.Origin[0]), -float64(sp.Meta.Origin[1]))
	geo.Scale(tile/art.TilePx*k, tile/art.TilePx*k)
	geo.Translate(float64(sx), float64(sy))
	lb.drawUnlit(screen, img, geo, cs)
	return true
}

// SteamHiss cues the vent's hiss once per burst (the first frame drawn of
// it, whatever the game speed skipped).
func (lb *Labels) SteamHiss(now uint64, x, y int) {
	burst := (now+env.SteamPhase(x, y))/uint64(env.SteamPeriod(x, y)) + 1 // 0: never heard
	k := [2]int{x, y}
	if lb.steamHeard[k] == burst {
		return
	}
	if lb.steamHeard == nil {
		lb.steamHeard = map[[2]int]uint64{}
	}
	lb.steamHeard[k] = burst
	lb.Cues.Cue("sfx.steam_vent", float64(x)+1, float64(y)+1, 1)
}
