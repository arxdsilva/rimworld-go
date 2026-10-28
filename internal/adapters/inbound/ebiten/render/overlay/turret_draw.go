package overlay

import (
	"fmt"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Sentry turret, radio console and trade beacon on the map (PLAY-024,
// ART §38). Presentation only: the gun frame comes from
// BuildingView.AimDir, the recoil and flash from ShotAge, the glow from
// the shared power look (power_draw.go). The beacon's blink is a renderer
// phase on v.Present, never sim state.
const (
	SentryTurretKind = "sentry turret"
	TradeBeaconKind  = "trade beacon"
	RadioConsoleKind = "radio console"

	TurretAimFrames = 16
	turretStepDeg   = 22.5
	// TurretRecoilTicks is how long after a shot the gun sits pushed back,
	// and turretRecoilTiles how far at the first tick.
	TurretRecoilTicks = 6
	turretRecoilTiles = 0.07
	// TurretRangeTiles and BeaconRadiusTiles are the rings shown on
	// selection and on the placement ghost (mirror the tuning table).
	TurretRangeTiles  = 10.0
	BeaconRadiusTiles = 6.0
	// turretEngagedTicks: a shot this recent means the turret holds a target.
	turretEngagedTicks = 90
	BeaconBlinkFrames  = 90
	beaconOnFrames     = 50
)

// TurretBaseKey is the turret base sprite.
const TurretBaseKey = "defense/sentry_turret_base"

// TurretGunKey is gun frame i (0..15) of the turret.
func TurretGunKey(i int) string { return fmt.Sprintf("defense/sentry_turret_gun_%02d", i) }

// aimFrame clamps a view's aim to 0..15.
func aimFrame(d int8) int { return ((int(d) % TurretAimFrames) + TurretAimFrames) % TurretAimFrames }

// TurretAimVec is the unit screen-space direction of aim frame i: frame 0
// points south, counting clockwise on screen.
func TurretAimVec(i int) motion.Vec {
	th := float64(i) * turretStepDeg * math.Pi / 180
	return motion.Vec{X: -math.Sin(th), Y: math.Cos(th)}
}

// turretGunAt is where turret b's gun frame is drawn (its origin, in
// world tiles): the frame's mount attachment on the base's mount.
func turretGunAt(c *art.Catalog, b *game.BuildingView) (key string, wx, wy float64, ok bool) {
	mx, my, ok := attachWorld(c, TurretBaseKey, "mount", float64(b.X), float64(b.Y))
	if !ok {
		return "", 0, 0, false
	}
	key = TurretGunKey(aimFrame(b.AimDir))
	g := c.Sprites[key]
	if g == nil {
		return "", 0, 0, false
	}
	gm, ok := g.Meta.Attach["mount"]
	if !ok {
		return "", 0, 0, false
	}
	return key, mx - (gm[0]-g.Meta.Origin[0])/art.TilePx, my - (gm[1]-g.Meta.Origin[1])/art.TilePx, true
}

// TurretMuzzle is the muzzle of turret b's gun in world tiles (ok false
// without the art).
func TurretMuzzle(c *art.Catalog, b *game.BuildingView) (motion.Vec, bool) {
	key, wx, wy, ok := turretGunAt(c, b)
	if !ok {
		return motion.Vec{}, false
	}
	x, y, ok := attachWorld(c, key, "muzzle", wx, wy)
	return motion.Vec{X: x, Y: y}, ok
}

// RecoilPush is how far back (tiles) the gun sits shotAge ticks after a
// shot: 0 before the first shot and once it has settled.
func RecoilPush(shotAge int) float64 {
	if shotAge < 0 || shotAge >= TurretRecoilTicks {
		return 0
	}
	return turretRecoilTiles * (1 - float64(shotAge)/TurretRecoilTicks)
}

// BuildingByID finds a drawn building by id (traces name a turret by it).
func (o *Set) BuildingByID(v *render.View, id entity.EntityID) (*game.BuildingView, bool) {
	for i := range v.W.Buildings {
		if v.W.Buildings[i].ID == id {
			return &v.W.Buildings[i], true
		}
	}
	return nil, false
}

// BeaconLit is the beacon light's blink phase on the present clock.
func BeaconLit(present uint64, id entity.EntityID, reduced bool) bool {
	if reduced {
		return true
	}
	return (present+uint64(id)*17)%BeaconBlinkFrames < beaconOnFrames
}

// DrawDefenseParts draws what the turret and the beacon add over their
// base sprite (drawn just before): the turret's gun on its aim frame,
// pushed back by the recoil, and the beacon's blinking light. A turret's
// and a console's status glow come from DrawPowerParts.
func (o *Wiring) DrawDefenseParts(v *render.View, ps sprite.Pass, ch *Chunks, dst *ebiten.Image, b *game.BuildingView, key string, tile float64, sw, sh int) {
	if ch.Baking || v.Art == nil {
		return
	}
	switch b.Kind {
	case SentryTurretKind:
		gk, wx, wy, ok := turretGunAt(v.Art, b)
		if !ok {
			return
		}
		if !v.ReducedMotion {
			if p := RecoilPush(b.ShotAge); p > 0 {
				d := TurretAimVec(aimFrame(b.AimDir))
				wx, wy = wx-d.X*p, wy-d.Y*p
			}
		}
		o.PowerTrace("gun", gk)
		o.P.DrawCS(v, ps, dst, gk, color.NRGBA{}, wx, wy, tile, sw, sh, sprite.NoCS)
	case TradeBeaconKind:
		g := o.Parts.glowOf(key)
		if v.Art.Has(g) && BeaconLit(v.Present, b.ID, v.ReducedMotion) &&
			o.drawUnlitSprite(v, dst, g, float64(b.X), float64(b.Y), tile, sw, sh, sprite.NoCS) {
			o.PowerTrace("glow", g)
		}
	}
}

// DrawRangeRing strokes a circle of radius tiles around the centre of
// cell (x, y): the turret's reach when selected, the beacon's pickup
// radius on its placement ghost.
func (o *Wiring) DrawRangeRing(v *render.View, dst *ebiten.Image, x, y int, radius float64, c color.NRGBA) {
	cx, cy := v.WorldToScreenF(float64(x)+0.5, float64(y)+0.5)
	t := v.Cam.Tile()
	o.PowerTrace("ring", fmt.Sprintf("%d,%d,%g", x, y, radius))
	vector.StrokeCircle(dst, cx, cy, float32(radius*t), float32(max(1.5, t*0.04)), c, true)
}

// PlacementRing is the ring a tool's ghost shows (the beacon's radius).
func PlacementRing(kind string) (float64, bool) {
	if kind == TradeBeaconKind {
		return BeaconRadiusTiles, true
	}
	return 0, false
}

// TurretLine is a turret's status line in the pane: No power, Target (it
// shot recently, so it holds a target) or Idle.
func TurretLine(b game.BuildingView) string {
	switch {
	case b.Kind != SentryTurretKind:
		return ""
	case !b.Powered:
		return "No power"
	case b.ShotAge >= 0 && b.ShotAge < turretEngagedTicks:
		return "Target locked"
	}
	return "Idle"
}
