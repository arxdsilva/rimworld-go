package ebiten

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Presentation of animals, held weapons and attack traces. Everything
// here reads the WorldView/PawnView; nothing writes back.

// syncAnimals advances per-animal presentation state (lerp, facing).
func (r *Renderer) syncAnimals(w game.WorldView) {
	tick := r.game.Tick()
	for _, an := range w.Animals {
		prev, seen := r.last[an.ID]
		r.last[an.ID] = faceAttack(nextVisualStep(prev, seen, an.X, an.Y, tick, r.present, an.StepTicks), w.Traces, an.X, an.Y)
	}
}

// faceAttack turns an attacker standing at (x, y) towards its latest
// target so weapons and bites point the right way.
func faceAttack(v pawnVisual, traces []game.TraceView, x, y int) pawnVisual {
	for _, t := range traces {
		if t.FromX == x && t.FromY == y && (t.ToX != x || t.ToY != y) {
			v.facing = facingFor(t.ToX-x, t.ToY-y, v.facing)
		}
	}
	return v
}

func (r *Renderer) interpAnimal(an game.AnimalView) (float64, float64) {
	vis, ok := r.last[an.ID]
	if !ok {
		return float64(an.X), float64(an.Y)
	}
	t := moveProgress(int(r.game.Tick()-vis.moveTick), max(an.StepTicks, 1))
	return lerp(vis.fromX, an.X, t), lerp(vis.fromY, an.Y, t)
}

// addAnimalDraws queues animals into the depth-sorted object pass.
func (r *Renderer) addAnimalDraws(w game.WorldView, add func(row, col float64, fn func()), screen *ebiten.Image, tile float64, sw, sh int) {
	for _, an := range w.Animals {
		an := an
		wx, wy := r.interpAnimal(an)
		add(wy+0.01, wx, func() {
			f := r.last[an.ID].facing
			key := "animal/" + an.Species + "_" + f.String()
			if !r.art.has(key) {
				sx, sy := r.worldToScreenF(wx, wy)
				vector.FillCircle(screen, sx+float32(tile)/2, sy+float32(tile)/2, float32(tile)*0.3, color.RGBA{150, 120, 80, 255}, false)
				return
			}
			r.drawSprite(screen, key, color.NRGBA{}, wx, wy, tile, sw, sh)
		})
	}
}

// drawAnimalOverlays draws hunt marks and health bars above animals.
func (r *Renderer) drawAnimalOverlays(screen *ebiten.Image, w game.WorldView, tile float64, sw, sh int) {
	for _, an := range w.Animals {
		wx, wy := r.interpAnimal(an)
		if an.Designated {
			r.drawSpriteScaled(screen, "overlay/designation_hunt", color.NRGBA{}, wx, wy-1.2, tile, sw, sh, noCS, 0.5)
		}
		if an.Health < 100 {
			sx, sy := r.worldToScreenF(wx+0.15, wy-0.32)
			bw := float32(tile) * 0.7
			vector.FillRect(screen, sx, sy, bw, 4, color.RGBA{30, 24, 20, 220}, false)
			vector.FillRect(screen, sx, sy, bw*float32(an.Health)/100, 4, healthColor(an.Health), false)
		}
	}
}

// Trace styling.
var (
	tracerClr = color.RGBA{255, 226, 140, 255}
	flashClr  = color.RGBA{255, 240, 190, 255}
	bloodClr  = color.RGBA{190, 44, 36, 255}
	swingClr  = color.RGBA{245, 240, 230, 255}
)

func fade(c color.RGBA, k float64) color.RGBA {
	if k <= 0 {
		return color.RGBA{}
	}
	if k > 1 {
		k = 1
	}
	return color.RGBA{uint8(float64(c.R) * k), uint8(float64(c.G) * k), uint8(float64(c.B) * k), uint8(float64(c.A) * k)}
}

// drawTraces renders recent attacks: rifle tracers with a muzzle flash
// and a hit puff, or a short swing arc for melee.
func (r *Renderer) drawTraces(screen *ebiten.Image, w game.WorldView, tile float64) {
	t := float32(tile)
	for _, tr := range w.Traces {
		fx, fy := r.worldToScreenF(float64(tr.FromX)+0.5, float64(tr.FromY)+0.3)
		tx, ty := r.worldToScreenF(float64(tr.ToX)+0.5, float64(tr.ToY)+0.4)
		if tr.Melee {
			if tr.Age < 10 {
				k := 1 - float64(tr.Age)/10
				a0 := math.Atan2(float64(ty-fy), float64(tx-fx)) - 0.9
				for i := 0; i < 3; i++ {
					a1, a2 := a0+float64(i)*0.6, a0+float64(i+1)*0.6
					rad := float64(t) * 0.35
					vector.StrokeLine(screen,
						tx+float32(math.Cos(a1)*rad), ty+float32(math.Sin(a1)*rad),
						tx+float32(math.Cos(a2)*rad), ty+float32(math.Sin(a2)*rad),
						3, fade(swingClr, k), true)
				}
			}
			if tr.Hit && tr.Age < 12 {
				vector.FillCircle(screen, tx, ty, t*0.08*float32(1+tr.Age/4), fade(bloodClr, 1-float64(tr.Age)/12), true)
			}
			continue
		}
		dx, dy := tx-fx, ty-fy
		n := float32(math.Hypot(float64(dx), float64(dy)))
		if n < 1 {
			n = 1
		}
		ux, uy := dx/n, dy/n
		ex, ey := tx, ty
		if !tr.Hit { // a miss flies past, off to one side
			side := float32(1)
			if (tr.FromX+tr.ToY)%2 == 0 {
				side = -1
			}
			ex, ey = tx+ux*t*0.8-uy*t*0.35*side, ty+uy*t*0.8+ux*t*0.35*side
		}
		mx, my := fx+ux*t*0.5, fy+uy*t*0.5 // muzzle
		if tr.Age < 6 {
			k := 1 - float64(tr.Age)/6
			// a short bright streak travelling along the line
			p := float32(tr.Age+1) / 6
			sx, sy := mx+(ex-mx)*(p-0.25), my+(ey-my)*(p-0.25)
			if p < 0.25 {
				sx, sy = mx, my
			}
			vector.StrokeLine(screen, mx, my, ex, ey, 1, fade(tracerClr, k*0.35), true)
			vector.StrokeLine(screen, sx, sy, mx+(ex-mx)*p, my+(ey-my)*p, 3, fade(tracerClr, k), true)
		}
		if tr.Age < 3 {
			vector.FillCircle(screen, mx, my, t*0.1, fade(flashClr, 1-float64(tr.Age)/3), true)
		}
		if tr.Hit && tr.Age >= 3 && tr.Age < 14 {
			k := 1 - float64(tr.Age-3)/11
			vector.FillCircle(screen, tx, ty, t*(0.05+0.012*float32(tr.Age)), fade(bloodClr, k), true)
		}
	}
}

// heldWeaponKey maps a wielded weapon to a sprite and how it is held.
func heldWeaponKey(weapon string) (key string, scale float64, ok bool) {
	switch weapon {
	case "rifle":
		return "weapon/rifle_basic", 0.62, true
	case "club":
		return "item/club_wood", 0.5, true
	}
	return "", 0, false
}

// drawHeldWeapon draws the wielded weapon with its grip at the body's
// hand attachment, pointing along the pawn's facing.
func (r *Renderer) drawHeldWeapon(screen *ebiten.Image, p game.PawnView, f facing, wx, wy, tile float64) {
	key, k, ok := heldWeaponKey(p.Weapon)
	if !ok {
		return
	}
	body := r.art.sprite("pawn/body_" + f.String())
	wsp := r.art.sprite(key)
	img := r.art.image(key, color.NRGBA{})
	if body == nil || wsp == nil || img == nil {
		return
	}
	hand, okH := body.meta.Attach["hand"]
	if !okH {
		return
	}
	sx, sy := r.worldToScreenF(wx, wy)
	bx, by, s := artOrigin(body.meta, float64(sx), float64(sy), tile)
	hx, hy := bx+hand[0]*s, by+hand[1]*s
	grip, okG := wsp.meta.Attach["grip"]
	if !okG {
		grip = [2]float64{float64(wsp.meta.Canvas[0]) / 2, float64(wsp.meta.Canvas[1]) / 2}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-grip[0], -grip[1])
	switch f {
	case faceWest:
		op.GeoM.Scale(-1, 1)
	case faceSouth:
		op.GeoM.Rotate(math.Pi / 2.6)
	case faceNorth:
		op.GeoM.Rotate(-math.Pi / 2.6)
	}
	op.GeoM.Scale(s*k, s*k)
	op.GeoM.Translate(hx, hy)
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(img, op)
}

// nextVisualStep is nextVisual for movers with their own pace.
func nextVisualStep(prev pawnVisual, seen bool, x, y int, tick clock.Tick, present uint64, step int) pawnVisual {
	v := nextVisual(prev, seen, x, y, tick, present)
	if seen && prev.x == x && prev.y == y && tick-prev.moveTick < clock.Tick(max(step, 1)) {
		v.fromX, v.fromY = prev.fromX, prev.fromY
	}
	return v
}

// settleVisuals finishes every in-flight step lerp. Captures freeze the
// simulation, so without this a mover is drawn where its last step began.
func (r *Renderer) settleVisuals() {
	for id, v := range r.last {
		v.fromX, v.fromY = v.x, v.y
		r.last[id] = v
	}
}
