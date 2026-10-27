package ebiten

import (
	"fmt"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Sentry turret, radio console and trade beacon on the map (PLAY-024,
// ART §38). Presentation only: the gun frame comes from
// BuildingView.AimDir, the recoil and flash from ShotAge, the glow from
// the shared power look (power_draw.go). The beacon's blink is a renderer
// phase on r.present, never sim state.
const (
	sentryTurretKind = "sentry turret"
	tradeBeaconKind  = "trade beacon"
	radioConsoleKind = "radio console"

	turretAimFrames = 16
	turretStepDeg   = 22.5
	// turretRecoilTicks is how long after a shot the gun sits pushed back,
	// and turretRecoilTiles how far at the first tick.
	turretRecoilTicks = 6
	turretRecoilTiles = 0.07
	// turretRangeTiles and beaconRadiusTiles are the rings shown on
	// selection and on the placement ghost (mirror the tuning table).
	turretRangeTiles  = 10.0
	beaconRadiusTiles = 6.0
	// turretEngagedTicks: a shot this recent means the turret holds a target.
	turretEngagedTicks = 90
	beaconBlinkFrames  = 90
	beaconOnFrames     = 50
)

// turretBaseKey is the turret base sprite.
const turretBaseKey = "defense/sentry_turret_base"

// turretGunKey is gun frame i (0..15) of the turret.
func turretGunKey(i int) string { return fmt.Sprintf("defense/sentry_turret_gun_%02d", i) }

// aimFrame clamps a view's aim to 0..15.
func aimFrame(d int8) int { return ((int(d) % turretAimFrames) + turretAimFrames) % turretAimFrames }

// turretAimVec is the unit screen-space direction of aim frame i: frame 0
// points south, counting clockwise on screen.
func turretAimVec(i int) vec {
	th := float64(i) * turretStepDeg * math.Pi / 180
	return vec{-math.Sin(th), math.Cos(th)}
}

// turretGunAt is where turret b's gun frame is drawn (its origin, in
// world tiles): the frame's mount attachment on the base's mount.
func turretGunAt(c *artCatalog, b *game.BuildingView) (key string, wx, wy float64, ok bool) {
	mx, my, ok := attachWorld(c, turretBaseKey, "mount", float64(b.X), float64(b.Y))
	if !ok {
		return "", 0, 0, false
	}
	key = turretGunKey(aimFrame(b.AimDir))
	g := c.sprites[key]
	if g == nil {
		return "", 0, 0, false
	}
	gm, ok := g.meta.Attach["mount"]
	if !ok {
		return "", 0, 0, false
	}
	return key, mx - (gm[0]-g.meta.Origin[0])/artTilePx, my - (gm[1]-g.meta.Origin[1])/artTilePx, true
}

// turretMuzzle is the muzzle of turret b's gun in world tiles (ok false
// without the art).
func turretMuzzle(c *artCatalog, b *game.BuildingView) (vec, bool) {
	key, wx, wy, ok := turretGunAt(c, b)
	if !ok {
		return vec{}, false
	}
	x, y, ok := attachWorld(c, key, "muzzle", wx, wy)
	return vec{x, y}, ok
}

// recoilPush is how far back (tiles) the gun sits shotAge ticks after a
// shot: 0 before the first shot and once it has settled.
func recoilPush(shotAge int) float64 {
	if shotAge < 0 || shotAge >= turretRecoilTicks {
		return 0
	}
	return turretRecoilTiles * (1 - float64(shotAge)/turretRecoilTicks)
}

// buildingByID finds a drawn building by id (traces name a turret by it).
func (r *Renderer) buildingByID(id entity.EntityID) (*game.BuildingView, bool) {
	for i := range r.drawW.Buildings {
		if r.drawW.Buildings[i].ID == id {
			return &r.drawW.Buildings[i], true
		}
	}
	return nil, false
}

// beaconLit is the beacon light's blink phase on the present clock.
func beaconLit(present uint64, id entity.EntityID, reduced bool) bool {
	if reduced {
		return true
	}
	return (present+uint64(id)*17)%beaconBlinkFrames < beaconOnFrames
}

// drawDefenseParts draws what the turret and the beacon add over their
// base sprite (drawn just before): the turret's gun on its aim frame,
// pushed back by the recoil, and the beacon's blinking light. A turret's
// and a console's status glow come from drawPowerParts.
func (r *Renderer) drawDefenseParts(dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	if r.lod.baking || r.art == nil {
		return
	}
	switch b.Kind {
	case sentryTurretKind:
		gk, wx, wy, ok := turretGunAt(r.art, b)
		if !ok {
			return
		}
		if !r.reducedMotion {
			if p := recoilPush(b.ShotAge); p > 0 {
				d := turretAimVec(aimFrame(b.AimDir))
				wx, wy = wx-d.x*p, wy-d.y*p
			}
		}
		r.powerTrace("gun", gk)
		r.drawSpriteCS(dst, gk, color.NRGBA{}, wx, wy, tile, sw, sh, noCS)
	case tradeBeaconKind:
		g := r.power.glowOf(key)
		if r.art.has(g) && beaconLit(r.present, b.ID, r.reducedMotion) &&
			r.drawUnlitSprite(dst, g, float64(b.X), float64(b.Y), tile, sw, sh, noCS) {
			r.powerTrace("glow", g)
		}
	}
}

// drawRangeRing strokes a circle of radius tiles around the centre of
// cell (x, y): the turret's reach when selected, the beacon's pickup
// radius on its placement ghost.
func (r *Renderer) drawRangeRing(dst *ebiten.Image, x, y int, radius float64, c color.NRGBA) {
	cx, cy := r.worldToScreenF(float64(x)+0.5, float64(y)+0.5)
	t := r.tilePx()
	r.powerTrace("ring", fmt.Sprintf("%d,%d,%g", x, y, radius))
	vector.StrokeCircle(dst, cx, cy, float32(radius*t), float32(max(1.5, t*0.04)), c, true)
}

// placementRing is the ring a tool's ghost shows (the beacon's radius).
func placementRing(kind string) (float64, bool) {
	if kind == tradeBeaconKind {
		return beaconRadiusTiles, true
	}
	return 0, false
}

// turretLine is a turret's status line in the pane: No power, Target (it
// shot recently, so it holds a target) or Idle.
func turretLine(b game.BuildingView) string {
	switch {
	case b.Kind != sentryTurretKind:
		return ""
	case !b.Powered:
		return "No power"
	case b.ShotAge >= 0 && b.ShotAge < turretEngagedTicks:
		return "Target locked"
	}
	return "Idle"
}
